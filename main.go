package main

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/processing"
	"metarr/internal/utils/benchmark"
	"metarr/internal/utils/fs/fsread"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
	"os"
	"os/signal"
	"runtime"
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
	startTime         time.Time
	sigInt            = syscall.SIGINT
	sigTerm           = syscall.SIGTERM
	benchFiles        *benchmark.BenchFiles
	err, benchErrExit error
)

func init() {
	startTime = time.Now()
	logging.I(startLogFormat, startTime.Format(timeFormat))

	// Benchmarking
	if cfg.GetBool(keys.Benchmarking) {
		// Get directory of main.go (helpful for benchmarking file save locations)
		_, mainGoPath, _, ok := runtime.Caller(0)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error getting current working directory. Got: %v\n", mainGoPath)
			os.Exit(1)
		}
		benchFiles, err = benchmark.SetupBenchmarking(mainGoPath)
	}

}

func main() {
	defer func() {
		if benchErrExit != nil {
			benchmark.CloseBenchFiles(benchFiles, "", err)
		} else {
			benchmark.CloseBenchFiles(benchFiles, fmt.Sprintf("Benchmark ending at: %s", time.Now().Format(timeFormat)), nil)
		}
	}()

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

	if err := fsread.InitFetchFilesVars(); err != nil {
		logging.E(0, "Failed to initialize variables to fetch files. Exiting...")
		benchErrExit = err
		cancel() // Do not remove call before exit
		os.Exit(1)
	}

	prompt.InitUserInputReader()

	if cfg.IsSet(keys.BatchPairs) {
		if err := processing.StartBatchLoop(core); err != nil {
			logging.E(0, "error during batch loop: %v", err)
			benchErrExit = err
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
