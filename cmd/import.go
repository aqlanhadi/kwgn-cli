package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aqlanhadi/kwgn/integrations/postgres"
	"github.com/spf13/cobra"
)

var (
	importPath        string
	importDBURL       string
	importForce       bool
	importType        string
	importTimeout     int
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import financial statements into PostgreSQL database",
	Long: `Imports PDF financial statements into a PostgreSQL database.

Supports both single file and directory imports. Uses natural key 
(account_number, statement_date) for deduplication.

Examples:
  kwgn import -f /path/to/statement.pdf --db-url postgresql://user:pass@localhost/db
  kwgn import -f /path/to/statements/ --db-url postgresql://user:pass@localhost/db
  kwgn import -f /path/to/statements/ --db-url postgresql://user:pass@localhost/db --force`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ltime | log.Lmsgprefix)

		// Validate required flags
		if importPath == "" {
			log.Fatal("error: --file/-f is required")
		}
		if importDBURL == "" {
			// Try environment variable
			importDBURL = os.Getenv("DATABASE_URL")
			if importDBURL == "" {
				log.Fatal("error: --db-url or DATABASE_URL environment variable is required")
			}
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(importTimeout)*time.Second)
		defer cancel()

		// Connect to database
		log.Println("Connecting to database...")
		db, err := postgres.Connect(ctx, importDBURL)
		if err != nil {
			log.Fatalf("error: database connection failed: %v", err)
		}
		defer db.Close()
		log.Println("Database connection successful")

		// Ensure schema exists
		log.Println("Ensuring database schema...")
		if err := db.EnsureSchema(ctx); err != nil {
			log.Fatalf("error: schema creation failed: %v", err)
		}
		log.Println("Database schema ready")

		// Run import
		opts := postgres.ImportOptions{
			Force:         importForce,
			StatementType: importType,
			Verbose:       verbose,
		}

		result, err := db.Import(ctx, importPath, opts)
		if err != nil {
			log.Fatalf("error: import failed: %v", err)
		}

		// Print summary
		fmt.Printf("\nComplete: %d processed, %d skipped, %d failed\n",
			result.Processed, result.Skipped, result.Failed)

		if len(result.Errors) > 0 && verbose {
			fmt.Println("\nErrors:")
			for _, e := range result.Errors {
				fmt.Printf("  - %s\n", e)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVarP(&importPath, "file", "f", "", "Path to PDF file or directory (required)")
	importCmd.Flags().StringVar(&importDBURL, "db-url", "", "PostgreSQL connection URL (or set DATABASE_URL env)")
	importCmd.Flags().BoolVar(&importForce, "force", false, "Force reprocessing of existing statements")
	importCmd.Flags().StringVarP(&importType, "type", "t", "", "Statement type override (auto-detected if not set)")
	importCmd.Flags().IntVar(&importTimeout, "timeout", 300, "Operation timeout in seconds")

	importCmd.MarkFlagRequired("file")
}
