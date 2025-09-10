package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"bytes"
	"encoding/json"

	"github.com/aqlanhadi/kwgn/extractor"
	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Embedded default configuration (from .kwgn-no-acc.yaml)
const defaultConfigYAML = `
accounts:
# No account configurations - will use default statement type detection
statement:
  MAYBANK_CASA_AND_MAE:
    patterns:
      starting_balance: BEGINNING BALANCE\s*([\d,]+\.\d+)
      ending_balance: ENDING BALANCE\s*:\s*([\d,]+\.\d+)
      credit_suffix: CR
      statement_date: (\d{2}\/\d{2}\/\d{2})
      statement_format: _2/01/06
      total_debit: TOTAL DEBIT\s*:\s*([\d,]+\.\d+)
      total_credit: TOTAL CREDIT\s*:\s*([\d,]+\.\d+)
      main_transaction_line: (\d{2}\/\d{2}(?:\/\d{2})?)(.+?)([\d,]*\.\d+[+-])\s([\d,]*\.\d+(DR)?)
      description_transaction_line: (^\s+\S.*)
      amount_debit_suffix: "-"
      balance_debit_suffix: DR
      date_format: "_2/01/06"
  MAYBANK_2_CC:
    patterns:
      credit_suffix: CR
      starting_balance: YOUR PREVIOUS STATEMENT BALANCE\s*([\d,]+\.\d+(?:CR)?)
      ending_balance: SUB TOTAL\/JUMLAH\s*([\d,]+\.\d+(?:CR)?)
      total_credit: TOTAL CREDIT THIS MONTH\s*\(JUMLAH KREDIT\)\s*([\d,]+\.\d+)
      total_debit: TOTAL DEBIT THIS MONTH\s*\(JUMLAH DEBIT\)\s*([\d,]+\.\d+)
      transaction: (\d{2}\/\d{2})\s+(\d{2}\/\d{2})\s+(.+?)\s+([\d,.]+(?:CR)?)\s*$
      statement_date: \d{2}\s(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC)\s\d{2}
      timezone: Asia/Kuala_Lumpur
      statement_format: "_2 Jan 06"
      date_format: "_2/1"
  TNG:
    patterns:
      transaction: '([A-Za-z''0-9: \&-]+?)\s+(\d{2}/\d{2}/\d{4})\s+(\d{2}:\d{2})\s+(.+?)\s+(.+?)\s+(.+?)\s+(.+?)\s+'
      transaction_date: 02/01/2006 15:04
      amount_numbers_pattern: ([+-]?)RM(\d+\.\d+)
      debit_suffix: "-"`

var (
	cfgFile string
	verbose bool
	rootCmd = &cobra.Command{
		Use:   "kwgn [filename]",
		Short: "A brief description of your application",
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
			textOnly := r.FormValue("text_only") == "true" || r.URL.Query().Get("text_only") == "true"
			statementType := r.FormValue("statement_type")
			if statementType == "" {
				statementType = r.URL.Query().Get("statement_type")
			}

			if textOnly {
				// Handle text-only extraction
				rows, err := common.ExtractRowsFromPDFReader(file)
				if err != nil || len(*rows) < 1 {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Could not extract text from file: " + err.Error()))
					return
				}

				text := strings.Join(*rows, "\n")
				textOutput := map[string]string{
					"filename": handler.Filename,
					"text":     text,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(textOutput)
				return
			}

			result := extractor.ProcessReader(file, handler.Filename, statementType)
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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (default is ./.kwgn-no-acc.yaml)")
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
		viper.SetConfigName(".kwgn-no-acc")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file found, use embedded default configuration
			if err := viper.ReadConfig(bytes.NewBufferString(defaultConfigYAML)); err != nil {
				fmt.Printf("Error loading embedded configuration: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("Error reading config file: %v\n", err)
			os.Exit(1)
		}
	}
}
