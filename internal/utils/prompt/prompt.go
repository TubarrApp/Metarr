package utils

import (
	"bufio"
	"context"
	"fmt"
	logging "metarr/internal/utils/logging"
	"os"
	"strings"
)

var (
	userInputChan = make(chan string) // Channel for user input
	decisionMade  bool
)

// InitUserInputReader initializes a user input reading function in a goroutine
func InitUserInputReader() {
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, _ := reader.ReadString('\n')
			userInputChan <- strings.TrimSpace(input)
		}
	}()
}

// PromptMetaReplace displays a prompt message and waits for valid user input.
// The option can be used to tell the program to overwrite all in the queue,
// preserve all in the queue, or move through value by value
func PromptMetaReplace(promptMsg string, ow, ps bool) (string, error) {

	logging.D(3, "Entering PromptUser dialogue...")
	ctx := context.Background()

	if decisionMade {
		// If overwriteAll, return "Y" without waiting
		if ow {

			logging.D(3, "Overwrite all is set...")
			return "Y", nil
		} else if ps {

			logging.D(3, "Preserve all is set...")
			return "N", nil
		}
	}

	fmt.Println()
	logging.I(promptMsg)

	// Wait for user input
	select {
	case response := <-userInputChan:
		if response == "Y" {
			ow = true
		}
		decisionMade = true
		return response, nil

	case <-ctx.Done():
		logging.I("Operation canceled during input.")
		return "", fmt.Errorf("operation canceled")
	}
}
