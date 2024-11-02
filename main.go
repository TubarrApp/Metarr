package main

import (
	"Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/processing"
	fsRead "Metarr/internal/utils/fs/read"
	logging "Metarr/internal/utils/logging"
	prompt "Metarr/internal/utils/prompt"
	"context"
	"fmt"
	"log"
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
}

func main() {

	var err error
	var directory string

	// TESTING FUNCTIONS
	if config.GetBool(keys.Benchmarking) {
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
			if config.GetBool(keys.Benchmarking) {
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
	// END OF TESTING FUNCTIONS: MEM TEST WRITE AT BOTTOM

	if err := config.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}

	if !config.GetBool("execute") {
		fmt.Println()
		logging.PrintI(`(Separate fields supporting multiple entries by commas with no spaces e.g. "title:example,date:20240101")`)
		fmt.Println()
		return // Exit early if not meant to execute
	}

	// Handle cleanup on interrupt or termination signals
	ctx, cancel := context.WithCancel(context.Background())
	config.Set(keys.Context, ctx)
	defer cancel()

	var openVideo *os.File
	var inputVideoDir string
	var inputVideo string

	if config.IsSet(keys.VideoDir) {

		inputVideoDir = config.GetString(keys.VideoDir)
		openVideo, err = os.Open(inputVideoDir)
		if err != nil {
			logging.PrintE(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openVideo.Close()
		directory = inputVideoDir

	} else if config.IsSet(keys.VideoFile) {

		inputVideo = config.GetString(keys.VideoFile)
		openVideo, err = os.Open(inputVideo)
		if err != nil {
			logging.PrintE(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openVideo.Close()
		directory = filepath.Dir(inputVideo)
	}
	config.Set(keys.OpenVideo, openVideo)

	var openJson *os.File
	var inputMetaDir string
	var inputMeta string

	if config.IsSet(keys.JsonDir) {

		inputMetaDir = config.GetString(keys.JsonDir)
		openJson, err = os.Open(inputMetaDir)
		if err != nil {
			logging.PrintE(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openJson.Close()
		if directory == "" {
			directory = inputMetaDir
		}

	} else if config.IsSet(keys.JsonFile) {

		inputMeta = config.GetString(keys.JsonFile)
		openJson, err = os.Open(inputMeta)
		if err != nil {
			logging.PrintE(0, "Error: %v", err)
			os.Exit(1)
		}
		defer openJson.Close()
		if directory == "" {
			directory = filepath.Dir(inputMeta)
		}
	}
	config.Set(keys.OpenJson, openJson)

	// Setup logging
	if directory != "" {
		err = logging.SetupLogging(directory)
		if err != nil {
			fmt.Printf("\n\nNotice: Log file was not created\nReason: %s\n\n", err)
		}
	} else {
		logging.PrintI("Directory and file strings were entered empty. Exiting...")
		os.Exit(1)
	}

	if err := fsRead.InitFetchFilesVars(); err != nil {
		logging.PrintE(0, "Failed to initialize variables to fetch files. Exiting...")
		os.Exit(1)
	}

	// Program control
	var wg sync.WaitGroup
	config.Set(keys.WaitGroup, &wg)

	cleanupChan := make(chan os.Signal, 1)
	signal.Notify(cleanupChan, syscall.SIGINT, syscall.SIGTERM)

	fieldOverwrite := config.GetBool(keys.MOverwrite)
	fieldPreserve := config.GetBool(keys.MPreserve)

	if fieldOverwrite && fieldPreserve {
		fmt.Println()
		logging.PrintE(0, "Cannot enter both meta preserve AND meta overwrite, exiting...")
		fmt.Println()
		os.Exit(1)
	}
	prompt.InitUserInputReader()

	// Proceed to process files (videos, metadata files, etc...)
	processing.ProcessFiles(ctx, cancel, &wg, cleanupChan, openVideo, openJson)

	logging.PrintI("Time elapsed: %v", time.Since(startTime).Seconds())
}
