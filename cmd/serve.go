package cmd

import (
	"log"
	"os"

	"github.com/aqlanhadi/kwgn/api"
	"github.com/spf13/cobra"
)

var (
	servePort string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP API server",
	Long:  `Starts the HTTP API server that accepts PDF files and returns extracted data as JSON.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Configure logging for server mode
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ltime | log.Lmsgprefix)

		// Create API server with configuration
		cfg := api.DefaultConfig()
		if servePort != "" {
			cfg.Port = ":" + servePort
		}
		cfg.LogPrefix = "SERVER: "

		server := api.New(cfg)
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&servePort, "port", "p", "8080", "Port to run the API server on")
}
