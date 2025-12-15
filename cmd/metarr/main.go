// Package main is the main entrypoint of the program.
package main

import (
	"bytes"
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/cfg"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/paths"
	"metarr/internal/domain/vars"
	"metarr/internal/file"
	"metarr/internal/models"
	"metarr/internal/processing"
	"metarr/internal/transformations"
	"metarr/internal/utils/prompt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/TubarrApp/gocommon/benchmark"
	"github.com/TubarrApp/gocommon/logging"
)

// Main program string constants.
const (
	timeFormat     = "2006-01-02 15:04:05.00 MST"
	startLogFormat = "Metarr started at: %s"
	endLogFormat   = "Metarr finished at: %s"
	elapsedFormat  = "Time elapsed: %.2f seconds\n"
)

// Log vars.
var (
	tubarrLogServer = "http://127.0.0.1:8827/metarr-logs"
	logMutex        sync.Mutex
	lastSentPos     int
	lastSentWrapped bool
)

// init before program run.
func init() {
	if err := paths.InitProgFilesDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Metarr exiting with error: %v\n", err)
		os.Exit(1)
	}
}

// main is the program entrypoint.
func main() {
	startTime := time.Now()
	// Setup logging.
	logConfig := logging.LoggingConfig{
		LogFilePath: paths.MetarrLogFilePath,
		MaxSizeMB:   1,
		MaxBackups:  3,
		Console:     os.Stderr,
		Program:     "Metarr",
	}

	pl, err := logging.SetupLogging(logConfig)
	if err != nil {
		fmt.Printf("Tubarr exiting with error: %v\n", err)
		return
	}
	logger.Pl = pl

	// Log start time.
	logger.Pl.I(startLogFormat, startTime.Format(timeFormat))

	// Panic recovery with proper cleanup.
	defer func() {
		if r := recover(); r != nil {
			logger.Pl.E("Panic recovered: %v", r)
			logger.Pl.E("Stack trace:\n\n%s", debug.Stack())
			os.Exit(1)
		}
	}()

	// Ensure benchmarking is closed on all exit paths.
	defer benchmark.CloseBenchFiles(logger.Pl, vars.BenchmarkFiles, "", nil)

	// Parse configuration.
	if err := cfg.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintf(os.Stderr, "\n")
		return
	}

	// Early exit if not executing.
	if !abstractions.GetBool("execute") {
		fmt.Fprintf(os.Stderr, "\n")
		return
	}

	// Setup context for cancellation.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer cancel()

	// Log POST goroutine.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendLogs()
			}
		}
	}()

	// Ensure log POST on main() exit.
	defer sendLogs()

	// Initialize cached variables.
	if err := file.InitFetchFilesVars(); err != nil {
		logger.Pl.E("Failed to initialize variables to fetch files. Exiting...")
		cancel()
		return
	}

	// Initialize user input reader (used for prompting the user during program run).
	prompt.InitUserInputReader()

	// Process batches.
	wg := new(sync.WaitGroup)
	core := &models.Core{
		Ctx: ctx,
		Wg:  wg,
	}

	fdArray := []*models.FileData{}
	fdArrayResult, err := processing.ProcessBatches(core)
	if err != nil {
		logger.Pl.E("error during batch loop: %v", err)
		cancel()
		wg.Wait()
		return
	}
	fdArray = append(fdArray, fdArrayResult...)

	// Wait for all goroutines to finish.
	wg.Wait()

	// Process renames.
	if len(fdArray) > 0 {
		logger.Pl.I("Processing file renames for %d file(s)...", len(fdArray))

		if err := transformations.RenameFiles(ctx, fdArray); err != nil {
			logger.Pl.E("Error during file renaming: %v", err)
		}
		logger.Pl.S("File renaming complete!")
	}

	// Check if shutdown was triggered by signal.
	select {
	case <-ctx.Done():
		logger.Pl.I("Shutdown was triggered by signal")
	default:
	}

	// End program run.
	endTime := time.Now()
	fmt.Fprintf(os.Stderr, "\n")
	logger.Pl.I(endLogFormat, endTime.Format(timeFormat))
	logger.Pl.I(elapsedFormat, endTime.Sub(startTime).Seconds())
}

// sendLogs POSTs logs to Tubarr.
func sendLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	pl, ok := logging.GetProgramLogger("Metarr")
	if !ok {
		return
	}

	// Get new logs since last successful send.
	logs := pl.GetLogsSincePosition(lastSentPos, lastSentWrapped)

	if len(logs) > 0 {
		// POST logs to Tubarr.
		body := bytes.Join(logs, []byte{})
		resp, err := http.Post(tubarrLogServer, "text/plain", bytes.NewReader(body))
		if err != nil {
			logger.Pl.E("Could not send logs to Tubarr: %v", err)
			return
		}

		// Update tracking if POST was successful.
		if resp.StatusCode == http.StatusOK {
			lastSentPos = pl.GetBufferPosition()
			lastSentWrapped = pl.IsBufferFull()
		}

		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Pl.E("Could not close response body: %v", closeErr)
		}
	}
}
