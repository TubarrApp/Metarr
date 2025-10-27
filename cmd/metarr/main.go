// Package main is the main entrypoint of the program.
package main

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/cfg"
	"metarr/internal/models"
	"metarr/internal/processing"
	"metarr/internal/utils/benchmark"
	"metarr/internal/utils/fs/fsread"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
)

// String constants
const (
	timeFormat     = "2006-01-02 15:04:05.00 MST"
	startLogFormat = "Metarr started at: %s"
	endLogFormat   = "Metarr finished at: %s"
	elapsedFormat  = "Time elapsed: %.2f seconds\n"
)

// main is the program entrypoint.
func main() {
	startTime := time.Now()
	logging.I(startLogFormat, startTime.Format(timeFormat))

	// Panic recovery with proper cleanup
	defer func() {
		if r := recover(); r != nil {
			logging.E("Panic recovered: %v", r)
			logging.E("Stack trace:\n\n%s", debug.Stack())
			os.Exit(1)
		}
	}()

	// Ensure benchmarking is closed on all exit paths
	defer benchmark.CloseBenchmarking()

	// Parse configuration
	if err := cfg.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}

	// Early exit if not executing
	if !abstractions.GetBool("execute") {
		fmt.Println()
		return
	}

	// Initialize application
	initializeApplication()

	// Setup context for cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Setup waitgroup for goroutine coordination
	wg := new(sync.WaitGroup)
	core := &models.Core{
		Ctx: ctx,
		Wg:  wg,
	}

	// Initialize cached variables
	if err := fsread.InitFetchFilesVars(); err != nil {
		logging.E("Failed to initialize variables to fetch files. Exiting...")
		cancel()
		os.Exit(1)
	}

	// Initialize user input reader (used for prompting the user during program run)
	prompt.InitUserInputReader()

	// Initialize batch configurations
	batches, err := initializeBatchConfigs()
	if err != nil {
		logging.E("Failed to initialize batch configs. Exiting...")
		cancel()
		os.Exit(1)
	}

	// Process batches
	if len(batches) > 0 {
		if err := processing.StartBatchLoop(core, batches); err != nil {
			logging.E("error during batch loop: %v", err)
			cancel()
			wg.Wait()
			os.Exit(1)
		}
	} else {
		logging.I("No files or directories to process. Exiting.")
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Check if shutdown was triggered by signal
	select {
	case <-ctx.Done():
		logging.I("Shutdown was triggered by signal")
	default:
	}

	// End program run
	endTime := time.Now()
	logging.I(endLogFormat, endTime.Format(timeFormat))
	logging.I(elapsedFormat, endTime.Sub(startTime).Seconds())
}
