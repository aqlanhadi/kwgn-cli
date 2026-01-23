package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Embedded default configuration (from .kwgn-no-acc.yaml)
const defaultConfigYAML = `
accounts: []
statement:
  MAYBANK_CASA_AND_MAE:
    patterns:
      starting_balance: 'BEGINNING BALANCE\s*([\d,]+\.\d+)'
      ending_balance: 'ENDING BALANCE\s*:\s*([\d,]+\.\d+)'
      credit_suffix: 'CR'
      statement_date: '(\d{2}\/\d{2}\/\d{2})'
      statement_format: '_2/01/06'
      total_debit: 'TOTAL DEBIT\s*:\s*([\d,]+\.\d+)'
      total_credit: 'TOTAL CREDIT\s*:\s*([\d,]+\.\d+)'
      main_transaction_line: '(\d{2}\/\d{2}(?:\/\d{2})?)(.+?)([\d,]*\.\d+[+-])\s([\d,]*\.\d+(DR)?)'
      description_transaction_line: '(^\s+\S.*)'
      amount_debit_suffix: '-'
      balance_debit_suffix: 'DR'
      date_format: '_2/01/06'
      account_number: '(\d{6}-\d{6}|\d{12})\n(?:.*\n)*?(?:ACCOUNT|NUMBER)'
      account_name: '(\d{2}/\d{2}/\d{2})\n(?:MR / ENCIK |ENCIK |MR |MS |CIK |MADAM )?([A-Z][A-Z\s'']+[A-Z])\nSTATEMENT DATE'
      account_type: '(?:DEPOSITOR\s+([A-Za-z][A-Za-z\-\s]+)|NUMBER\n([A-Za-z][A-Za-z\-\s]+?)\n)'

  MAYBANK_2_CC:
    patterns:
      credit_suffix: 'CR'
      starting_balance: 'YOUR PREVIOUS STATEMENT BALANCE\s*([\d,]+\.\d+(?:CR)?)'
      ending_balance: 'SUB TOTAL\/JUMLAH\s*([\d,]+\.\d+(?:CR)?)'
      total_credit: 'TOTAL CREDIT THIS MONTH\s*\(JUMLAH KREDIT\)\s*([\d,]+\.\d+)'
      total_debit: 'TOTAL DEBIT THIS MONTH\s*\(JUMLAH DEBIT\)\s*([\d,]+\.\d+)'
      transaction: '(\d{2}\/\d{2})\s+(\d{2}\/\d{2})\s+(.+?)\s+([\d,.]+(?:CR)?)\s*$'
      statement_date: '\d{2}\s(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC)\s\d{2}'
      timezone: 'Asia/Kuala_Lumpur'
      statement_format: '_2 Jan 06'
      date_format: '_2/1'
      account_number: '(MASTERCARD|AMEX)\s+:\s+(\d{4}\s\d{4}\s\d{4}\s\d{4}|\d{4}\s\d{6}\s\d{5})'
      account_name: '(?:ENCIK|MR|MS|CIK|MADAM)\s+([A-Z][A-Z\s]+[A-Z])\n'
      account_type: '(MAYBANK 2 (?:PLAT(?:INUM)?|GOLD|CLASSIC)\s+(?:MASTERCARD|AMEX))'

  TNG:
    patterns:
      transaction: '([A-Za-z''0-9: \&-]+?)\s+(\d{2}/\d{2}/\d{4})\s+(\d{2}:\d{2})\s+(.+?)\s+(.+?)\s+(.+?)\s+(.+?)\s+'
      transaction_date: '02/01/2006 15:04'
      amount_numbers_pattern: '([+-]?)RM(\d+\.\d+)'
      debit_suffix: '-'
      account_number: 'Wallet ID\s+(\d+)'
      account_name: 'Registered Name\s+([A-Z][A-Z\s]+[A-Z])\s'
      account_type: 'TNG_EWALLET'
      statement_date: 'Transaction Period\s+\d{2}\s\w+\s\d{4}\s+-\s+(\d{2}\s\w+\s\d{4})'
      statement_date_format: '02 January 2006'

  TNG_EMAIL:
    patterns:
      transaction: '(?s)(\d+/\d+/\d{4})\s+(\w+)\s+([A-Za-z0-9_ ]+?)\s+(\d{11})\s+(.*?)\s+(RM\d+\.\d{2})\s+(RM\d+\.\d{2})'
      date_format: '2/1/2006'
      datetime_pattern: '\d+/\d+/\d{4} \d{2}:\d{2} (AM|PM)'
      datetime_format: '2/1/2006 03:04 PM'
      credit_transaction_types: 'Reload,Transfer to Wallet,Balance Top Up,DUITNOW_RECEI'
      account_number: 'Wallet ID[:\s]+(\d+)'
      account_name: 'Name[:\s]+([A-Z][A-Z\s]+[A-Z])'
      account_type: 'TNG_EWALLET'
`

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
