package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor"
	extractor_common "github.com/aqlanhadi/kwgn/extractor/common"
)

// ImportResult tracks the outcome of an import operation
type ImportResult struct {
	Processed int
	Skipped   int
	Failed    int
	Errors    []string
}

// ImportOptions configures the import behavior
type ImportOptions struct {
	Force         bool   // Force reprocessing of existing statements
	StatementType string // Override auto-detection
	Verbose       bool   // Enable verbose logging
}

// sentinelDate is used for TNG CSV imports to ensure all transactions go to the same statement
// This enables proper deduplication via the unique index on (statement_id, reference)
var sentinelDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// isTNGCSVAccount checks if an account is a TNG CSV export account (not reconciliable)
func isTNGCSVAccount(account extractor_common.Account) bool {
	return account.AccountType == "TNG_CSV_EXPORT" && !account.Reconciliable
}

// ImportFile processes a single PDF/CSV file and stores it in the database
// Returns: processed count, skipped count, failed count, error messages
func (db *DB) ImportFile(ctx context.Context, filePath string, opts ImportOptions) (processed int, skipped int, failed int, errors []string) {
	fileName := filepath.Base(filePath)

	// Extract statements from PDF/CSV (may return multiple for CC with multiple cards or CSV with multiple MFG numbers)
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, 1, []string{fmt.Sprintf("%s: failed to open file: %v", fileName, err)}
	}
	defer f.Close()

	statements := extractor.ProcessReaderMulti(f, filePath, opts.StatementType)

	if len(statements) == 0 {
		return 0, 0, 1, []string{fmt.Sprintf("%s: no statements extracted", fileName)}
	}

	// Process each statement
	for _, statement := range statements {
		// Validate extraction
		if statement.Account.AccountNumber == "" {
			failed++
			errors = append(errors, fmt.Sprintf("%s: no account number extracted", fileName))
			continue
		}
		if statement.StatementDate == nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s [%s]: no statement date extracted", fileName, statement.Account.AccountNumber))
			continue
		}

		// Check if this is a TNG CSV account (requires special idempotent handling)
		isTNGCSV := isTNGCSVAccount(statement.Account)

		// For TNG CSV accounts, use a sentinel date to ensure all transactions go to the same statement
		// This enables proper deduplication across multiple CSV imports
		effectiveStatementDate := *statement.StatementDate
		if isTNGCSV {
			effectiveStatementDate = sentinelDate
		}

		// Get or create account
		accountID, err := db.GetOrCreateAccount(ctx, statement.Account)
		if err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s [%s]: account error: %v", fileName, statement.Account.AccountNumber, err))
			continue
		}

		// Check if statement exists (natural key: account_id + statement_date)
		exists, existingID, err := db.StatementExists(ctx, accountID, effectiveStatementDate)
		if err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s [%s]: check error: %v", fileName, statement.Account.AccountNumber, err))
			continue
		}

		var statementID string

		if isTNGCSV {
			// For TNG CSV: reuse existing statement or create new one
			// Transactions are inserted idempotently (duplicates skipped by reference)
			if exists {
				statementID = existingID
			} else {
				// Create statement with sentinel date
				stmtCopy := statement
				stmtCopy.StatementDate = &effectiveStatementDate
				statementID, err = db.CreateStatement(ctx, accountID, stmtCopy)
				if err != nil {
					failed++
					errors = append(errors, fmt.Sprintf("%s [%s]: statement error: %v", fileName, statement.Account.AccountNumber, err))
					continue
				}
			}

			// Insert transactions idempotently (duplicates by reference are skipped)
			if err := db.CreateTransactionsIdempotent(ctx, statementID, statement.Transactions, true); err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s [%s]: transactions error: %v", fileName, statement.Account.AccountNumber, err))
				continue
			}

			if opts.Verbose {
				log.Printf("OK   %s [%s] (%d transactions, idempotent)", fileName, statement.Account.AccountNumber, len(statement.Transactions))
			}
			processed++
		} else {
			// Standard flow for non-TNG accounts
			if exists && !opts.Force {
				if opts.Verbose {
					log.Printf("SKIP %s [%s] (already exists)", fileName, statement.Account.AccountNumber)
				}
				skipped++
				continue
			}

			// If forcing, delete existing statement first
			if exists && opts.Force {
				if err := db.DeleteStatement(ctx, existingID); err != nil {
					failed++
					errors = append(errors, fmt.Sprintf("%s [%s]: delete error: %v", fileName, statement.Account.AccountNumber, err))
					continue
				}
			}

			// Create statement
			statementID, err = db.CreateStatement(ctx, accountID, statement)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s [%s]: statement error: %v", fileName, statement.Account.AccountNumber, err))
				continue
			}

			// Create transactions
			if err := db.CreateTransactions(ctx, statementID, statement.Transactions); err != nil {
				// Rollback by deleting the statement
				_ = db.DeleteStatement(ctx, statementID)
				failed++
				errors = append(errors, fmt.Sprintf("%s [%s]: transactions error: %v", fileName, statement.Account.AccountNumber, err))
				continue
			}

			if opts.Verbose {
				log.Printf("OK   %s [%s] (%d transactions)", fileName, statement.Account.AccountNumber, len(statement.Transactions))
			}
			processed++
		}
	}

	return processed, skipped, failed, errors
}

// ImportDirectory processes all PDF and CSV files in a directory
func (db *DB) ImportDirectory(ctx context.Context, dirPath string, opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Filter PDF and CSV files
	var dataFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".csv") {
			dataFiles = append(dataFiles, filepath.Join(dirPath, e.Name()))
		}
	}

	log.Printf("Scanning: %s", dirPath)
	log.Printf("Found %d files (PDF/CSV)\n", len(dataFiles))

	for _, filePath := range dataFiles {
		processed, skipped, failed, errors := db.ImportFile(ctx, filePath, opts)

		result.Processed += processed
		result.Skipped += skipped
		result.Failed += failed
		result.Errors = append(result.Errors, errors...)

		// Log failures if verbose
		if opts.Verbose && failed > 0 {
			for _, errMsg := range errors {
				log.Printf("FAIL %s", errMsg)
			}
		}
	}

	return result, nil
}

// Import handles both file and directory imports
func (db *DB) Import(ctx context.Context, path string, opts ImportOptions) (*ImportResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		return db.ImportDirectory(ctx, path, opts)
	}

	// Single file
	result := &ImportResult{}
	processed, skipped, failed, errors := db.ImportFile(ctx, path, opts)

	result.Processed = processed
	result.Skipped = skipped
	result.Failed = failed
	result.Errors = errors

	return result, nil
}
