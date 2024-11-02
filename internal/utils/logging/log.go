package utils

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var ErrorArray []error
var Loggable bool = false
var Logger *log.Logger

// Regular expression to match ANSI escape codes
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// SetupLogging creates and/or opens the log file
func SetupLogging(targetDir string) error {

	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(targetDir, "/metarr.log"), // Log file path
		MaxSize:    1,                                       // Max size in MB before rotation
		MaxBackups: 3,                                       // Number of backups to retain
		Compress:   true,                                    // Gzip compression
	}

	// Assign lumberjack logger to standard log output
	Logger = log.New(logFile, "", log.LstdFlags)
	Loggable = true

	Logger.Printf(":\n=========== %v ===========\n\n", time.Now().Format(time.RFC1123Z))
	return nil
}

// Write writes error information to the log file
func Write(msg string, level int) {
	// Do not add mutex, only called by callers which themselves use mutex
	if Loggable && level < 2 {
		if !strings.HasPrefix(msg, "\n") {
			msg += "\n"
		}
		Logger.Print(ansiEscape.ReplaceAllString(msg, ""))
	}
}

// WriteArray writes an array of error information to the log file
func WriteArray(msgs []string, args ...interface{}) {
	if Loggable {
		if len(msgs) != 0 {
			var msg string
			for i, entry := range msgs {
				switch i {
				case len(msgs) - 1:
					msg += fmt.Sprintf(entry, args...)
				default:
					msg += fmt.Sprintf(entry+", ", args...)
				}
				Logger.Print(ansiEscape.ReplaceAllString(msg, ""))
			}
		}
	}
}
