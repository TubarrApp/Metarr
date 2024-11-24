package main

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/processing"
	fsRead "metarr/internal/utils/fs/read"
	logging "metarr/internal/utils/logging"
	prompt "metarr/internal/utils/prompt"
	"os"
	"os/signal"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"syscall"
	"time"
)

// String constants
const (
	timeFormat     = "2006-01-02 15:04:05.00 MST"
	startLogFormat = "metarr started at: %s"
	endLogFormat   = "metarr finished at: %s"
	elapsedFormat  = "Time elapsed: %.2f seconds"
)

// Sigs here prevents heap escape
var (
	startTime time.Time
	sigInt    = syscall.SIGINT
	sigTerm   = syscall.SIGTERM
)

func init() {
	startTime = time.Now()
	logging.I(startLogFormat, startTime.Format(timeFormat))

	// Benchmarking
	if cfg.GetBool(keys.Benchmarking) {
		setupBenchmarking()
	}
}

func main() {
	if err := cfg.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}

	if !cfg.GetBool("execute") {
		fmt.Println()
		logging.I(`(Separate fields supporting multiple entries by commas with no spaces e.g. "title:example,date:20240101")`)
		fmt.Println()
		return // Exit early if not meant to execute
	}

	// Program elements
	ctx, cancel := context.WithCancel(context.Background())
	cleanupChan := make(chan os.Signal, 1)
	signal.Notify(cleanupChan, sigInt, sigTerm)
	wg := new(sync.WaitGroup)

	core := &models.Core{
		Cleanup: cleanupChan,
		Cancel:  cancel,
		Ctx:     ctx,
		Wg:      wg,
	}

	if err := fsRead.InitFetchFilesVars(); err != nil {
		logging.E(0, "Failed to initialize variables to fetch files. Exiting...")
		cancel() // Do not remove call before exit
		os.Exit(1)
	}

	prompt.InitUserInputReader()

	if cfg.IsSet(keys.BatchPairs) {
		if err := processing.StartBatchLoop(core); err != nil {
			logging.E(0, "error during batch loop: %v", err)
			cancel()
			os.Exit(1)
		}
	} else {
		logging.I("No files or directories to process. Exiting.")
	}

	endTime := time.Now()
	logging.I(endLogFormat, endTime.Format(timeFormat))
	logging.I(elapsedFormat, endTime.Sub(startTime).Seconds())
	fmt.Println()
}

// Benchmarking ////////////////////////////////////////////////////////////////////////////////////////////

type benchFiles struct {
	cpuFile   *os.File
	memFile   *os.File
	traceFile *os.File
}

func setupBenchmarking() {
	var (
		b   benchFiles
		err error
	)

	// CPU profile
	b.cpuFile, err = os.Create("cpu.prof")
	if err != nil {
		closeBenchFiles(&b, fmt.Sprintf("could not create CPU profile: %v", err))
	}

	if err := pprof.StartCPUProfile(b.cpuFile); err != nil {
		closeBenchFiles(&b, fmt.Sprintf("could not start CPU profile: %v", err))
	}

	defer pprof.StopCPUProfile()

	// Memory profile
	b.memFile, err = os.Create("mem.prof")
	if err != nil {
		closeBenchFiles(&b, fmt.Sprintf("could not create memory profile: %v", err))
	}
	defer func() {
		if cfg.GetBool(keys.Benchmarking) {
			if err := pprof.WriteHeapProfile(b.memFile); err != nil {
				closeBenchFiles(&b, fmt.Sprintf("could not write memory profile: %v", err))
			}
		}
	}()

	// Trace
	b.traceFile, err = os.Create("trace.out")
	if err != nil {
		closeBenchFiles(&b, fmt.Sprintf("could not create trace file: %v", err))
	}
	if err := trace.Start(b.traceFile); err != nil {
		closeBenchFiles(&b, fmt.Sprintf("could not start trace: %v", err))
	}
}

// closeBenchFiles closes bench files on program termination
func closeBenchFiles(b *benchFiles, exitMsg string) {

	if b.cpuFile != nil {
		b.cpuFile.Close()
	}

	if b.memFile != nil {
		b.memFile.Close()
	}

	if b.traceFile != nil {
		b.traceFile.Close()
	}

	logging.E(0, exitMsg)
	os.Exit(1)
}
