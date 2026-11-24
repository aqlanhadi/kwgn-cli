package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	verbose bool
	rootCmd = &cobra.Command{
		Use:   "kwgn [filename]",
		Short: "Extract structured data from Malaysian bank statements",
		Long:  `kwgn is a utility to extract structured data out of your financial statements`,
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 1 {
				viper.Set("target", args[0])
				handler(extractCmd, []string{})
				return
			}
			cmd.Help()
		},
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig, initLogging)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}

func initLogging() {
	if verbose {
		log.SetFlags(log.Ltime | log.Lmsgprefix | log.Lshortfile)
	} else {
		log.SetFlags(0)
	}
	// Ensure logs go to Stderr so they don't interfere with JSON output on Stdout
	log.SetOutput(os.Stderr)
}

func initConfig() {
	// Statement patterns live with the extractors (extractor/common/patterns.go);
	// the CLI has no config file. Flags are bound through viper in each command.
	viper.AutomaticEnv()
}
