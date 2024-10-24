package main

import (
	"Metarr/internal/cmd"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/naming"
	"Metarr/internal/processing"
	"Metarr/internal/shared"
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

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}

	if !cmd.GetBool("execute") {
		fmt.Println()
		logging.PrintI(`(Separate fields supporting multiple entries by commas with no spaces e.g. "title:example,date:20240101")`)
		fmt.Println()
		return // Exit early if not meant to execute
	}

	// Handle cleanup on interrupt or termination signals
	ctx, cancel := context.WithCancel(context.Background())
	cmd.Set(keys.Context, ctx)
	defer cancel()

	// Open input video directory
	inputVideoDir := cmd.GetString(keys.VideoDir)
	openVideoDir, err := os.Open(inputVideoDir)
	if err != nil {
		logging.PrintE(0, "Error: %v", err)
		os.Exit(1)
	}
	defer openVideoDir.Close()
	cmd.Set(keys.OpenVideoDir, openVideoDir)

	directory = inputVideoDir

	// Open input metadata directory
	inputMetaDir := cmd.GetString(keys.JsonDir)
	openJsonDir, err := os.Open(inputMetaDir)
	if err != nil {
		logging.PrintE(0, "Error: %v", err)
		os.Exit(1)
	}
	defer openJsonDir.Close()
	cmd.Set(keys.OpenJsonDir, openJsonDir)

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
	cmd.Set(keys.WaitGroup, &wg)

	cleanupChan := make(chan os.Signal, 1)
	signal.Notify(cleanupChan, syscall.SIGINT, syscall.SIGTERM)

	naming.FieldOverwrite = cmd.GetBool(keys.MOverwrite)
	naming.FieldPreserve = cmd.GetBool(keys.MPreserve)

	if naming.FieldOverwrite && naming.FieldPreserve {
		fmt.Println()
		logging.PrintE(0, "Cannot enter both meta preserve AND meta overwrite, exiting...")
		fmt.Println()
		os.Exit(1)
	}

	shared.InitUserInputReader()

	// Proceed to process files (videos, metadata files, etc...)
	processing.ProcessFiles(ctx, cancel, &wg, cleanupChan, openVideoDir, openJsonDir)
}
