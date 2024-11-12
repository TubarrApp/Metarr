package main

import (
	"context"
	"fmt"
	"log"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/processing"
	fsRead "metarr/internal/utils/fs/read"
	logging "metarr/internal/utils/logging"
	prompt "metarr/internal/utils/prompt"
	"os"
	"os/signal"
	"path/filepath"
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
	var (
		err       error
		directory string
	)

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

	var (
		inputVideoDir,
		inputVideo string

		openVideo *os.File
	)
	if cfg.IsSet(keys.VideoDir) {

		inputVideoDir = cfg.GetString(keys.VideoDir)
		openVideo, err = os.Open(inputVideoDir)
		if err != nil {
			logging.E(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openVideo.Close()
		directory = inputVideoDir

	} else if cfg.IsSet(keys.VideoFile) {

		inputVideo = cfg.GetString(keys.VideoFile)
		openVideo, err = os.Open(inputVideo)
		if err != nil {
			logging.E(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openVideo.Close()
		directory = filepath.Dir(inputVideo)
	}
	cfg.Set(keys.OpenVideo, openVideo)

	var (
		inputMetaDir,
		inputMeta string

		openJson *os.File
	)
	if cfg.IsSet(keys.JsonDir) {

		inputMetaDir = cfg.GetString(keys.JsonDir)
		openJson, err = os.Open(inputMetaDir)
		if err != nil {
			logging.E(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openJson.Close()
		if directory == "" {
			directory = inputMetaDir
		}

	} else if cfg.IsSet(keys.JsonFile) {

		inputMeta = cfg.GetString(keys.JsonFile)
		openJson, err = os.Open(inputMeta)
		if err != nil {
			logging.E(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openJson.Close()
		if directory == "" {
			directory = filepath.Dir(inputMeta)
		}
	}
	cfg.Set(keys.OpenJson, openJson)

	// Setup logging
	if directory != "" {
		err = logging.SetupLogging(directory)
		if err != nil {
			fmt.Printf("\n\nNotice: Log file was not created\nReason: %s\n\n", err)
		}
	} else {
		logging.I("Directory and file strings were entered empty. Exiting...")
		os.Exit(1)
	}

	if err := fsRead.InitFetchFilesVars(); err != nil {
		logging.E(0, "Failed to initialize variables to fetch files. Exiting...")
		os.Exit(1)
	}

	// Program control
	var wg sync.WaitGroup
	cfg.Set(keys.WaitGroup, &wg)

	cleanupChan := make(chan os.Signal, 1)
	signal.Notify(cleanupChan, syscall.SIGINT, syscall.SIGTERM)
	prompt.InitUserInputReader()

	// Proceed to process files (videos, metadata files, etc...)
	processing.ProcessFiles(ctx, cancel, &wg, cleanupChan, openVideo, openJson)

	endTime := time.Now()
	logging.I("metarr finished at: %v", endTime.Format("2006-01-02 15:04:05.00 MST"))
	logging.I("Time elapsed: %.2f seconds", endTime.Sub(startTime).Seconds())
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
