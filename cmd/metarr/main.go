package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"metarr/internal/cfg"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/processing"
	"metarr/internal/utils/benchmark"
	"metarr/internal/utils/fs/fsread"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
)

// String constants
const (
	timeFormat     = "2006-01-02 15:04:05.00 MST"
	startLogFormat = "Metarr started at: %s"
	endLogFormat   = "Metarr finished at: %s"
	elapsedFormat  = "Time elapsed: %.2f seconds\n"
)

// Sigs here prevents heap escape
var (
	startTime time.Time
	sigInt    = syscall.SIGINT
	sigTerm   = syscall.SIGTERM
	benchErr  error
)

func init() {
	startTime = time.Now()
	logging.I(startLogFormat, startTime.Format(timeFormat))

	_, mainGoPath, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error getting current working directory. Got: %v\n", mainGoPath)
		os.Exit(1)
	}
	benchmark.InjectMainWorkDir(mainGoPath)
}

func main() {
	if err := cfg.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}

	if !cfg.GetBool("execute") {
		logging.I("\n(Separate fields supporting multiple entries by commas with no spaces e.g. \"title:example,date:20240101\")\n")
		return // Exit early if not meant to execute
	}

	defer func() {
		if cfg.IsSet(keys.BenchFiles) {
			benchFiles, ok := cfg.Get(keys.BenchFiles).(*benchmark.BenchFiles)
			if !ok || benchFiles == nil {
				logging.E("Null benchFiles or wrong type. Got type: %T", benchFiles)
				return
			}
			if benchErr != nil {
				benchmark.CloseBenchFiles(benchFiles, "", benchErr)
			} else {
				benchmark.CloseBenchFiles(benchFiles, fmt.Sprintf("Benchmark ended at %v", time.Now().Format(time.RFC1123Z)), nil)
			}
		}
	}()

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
		logging.E("Failed to initialize variables to fetch files. Exiting...")
		benchErr = err
		cancel() // Do not remove call before exit
		os.Exit(1)
	}

	prompt.InitUserInputReader()

	if cfg.IsSet(keys.BatchPairs) {
		if err := processing.StartBatchLoop(core); err != nil {
			logging.E("error during batch loop: %v", err)
			benchErr = err
			cancel()
			os.Exit(1)
		}
	} else {
		logging.I("No files or directories to process. Exiting.")
	}

	endTime := time.Now()
	logging.I(endLogFormat, endTime.Format(timeFormat))
	logging.I(elapsedFormat, endTime.Sub(startTime).Seconds())
}
