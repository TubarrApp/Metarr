package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"metarr/internal/cfg"
	"metarr/internal/models"
	"metarr/internal/processing"
	"metarr/internal/utils/benchmark"
	"metarr/internal/utils/fs/fsread"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"

	"github.com/spf13/viper"
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
)

func init() {
	startTime = time.Now()
	logging.I(startLogFormat, startTime.Format(timeFormat))
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			logging.E("Panic recovered: %v", r)
			benchmark.CloseBenchmarking()
			panic(r) // Re-panic after cleanup
		}
	}()

	metaOps, err := cfg.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}

	if !viper.GetBool("execute") {
		fmt.Println()
		logging.I("(Separate fields supporting multiple entries by commas with no spaces e.g. \"title:example,date:20240101\")\n")
		return // Exit early if not meant to execute
	}

	// Initialize meta ops (outside of Execute command to return MetaOps)

	if err := initializeApplication(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}
	defer benchmark.CloseBenchmarking()

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
		cancel()
		os.Exit(1)
	}

	prompt.InitUserInputReader()

	batches, err := initializeBatchConfigs(metaOps)
	if err != nil {
		logging.E("Failed to initialize batch configs. Exiting...")
		cancel()
		os.Exit(1)
	}

	if len(batches) > 0 {
		if err := processing.StartBatchLoop(core, batches); err != nil {
			logging.E("error during batch loop: %v", err)
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
