package main

import (
	"context"
	"fmt"
	"log"
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

var startTime time.Time

func init() {
	startTime = time.Now()
	logging.I("metarr started at: %v", startTime.Format("2006-01-02 15:04:05.00 MST"))

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

	// Handle cleanup on interrupt or termination signals
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Set(keys.Context, ctx)

	// Program control
	cleanupChan := make(chan os.Signal, 1)
	signal.Notify(cleanupChan, syscall.SIGINT, syscall.SIGTERM)
	var wg sync.WaitGroup

	core := &models.Core{
		Cleanup: cleanupChan,
		Cancel:  cancel,
		Ctx:     ctx,
		Wg:      &wg,
	}

	if err := fsRead.InitFetchFilesVars(); err != nil {
		logging.E(0, "Failed to initialize variables to fetch files. Exiting...")
		cancel() // Do not remove call before exit
		os.Exit(1)
	}

	prompt.InitUserInputReader()

	if cfg.IsSet(keys.BatchPairs) {
		processing.StartBatchLoop(core)
	} else {
		logging.I("No files or directories to process. Exiting.")
	}

	endTime := time.Now()
	logging.I("metarr finished at: %v", endTime.Format("2006-01-02 15:04:05.00 MST"))
	logging.I("Time elapsed: %.2f seconds", endTime.Sub(startTime).Seconds())
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
		log.Fatal("could not create CPU profile: ", err)
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

	log.Fatal(exitMsg)
}
