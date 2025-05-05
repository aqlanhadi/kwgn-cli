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
)

func handler(cmd *cobra.Command, args []string) {
	// Access the configuration using Viper
	target := viper.GetString("target")
	log.Println("📂 Scanning ", target)
	extractor.ExecuteAgainstPath(target, transactionOnly)
}

func init() {
	rootCmd.AddCommand(extractCmd)

	// Add flags to extract command
	extractCmd.Flags().StringP("folder", "f", ".", "Folder in which kwgn will scan for files")
	extractCmd.Flags().StringP("config", "c", "", "Config file path (default is ./.kwgn.yaml)")
	extractCmd.Flags().StringP("output", "o", ".", "Folder in which kwgn will save the extracted data")
	extractCmd.Flags().BoolVar(&transactionOnly, "transaction-only", false, "Print only transaction statements")

	extractCmd.MarkFlagRequired("folder")

	// Bind flags to viper
	viper.BindPFlag("target", extractCmd.Flags().Lookup("folder"))
	viper.BindPFlag("config", extractCmd.Flags().Lookup("config"))
	viper.BindPFlag("output", extractCmd.Flags().Lookup("output"))
	viper.BindPFlag("transaction_only", extractCmd.Flags().Lookup("transaction-only"))
}
