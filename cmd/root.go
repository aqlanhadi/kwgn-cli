package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"encoding/json"

	"github.com/aqlanhadi/kwgn/extractor"
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

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP API server",
	Run: func(cmd *cobra.Command, args []string) {
		http.HandleFunc("/extract", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				w.Write([]byte("Method not allowed"))
				return
			}

			// Parse multipart form
			err := r.ParseMultipartForm(32 << 20) // 32MB max memory
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Could not parse multipart form: " + err.Error()))
				return
			}

			file, handler, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Could not get uploaded file: " + err.Error()))
				return
			}
			defer file.Close()

			// Flags: allow as form values or query params
			statementOnly := r.FormValue("statement_only") == "true" || r.URL.Query().Get("statement_only") == "true"
			transactionOnly := r.FormValue("transaction_only") == "true" || r.URL.Query().Get("transaction_only") == "true"

			result := extractor.ProcessReader(file, handler.Filename)
			finalOutput := extractor.CreateFinalOutput(result, transactionOnly, statementOnly)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(finalOutput)
		})

		port := ":8080"
		log.Printf("Starting API server on %s", port)
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig, initLogging)

	// Add config flag to root command
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (default is ./.kwgn.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	rootCmd.AddCommand(serveCmd)
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
		viper.AddConfigPath(".")  // First check current directory
		viper.AddConfigPath(home) // Then check home directory
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
