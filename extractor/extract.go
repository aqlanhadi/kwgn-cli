package extractor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
	"github.com/aqlanhadi/kwgn/extractor/tng"
	"github.com/spf13/viper"
)

// Removed StatementWithoutTransactions struct as we'll use maps for marshalling

// CreateFinalOutput prepares the data structure for JSON marshalling based on flags.
func CreateFinalOutput(stmt common.Statement, transactionOnly bool, statementOnly bool) interface{} {
	if transactionOnly {
		return stmt.Transactions // Return only transactions if flag is set
	}

	outputMap := make(map[string]interface{})

	// Always include these fields
	outputMap["source"] = stmt.Source
	outputMap["account"] = stmt.Account // Includes Reconciliable flag now
	outputMap["total_credit"] = stmt.TotalCredit
	outputMap["total_debit"] = stmt.TotalDebit
	outputMap["nett"] = stmt.Nett
	outputMap["transaction_start_date"] = stmt.TransactionStartDate
	outputMap["transaction_end_date"] = stmt.TransactionEndDate
	// if stmt.TransactionStartDate != (time.Time{}) && stmt.TransactionEndDate != (time.Time{}) {
	// 	outputMap["transactions_range"] = fmt.Sprintf("%s - %s", stmt.TransactionStartDate.Format(time.RFC3339), stmt.TransactionEndDate.Format(time.RFC3339))
	// }

	// Include reconcilable fields only if the account is reconcilable
	if stmt.Account.Reconciliable {
		if stmt.StatementDate != nil && !stmt.StatementDate.IsZero() {
			outputMap["statement_date"] = stmt.StatementDate
		}
		if !stmt.StartingBalance.IsZero() {
			outputMap["starting_balance"] = stmt.StartingBalance
		}
		if !stmt.EndingBalance.IsZero() {
			outputMap["ending_balance"] = stmt.EndingBalance
		}
		if !stmt.CalculatedEndingBalance.IsZero() {
			outputMap["calculated_ending_balance"] = stmt.CalculatedEndingBalance
		}
	}

	// Include transactions unless statementOnly flag is set
	if !statementOnly {
		if len(stmt.Transactions) > 0 {
			outputMap["transactions"] = stmt.Transactions
		}
	}

	return outputMap
}

func ExecuteAgainstPath(path string, transactionOnly bool, statementOnly bool, statementType string, textOnly bool) {
	startTime := time.Now()
	defer func() {
		log.Printf("Total execution time: %v", time.Since(startTime))
	}()

	if info, err := os.Stat(path); err == nil && info.IsDir() {
		log.Println("Processing directory:", path)

		if textOnly {
			// For text-only extraction from directory
			entries, err := os.ReadDir(path)
			if err != nil {
				log.Fatal(err)
			}
			allTexts := []map[string]string{}
			for _, e := range entries {
				fileStartTime := time.Now()
				log.Printf("Extracting text from: %s", e.Name())
				filePath := filepath.Join(path, e.Name())
				f, err := os.Open(filePath)
				if err != nil {
					log.Printf("Failed to open file %s: %v", e.Name(), err)
					continue
				}
				defer f.Close()

				rows, err := common.ExtractRowsFromPDFReader(f)
				if err != nil || len(*rows) < 1 {
					log.Printf("Error or no text found in %s: %v", e.Name(), err)
					continue
				}

				text := strings.Join(*rows, "\n")
				allTexts = append(allTexts, map[string]string{
					"filename": e.Name(),
					"text":     text,
				})
				log.Printf("Extracted text from %s (took %v)", e.Name(), time.Since(fileStartTime))
			}

			as_json, _ := json.MarshalIndent(allTexts, "", "  ")
			fmt.Println(string(as_json))
			return
		}

		processedStatements := []common.Statement{}

		entries, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range entries {
			fileStartTime := time.Now()
			log.Printf("Processing file: %s", e.Name())
			filePath := filepath.Join(path, e.Name())
			f, err := os.Open(filePath)
			if err != nil {
				log.Printf("Failed to open file %s: %v", e.Name(), err)
				continue
			}
			defer f.Close()
			statement := ProcessReader(f, filePath, statementType)
			if len(statement.Transactions) > 0 || statement.Account.AccountNumber != "" { // Process if transactions or account found
				log.Printf("Processed %s (took %v)", e.Name(), time.Since(fileStartTime))
				processedStatements = append(processedStatements, statement)
			} else {
				log.Printf("No data found in %s (took %v)", e.Name(), time.Since(fileStartTime))
			}
		}

		// Prepare final output based on flags
		var finalOutput interface{}
		if transactionOnly {
			allTransactions := []common.Transaction{}
			for _, stmt := range processedStatements {
				allTransactions = append(allTransactions, stmt.Transactions...)
			}
			finalOutput = allTransactions
		} else {
			outputList := []interface{}{}
			for _, stmt := range processedStatements {
				outputList = append(outputList, CreateFinalOutput(stmt, false, statementOnly))
			}
			finalOutput = outputList
		}

		as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
		fmt.Println(string(as_json))

	} else {
		log.Printf("Processing single file: %s", path)
		fileStartTime := time.Now()
		f, err := os.Open(path)
		if err != nil {
			log.Printf("Failed to open file %s: %v", path, err)
			return
		}
		defer f.Close()

		if textOnly {
			// For text-only extraction from single file
			rows, err := common.ExtractRowsFromPDFReader(f)
			if err != nil || len(*rows) < 1 {
				log.Printf("Error or no text found in %s: %v", path, err)
				emptyJSON := struct{}{}
				jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
				fmt.Println(string(jsonBytes))
				return
			}

			text := strings.Join(*rows, "\n")
			textOutput := map[string]string{
				"filename": filepath.Base(path),
				"text":     text,
			}

			as_json, _ := json.MarshalIndent(textOutput, "", "  ")
			fmt.Println(string(as_json))
			log.Printf("Extracted text from %s (took %v)", path, time.Since(fileStartTime))
			return
		}

		result := ProcessReader(f, path, statementType)

		if len(result.Transactions) < 1 && result.Account.AccountNumber == "" { // Check if anything was found
			log.Printf("No data found in %s (took %v)", path, time.Since(fileStartTime))
			emptyJSON := struct{}{}
			jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
			fmt.Println(string(jsonBytes))
			return
		}

		log.Printf("Processed %s (took %v)", path, time.Since(fileStartTime))

		// Prepare final output based on flags
		finalOutput := CreateFinalOutput(result, transactionOnly, statementOnly)

		as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
		fmt.Println(string(as_json))
	}
}

// processStatementByType selects and executes the correct extraction logic based on statementConfigName
func processStatementByType(filename string, rows *[]string, account common.Account, statementConfigName string) common.Statement {
	processStartTime := time.Now()
	log.Printf("Processing as %s statement", statementConfigName) // Log added here for consistency

	var statement common.Statement
	switch statementConfigName {
	case "MAYBANK_CASA_AND_MAE":
		statement = mbb_mae_and_casa.Extract(filename, rows)
	case "MAYBANK_2_CC":
		statement = mbb_2_cc.Extract(filename, rows)
	case "TNG":
		statement = tng.Extract(filename, rows)
	default:
		log.Printf("Unknown statement type provided: %s", statementConfigName)
		return common.Statement{} // Return empty if type is unknown
	}

	statement.Account = account
	log.Printf("Statement processing completed (took %v)", time.Since(processStartTime))
	return statement
}

func ProcessReader(reader io.Reader, filename string, statementType string) common.Statement {
	startTime := time.Now()
	log.Printf("Extracting rows from PDF: %s", filename)

	// read file contents
	pdfStartTime := time.Now()
	rows, err := common.ExtractRowsFromPDFReader(reader)
	pdfDuration := time.Since(pdfStartTime)

	if (err != nil) || (len(*rows) < 1) {
		log.Printf("Error or no rows found in %s: %v (PDF extraction took %v)", filename, err, pdfDuration)
		return common.Statement{}
	}

	log.Printf("Successfully extracted %d rows from %s (took %v)", len(*rows), filename, pdfDuration)
	text := strings.Join(*rows, "\n")

	// Check if accounts configuration exists
	accountsConfig := viper.Get("accounts")

	if accountsConfig == nil {
		log.Printf("No accounts configuration found (nil)")
		// If statementType is provided, process without account matching
		if statementType != "" {
			log.Printf("Processing with statement type override: %s", statementType)
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}

		// No accounts config and no statementType override - try all statement types
		log.Printf("Trying all available statement types for %s", filename)
		statementTypes := []string{"MAYBANK_CASA_AND_MAE", "MAYBANK_2_CC", "TNG"}

		for _, stmtType := range statementTypes {
			log.Printf("Attempting to process as %s", stmtType)
			result := processStatementByType(filename, rows, common.Account{}, stmtType)

			// Check if we got a successful result (has transactions or account info)
			if len(result.Transactions) > 0 || result.Account.AccountNumber != "" {
				log.Printf("Successfully processed %s as %s", filename, stmtType)
				return result
			}
		}

		log.Printf("No statement type matched for %s (total processing took %v)", filename, time.Since(startTime))
		return common.Statement{}
	}

	accounts, ok := accountsConfig.([]interface{})
	if !ok {
		log.Printf("Invalid accounts configuration format (not a slice)")
		// If statementType is provided, process without account matching
		if statementType != "" {
			log.Printf("Processing with statement type override: %s", statementType)
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}
		log.Printf("No matching account configuration found for %s (total processing took %v)", filename, time.Since(startTime))
		return common.Statement{}
	}

	// Check if accounts array is empty
	if len(accounts) == 0 {
		log.Printf("Accounts configuration is empty")
		// If statementType is provided, process without account matching
		if statementType != "" {
			log.Printf("Processing with statement type override: %s", statementType)
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}

		// Empty accounts config and no statementType override - try all statement types
		log.Printf("Trying all available statement types for %s", filename)
		statementTypes := []string{"MAYBANK_CASA_AND_MAE", "MAYBANK_2_CC", "TNG"}

		for _, stmtType := range statementTypes {
			log.Printf("Attempting to process as %s", stmtType)
			result := processStatementByType(filename, rows, common.Account{}, stmtType)

			// Check if we got a successful result (has transactions or account info)
			if len(result.Transactions) > 0 || result.Account.AccountNumber != "" {
				log.Printf("Successfully processed %s as %s", filename, stmtType)
				return result
			}
		}

		log.Printf("No statement type matched for %s (total processing took %v)", filename, time.Since(startTime))
		return common.Statement{}
	}

	// loop accounts to find match
	matchStartTime := time.Now()

	// Check for statementType override first
	if statementType != "" {
		log.Printf("Attempting to override statement type with: %s", statementType)
		foundOverride := false
		for _, acc := range accounts {
			accountMap := acc.(map[string]interface{})
			if configName, ok := accountMap["statement_config"].(string); ok && configName == statementType {
				log.Printf("Found matching configuration for override: %s", accountMap["name"].(string))
				account := common.Account{
					AccountNumber: accountMap["number"].(string),
					AccountType:   accountMap["type"].(string),
					AccountName:   accountMap["name"].(string),
					DebitCredit:   accountMap["drcr"].(string),
					Reconciliable: accountMap["reconciliable"].(bool),
				}
				// Directly process based on the overridden statement type
				return processStatementByType(filename, rows, account, statementType) // Call helper function
				// foundOverride = true // This line is now unreachable due to returns in switch cases
				// break
			}
		}
		if !foundOverride {
			log.Printf("Warning: Statement type override '%s' provided, but no matching configuration found. Processing without account details.", statementType)
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}
	} else {
		// Original logic: loop accounts to find match based on regex
		for _, acc := range accounts {
			accountMap := acc.(map[string]interface{})
			accountRegex := regexp.MustCompile(accountMap["regex_identifier"].(string))

			if accountRegex.Match([]byte(text)) {
				log.Printf("Matched account: %s (account matching took %v)", accountMap["name"].(string), time.Since(matchStartTime))
				account := common.Account{
					AccountNumber: accountMap["number"].(string),
					AccountType:   accountMap["type"].(string),
					AccountName:   accountMap["name"].(string),
					DebitCredit:   accountMap["drcr"].(string),
					Reconciliable: accountMap["reconciliable"].(bool),
				}

				// process based on statement config from the matched account
				return processStatementByType(filename, rows, account, accountMap["statement_config"].(string)) // Call helper function
			}
		}
	}

	log.Printf("No matching account configuration found for %s (total processing took %v)", filename, time.Since(startTime))
	return common.Statement{}
}
