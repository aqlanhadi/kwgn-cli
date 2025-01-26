package cmd

import (
	"fmt"

	"github.com/aqlanhadi/kwgn/extractor"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var extractCmd = &cobra.Command{
    Use:   "extract",
    Short: "Extracts statement(s)",
    Long: `Extracts a given statement or statements. 
This command will match the file names against the provided 
config and run the respective extraction pipeline.`,
    Run: handler,
}

func handler(cmd *cobra.Command, args []string) {
    // Access the configuration using Viper
    target := viper.GetString("target")
    fmt.Println("scanning ", target)
    extractor.ExecuteAgainstDirectory(target)
}

func init() {
    rootCmd.AddCommand(extractCmd)
    extractCmd.Flags().StringP("folder", "f", ".", "Folder in which kwgn will scan for files")
    viper.BindPFlag("target", extractCmd.Flags().Lookup("folder"))
}