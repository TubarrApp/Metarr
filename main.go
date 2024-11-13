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
	defer cancel()

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
		os.Exit(1)
	}

	prompt.InitUserInputReader()

	if cfg.IsSet(keys.BatchPairs) {

		batch, ok := cfg.Get(keys.BatchPairs).([]*models.Batch)
		if !ok {
			logging.E(0, "Wrong type")
		}

		for _, b := range batch {
			b.Core.Cancel = cancel
			b.Core.Ctx = ctx
			b.Core.Wg = &wg
			b.Core.Cleanup = cleanupChan
		}
		processing.StartBatchLoop(core)

	} else {
		logging.I("No files or directories to process. Exiting.")
	}

	endTime := time.Now()
	logging.I("metarr finished at: %v", endTime.Format("2006-01-02 15:04:05.00 MST"))
	logging.I("Time elapsed: %.2f seconds", endTime.Sub(startTime).Seconds())
	fmt.Println()
}

func setupBenchmarking() {
	// CPU profile
	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	defer cpuFile.Close() // Don't forget to close the file
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	// Memory profile
	memFile, err := os.Create("mem.prof")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer memFile.Close()
	defer func() {
		if cfg.GetBool(keys.Benchmarking) {
			if err := pprof.WriteHeapProfile(memFile); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}
	}()

	// Trace
	traceFile, err := os.Create("trace.out")
	if err != nil {
		log.Fatal("could not create trace file: ", err)
	}
	defer traceFile.Close()
	if err := trace.Start(traceFile); err != nil {
		log.Fatal("could not start trace: ", err)
	}
	defer trace.Stop()
}
