// Package prompt handles user prompting (usually for y/n type directives).
package prompt

import (
	"bufio"
	"context"
	"fmt"
	"metarr/internal/domain/logger"
	"os"
	"strings"
)

var (
	userInputChan = make(chan string)
	decisionMade  bool
)

// InitUserInputReader initializes a user input reading function in a goroutine.
func InitUserInputReader() {
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, _ := reader.ReadString('\n')
			userInputChan <- strings.TrimSpace(input)
		}
	}()
}

// MetaReplace displays a prompt message and waits for valid user input.
//
// The option can be used to tell the program to overwrite all in the queue,
// preserve all in the queue, or move through value by value.
func MetaReplace(ctx context.Context, promptMsg string, ow, ps bool) (string, error) {
	logger.Pl.D(3, "Entering PromptUser dialogue...")

	if decisionMade {
		// If overwriteAll, return "Y" without waiting.
		if ow {
			logger.Pl.D(3, "Overwrite all is set...")
			return "Y", nil
		} else if ps {
			logger.Pl.D(3, "Preserve all is set...")
			return "N", nil
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
	logger.Pl.I("%s", promptMsg)

	// Wait for user input.
	select {
	case response := <-userInputChan:
		decisionMade = true
		return response, nil

	case <-ctx.Done():
		return "", fmt.Errorf("operation canceled during prompt %q", promptMsg)
	}
}
