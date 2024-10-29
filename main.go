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

	// Open input video directory
	inputVideoDir := config.GetString(keys.VideoDir)
	openVideoDir, err := os.Open(inputVideoDir)
	if err != nil {
		logging.PrintE(0, "Error: %v", err)
		os.Exit(1)
	}
	defer openVideoDir.Close()
	config.Set(keys.OpenVideoDir, openVideoDir)

	directory = inputVideoDir

	// Open input metadata directory
	inputMetaDir := config.GetString(keys.JsonDir)
	openJsonDir, err := os.Open(inputMetaDir)
	if err != nil {
		logging.PrintE(0, "Error: %v", err)
		os.Exit(1)
	}
	defer openJsonDir.Close()
	config.Set(keys.OpenJsonDir, openJsonDir)

	if directory == "" {
		directory = inputMetaDir
	}

	// Setup logging
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
	processing.ProcessFiles(ctx, cancel, &wg, cleanupChan, openVideoDir, openJsonDir)
}
