package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
    cfgFile string
    verbose bool
    rootCmd = &cobra.Command{
        Use:   "kwgn",
        Short: "A brief description of your application",
        Long:  `kwgn is a utility to extract structured data out of your financial statements`,
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
    
    // Add config flag to root command
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default is ./.kwgn.yaml)")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}

func initLogging() {
    if !verbose {
        log.SetOutput(io.Discard)
    } else {
        log.SetFlags(log.Ltime | log.Lmsgprefix)
        log.SetPrefix("INFO: ")
    }
}

func initConfig() {
    if cfgFile != "" {
        // Use config file from the flag
        viper.SetConfigFile(cfgFile)
    } else {
        // Search for config in current directory and home directory
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)

        // Add config paths in order of priority
        viper.AddConfigPath(".")          // First check current directory
        viper.AddConfigPath(home)         // Then check home directory
        viper.SetConfigName(".kwgn")
        viper.SetConfigType("yaml")
    }

    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            fmt.Printf("No config file found. Please specify one using --config flag\n")
            fmt.Printf("Expected config file: .kwgn.yaml in current directory or home directory\n")
            os.Exit(1)
        } else {
            fmt.Printf("Error reading config file: %v\n", err)
            os.Exit(1)
        }
    }
}