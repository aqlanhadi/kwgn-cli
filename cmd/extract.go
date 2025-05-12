package cmd

import (
	"log"

	"github.com/aqlanhadi/kwgn/extractor"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	extractCmd = &cobra.Command{
		Use:   "extract",
		Short: "Extracts statement(s)",
		Long:  `Extracts a given statement or statements. \nThis command will match the file names against the provided \nconfig and run the respective extraction pipeline.`,
		Run:   handler,
	}
	transactionOnly bool
	statementOnly   bool
	statementType   string
)

func handler(cmd *cobra.Command, args []string) {
	// Check for mutually exclusive flags
	if transactionOnly && statementOnly {
		log.Fatal("Error: --transaction-only and --statement-only flags are mutually exclusive")
	}

	// Access the configuration using Viper
	target := viper.GetString("target")
	log.Println("📂 Scanning ", target)
	extractor.ExecuteAgainstPath(target, transactionOnly, statementOnly, statementType)
}

func init() {
	rootCmd.AddCommand(extractCmd)

	// Add flags to extract command
	extractCmd.Flags().StringP("folder", "f", ".", "Folder in which kwgn will scan for files")
	extractCmd.Flags().StringP("output", "o", ".", "Folder in which kwgn will save the extracted data")
	extractCmd.Flags().BoolVar(&transactionOnly, "transaction-only", false, "Print only transaction statements")
	extractCmd.Flags().BoolVar(&statementOnly, "statement-only", false, "Print only statement details (excluding transactions)")
	extractCmd.Flags().StringVar(&statementType, "statement-type", "", "Override statement type detection (e.g., MAYBANK_CASA_AND_MAE)")

	extractCmd.MarkFlagRequired("folder")

	// Bind flags to viper
	viper.BindPFlag("target", extractCmd.Flags().Lookup("folder"))
	viper.BindPFlag("output", extractCmd.Flags().Lookup("output"))
	viper.BindPFlag("transaction_only", extractCmd.Flags().Lookup("transaction-only"))
	viper.BindPFlag("statement_only", extractCmd.Flags().Lookup("statement-only"))
	viper.BindPFlag("statement_type", extractCmd.Flags().Lookup("statement-type"))
}
