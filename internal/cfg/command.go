// Package cfg intializes Viper, Cobra, etc.
package cfg

import (
	"fmt"
	"metarr/internal/domain/keys"
	"metarr/internal/utils/benchmark"
	"metarr/internal/utils/logging"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "metarr",
	Short: "Metarr is a video and metatagging tool.",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		// Set logging level
		logging.Level = min(max(viper.GetInt(keys.DebugLevel), 0), 5)

		// Setup benchmarking if flag is set
		if viper.GetBool(keys.Benchmarking) {
			var err error
			benchmark.BenchmarkFiles, err = benchmark.SetupBenchmarking()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to setup benchmarking: %v\n", err)
				return
			}
		}

		// Setup flags from config file
		if viper.IsSet(keys.ConfigPath) {
			configFile := viper.GetString(keys.ConfigPath)

			cInfo, err := os.Stat(configFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed check for entered config file path %q: %v\n", configFile, err)
				os.Exit(1)
			} else if cInfo.IsDir() {
				fmt.Fprintf(os.Stderr, "config file entered (%s) is a directory, should be a file\n", configFile)
				os.Exit(1)
			}

			// Load in config file
			if configFile != "" {
				if err := loadConfigFile(configFile); err != nil {
					fmt.Fprintf(os.Stderr, "failed loading config file: %v\n", err)
					os.Exit(1)
				}
			}
		}
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		if cmd.Flags().Lookup("help").Changed {
			return nil
		}
		viper.Set("execute", true)
		return execute()
	},
}

// Execute is the primary initializer of Viper.
func Execute() error {
	fmt.Println()
	if err := rootCmd.Execute(); err != nil {
		logging.E("Failed to execute cobra")
		return err
	}
	return nil
}
