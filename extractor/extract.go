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

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
	"github.com/aqlanhadi/kwgn/extractor/tng"
	"github.com/aqlanhadi/kwgn/extractor/tng_csv_export"
	"github.com/aqlanhadi/kwgn/extractor/tng_email"
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
	// Include account only if it has non-default values
	if stmt.Account.AccountNumber != "" ||
		stmt.Account.AccountName != "" ||
		stmt.Account.AccountType != "" ||
		stmt.Account.DebitCredit != "" ||
		stmt.Account.Reconciliable {
		outputMap["account"] = stmt.Account
	}
	outputMap["total_credit"] = stmt.TotalCredit
	outputMap["total_debit"] = stmt.TotalDebit
	outputMap["nett"] = stmt.Nett
	outputMap["transaction_start_date"] = stmt.TransactionStartDate
	outputMap["transaction_end_date"] = stmt.TransactionEndDate
	// if stmt.TransactionStartDate != (time.Time{}) && stmt.TransactionEndDate != (time.Time{}) {
	// 	outputMap["transactions_range"] = fmt.Sprintf("%s - %s", stmt.TransactionStartDate.Format(time.RFC3339), stmt.TransactionEndDate.Format(time.RFC3339))
	// }

	// Include balance fields if they have values
	if stmt.StatementDate != nil && !stmt.StatementDate.IsZero() {
		outputMap["statement_date"] = stmt.StatementDate
	}
	if !stmt.StartingBalance.IsZero() {
		outputMap["starting_balance"] = stmt.StartingBalance
	}
	// Include ending balance if transactions exist (0 is a valid balance)
	if len(stmt.Transactions) > 0 {
		outputMap["ending_balance"] = stmt.EndingBalance
		outputMap["calculated_ending_balance"] = stmt.CalculatedEndingBalance
	} else {
		// If no transactions, only include if non-zero (to avoid showing 0 for unextracted values)
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
	if info, err := os.Stat(path); err == nil && info.IsDir() {

		if textOnly {
			// For text-only extraction from directory
			entries, err := os.ReadDir(path)
			if err != nil {
				log.Fatal(err)
			}
			allTexts := []map[string]string{}
			for _, e := range entries {
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
			filePath := filepath.Join(path, e.Name())
			f, err := os.Open(filePath)
			if err != nil {
				log.Printf("Failed to open file %s: %v", e.Name(), err)
				continue
			}
			defer f.Close()
			statement := ProcessReader(f, filePath, statementType)
			if len(statement.Transactions) > 0 || statement.Account.AccountNumber != "" { // Process if transactions or account found
				processedStatements = append(processedStatements, statement)
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
		f, err := os.Open(path)
		if err != nil {
			log.Printf("Failed to open file %s: %v", path, err)
			return
		}
		defer f.Close()

		// Handle CSV files
		if IsCSVFile(path) {
			statements, err := ProcessCSVFile(f, path, statementType)
			if err != nil {
				log.Printf("Error processing CSV file %s: %v", path, err)
				emptyJSON := struct{}{}
				jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
				fmt.Println(string(jsonBytes))
				return
			}

			// Validate balances for each statement
			for _, stmt := range statements {
				valid, msg := tng_csv_export.ValidateBalance(stmt)
				if valid {
					log.Printf("OK [%s] %s", stmt.Account.AccountNumber, msg)
				} else {
					log.Printf("WARN [%s] %s", stmt.Account.AccountNumber, msg)
				}
			}

			// Prepare final output based on flags
			var finalOutput interface{}
			if transactionOnly {
				allTransactions := []common.Transaction{}
				for _, stmt := range statements {
					allTransactions = append(allTransactions, stmt.Transactions...)
				}
				finalOutput = allTransactions
			} else if len(statements) == 1 {
				finalOutput = CreateFinalOutput(statements[0], false, statementOnly)
			} else {
				outputList := []interface{}{}
				for _, stmt := range statements {
					outputList = append(outputList, CreateFinalOutput(stmt, false, statementOnly))
				}
				finalOutput = outputList
			}

			as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
			fmt.Println(string(as_json))
			return
		}

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
			return
		}

		result := ProcessReader(f, path, statementType)

		if len(result.Transactions) < 1 && result.Account.AccountNumber == "" { // Check if anything was found
			emptyJSON := struct{}{}
			jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
			fmt.Println(string(jsonBytes))
			return
		}

		// Prepare final output based on flags
		finalOutput := CreateFinalOutput(result, transactionOnly, statementOnly)

		as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
		fmt.Println(string(as_json))
	}
}

// IsCSVFile checks if the file is a CSV based on extension
func IsCSVFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".csv")
}

// detectCSVType attempts to detect the CSV type from its header
func detectCSVType(reader io.Reader) (string, io.Reader, error) {
	// We need to peek at the header without consuming the reader
	// So we'll read some bytes and reconstruct the reader
	firstBytes := make([]byte, 500)
	n, err := reader.Read(firstBytes)
	if err != nil && err != io.EOF {
		return "", reader, err
	}

	header := string(firstBytes[:n])

	// Check for TNG CSV export signature
	if strings.Contains(header, "MFG Number,Trans. No.,Transaction Date/Time,Posted Date,Trans. Type,Sector") {
		// Reconstruct reader with the read bytes plus remaining
		newReader := io.MultiReader(strings.NewReader(string(firstBytes[:n])), reader)
		return "TNG_CSV_EXPORT", newReader, nil
	}

	// Unknown CSV type - reconstruct reader
	newReader := io.MultiReader(strings.NewReader(string(firstBytes[:n])), reader)
	return "", newReader, fmt.Errorf("unknown CSV format")
}

// ProcessCSVFile processes a CSV file and returns statements
func ProcessCSVFile(reader io.Reader, filename string, statementType string) ([]common.Statement, error) {
	// If no type specified, try to detect from header
	if statementType == "" {
		detectedType, newReader, err := detectCSVType(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to detect CSV type: %w", err)
		}
		statementType = detectedType
		reader = newReader
	}

	// Route to appropriate CSV extractor
	switch statementType {
	case "TNG_CSV_EXPORT":
		return tng_csv_export.ExtractMulti(reader, filename)
	default:
		return nil, fmt.Errorf("unsupported CSV statement type: %s", statementType)
	}
}

// processStatementByType selects and executes the correct extraction logic based on statementConfigName
func processStatementByType(filename string, rows *[]string, account common.Account, statementConfigName string) common.Statement {
	var statement common.Statement
	switch statementConfigName {
	case "MAYBANK_CASA_AND_MAE":
		statement = mbb_mae_and_casa.Extract(filename, rows)
	case "MAYBANK_2_CC":
		statement = mbb_2_cc.Extract(filename, rows)
	case "TNG":
		statement = tng.Extract(filename, rows)
	case "TNG_EMAIL":
		statement = tng_email.Extract(filename, rows)
	case "TNG_CSV_EXPORT":
		// This shouldn't be called for CSV files; use ProcessCSVFile instead
		log.Printf("TNG_CSV_EXPORT should be processed via ProcessCSVFile, not processStatementByType")
		return common.Statement{}
	default:
		log.Printf("Unknown statement type provided: %s", statementConfigName)
		return common.Statement{} // Return empty if type is unknown
	}

	// Merge account info: config values take precedence over extracted values
	// but only if they are non-empty
	if account.AccountNumber != "" {
		statement.Account.AccountNumber = account.AccountNumber
	}
	if account.AccountName != "" {
		statement.Account.AccountName = account.AccountName
	}
	if account.AccountType != "" {
		statement.Account.AccountType = account.AccountType
	}
	if account.DebitCredit != "" {
		statement.Account.DebitCredit = account.DebitCredit
	}
	if account.Reconciliable {
		statement.Account.Reconciliable = account.Reconciliable
	}
	return statement
}

// ProcessReaderMulti processes a PDF/CSV and returns multiple statements if applicable (e.g., CC with multiple cards, CSV with multiple accounts)
func ProcessReaderMulti(reader io.Reader, filename string, statementType string) []common.Statement {
	// Handle CSV files
	if IsCSVFile(filename) {
		statements, err := ProcessCSVFile(reader, filename, statementType)
		if err != nil {
			log.Printf("Error processing CSV file %s: %v", filename, err)
			return []common.Statement{}
		}
		return statements
	}

	// read file contents for PDF
	rows, err := common.ExtractRowsFromPDFReader(reader)

	if (err != nil) || (len(*rows) < 1) {
		log.Printf("Error or no rows found in %s: %v", filename, err)
		return []common.Statement{}
	}

	// For CC statements, check if multiple cards exist
	if statementType == "" || statementType == "MAYBANK_2_CC" {
		// Check if this looks like a CC statement with multiple cards
		if mbb_2_cc.HasMultipleCards(rows) || statementType == "MAYBANK_2_CC" {
			statements := mbb_2_cc.ExtractMulti(filename, rows)
			if len(statements) > 0 {
				return statements
			}
		}
	}

	// For other statement types or fallback, use single extraction
	// Re-open is not possible, so we need to process with the rows we have
	text := strings.Join(*rows, "\n")

	// Check if accounts configuration exists
	accountsConfig := viper.Get("accounts")

	if accountsConfig == nil || len(accountsConfig.([]interface{})) == 0 {
		// If statementType is provided, process without account matching
		if statementType != "" {
			result := processStatementByType(filename, rows, common.Account{}, statementType)
			if result.Account.AccountNumber != "" || len(result.Transactions) > 0 {
				return []common.Statement{result}
			}
			return []common.Statement{}
		}

		// No accounts config and no statementType override - try all statement types
		statementTypes := []string{"MAYBANK_CASA_AND_MAE", "MAYBANK_2_CC", "TNG", "TNG_EMAIL"}

		for _, stmtType := range statementTypes {
			if stmtType == "MAYBANK_2_CC" {
				// Already tried multi above
				continue
			}
			result := processStatementByType(filename, rows, common.Account{}, stmtType)
			if len(result.Transactions) > 0 || result.Account.AccountNumber != "" {
				return []common.Statement{result}
			}
		}

		return []common.Statement{}
	}

	accounts := accountsConfig.([]interface{})

	// Check for statementType override first
	if statementType != "" {
		for _, acc := range accounts {
			accountMap := acc.(map[string]interface{})
			if configName, ok := accountMap["statement_config"].(string); ok && configName == statementType {
				account := common.Account{
					AccountNumber: accountMap["number"].(string),
					AccountType:   accountMap["type"].(string),
					AccountName:   accountMap["name"].(string),
					DebitCredit:   accountMap["drcr"].(string),
					Reconciliable: accountMap["reconciliable"].(bool),
				}
				result := processStatementByType(filename, rows, account, statementType)
				return []common.Statement{result}
			}
		}
		log.Printf("Warning: Statement type override '%s' provided, but no matching configuration found.", statementType)
		result := processStatementByType(filename, rows, common.Account{}, statementType)
		if result.Account.AccountNumber != "" || len(result.Transactions) > 0 {
			return []common.Statement{result}
		}
		return []common.Statement{}
	}

	// Original logic: loop accounts to find match based on regex
	for _, acc := range accounts {
		accountMap := acc.(map[string]interface{})
		accountRegex := regexp.MustCompile(accountMap["regex_identifier"].(string))

		if accountRegex.Match([]byte(text)) {
			account := common.Account{
				AccountNumber: accountMap["number"].(string),
				AccountType:   accountMap["type"].(string),
				AccountName:   accountMap["name"].(string),
				DebitCredit:   accountMap["drcr"].(string),
				Reconciliable: accountMap["reconciliable"].(bool),
			}
			result := processStatementByType(filename, rows, account, accountMap["statement_config"].(string))
			return []common.Statement{result}
		}
	}

	return []common.Statement{}
}

func ProcessReader(reader io.Reader, filename string, statementType string) common.Statement {
	// read file contents
	rows, err := common.ExtractRowsFromPDFReader(reader)

	if (err != nil) || (len(*rows) < 1) {
		log.Printf("Error or no rows found in %s: %v", filename, err)
		return common.Statement{}
	}

	text := strings.Join(*rows, "\n")

	// Check if accounts configuration exists
	accountsConfig := viper.Get("accounts")

	if accountsConfig == nil {
		// If statementType is provided, process without account matching
		if statementType != "" {
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}

		// No accounts config and no statementType override - try all statement types
		statementTypes := []string{"MAYBANK_CASA_AND_MAE", "MAYBANK_2_CC", "TNG", "TNG_EMAIL"}

		for _, stmtType := range statementTypes {
			result := processStatementByType(filename, rows, common.Account{}, stmtType)

			// Check if we got a successful result (has transactions or account info)
			if len(result.Transactions) > 0 || result.Account.AccountNumber != "" {
				return result
			}
		}

		return common.Statement{}
	}

	accounts, ok := accountsConfig.([]interface{})
	if !ok {
		log.Printf("Invalid accounts configuration format (not a slice)")
		// If statementType is provided, process without account matching
		if statementType != "" {
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}
		return common.Statement{}
	}

	// Check if accounts array is empty
	if len(accounts) == 0 {
		// If statementType is provided, process without account matching
		if statementType != "" {
			return processStatementByType(filename, rows, common.Account{}, statementType)
		}

		// Empty accounts config and no statementType override - try all statement types
		statementTypes := []string{"MAYBANK_CASA_AND_MAE", "MAYBANK_2_CC", "TNG", "TNG_EMAIL"}

		for _, stmtType := range statementTypes {
			result := processStatementByType(filename, rows, common.Account{}, stmtType)

			// Check if we got a successful result (has transactions or account info)
			if len(result.Transactions) > 0 || result.Account.AccountNumber != "" {
				return result
			}
		}

		return common.Statement{}
	}

	// loop accounts to find match

	// Check for statementType override first
	if statementType != "" {
		foundOverride := false
		for _, acc := range accounts {
			accountMap := acc.(map[string]interface{})
			if configName, ok := accountMap["statement_config"].(string); ok && configName == statementType {
				account := common.Account{
					AccountNumber: accountMap["number"].(string),
					AccountType:   accountMap["type"].(string),
					AccountName:   accountMap["name"].(string),
					DebitCredit:   accountMap["drcr"].(string),
					Reconciliable: accountMap["reconciliable"].(bool),
				}
				// Directly process based on the overridden statement type
				return processStatementByType(filename, rows, account, statementType) // Call helper function
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

	return common.Statement{}
}
