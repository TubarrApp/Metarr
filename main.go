package main

import (
	"Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/processing"
	logging "Metarr/internal/utils/logging"
	prompt "Metarr/internal/utils/prompt"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

func main() {

	var err error
	var directory string

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
		logFilePath := filepath.Join(directory, "metarr-log.txt")

		logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logging.PrintE(0, "Error: %v", err)
			os.Exit(1)
		}
		defer logFile.Close()

		err = logging.SetupLogging(directory, logFile)
		if err != nil {
			fmt.Printf(`

Notice: Log file was not created
Reason: %s

`, err)
		}
	} else {
		logging.PrintI("Directory and file strings were entered empty. Exiting...")
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
}
