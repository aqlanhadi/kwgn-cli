package extractor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
	"github.com/aqlanhadi/kwgn/extractor/tng"
	"github.com/aqlanhadi/kwgn/extractor/tng_csv_export"
	"github.com/aqlanhadi/kwgn/extractor/tng_email"
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
	// Include starting balance if transactions exist (0 is a valid balance for continuity)
	// or if it's explicitly non-zero
	if len(stmt.Transactions) > 0 || !stmt.StartingBalance.IsZero() {
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

		processedStatements, err := ProcessDir(path, statementType)
		if err != nil {
			log.Fatal(err)
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

		results := ProcessReaderMulti(f, path, statementType)

		if len(results) == 0 {
			emptyJSON := struct{}{}
			jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
			fmt.Println(string(jsonBytes))
			return
		}

		// Handle output for multiple statements
		var finalOutput interface{}
		if transactionOnly {
			allTransactions := []common.Transaction{}
			for _, stmt := range results {
				allTransactions = append(allTransactions, stmt.Transactions...)
			}
			finalOutput = allTransactions
		} else if len(results) == 1 {
			finalOutput = CreateFinalOutput(results[0], false, statementOnly)
		} else {
			outputList := []interface{}{}
			for _, stmt := range results {
				outputList = append(outputList, CreateFinalOutput(stmt, false, statementOnly))
			}
			finalOutput = outputList
		}

		as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
		fmt.Println(string(as_json))
	}
}

// ProcessDir extracts every file directly inside dir through the same path as a
// single file and ExtractFile (ProcessReaderMulti), so one file can yield
// several statements: a Maybank 2 PDF carries one per card, a TnG CSV one per
// card number. Subdirectories are skipped.
func ProcessDir(dir string, statementType string) ([]common.Statement, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	statements := []common.Statement{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		filePath := filepath.Join(dir, e.Name())
		f, err := os.Open(filePath)
		if err != nil {
			log.Printf("Failed to open file %s: %v", e.Name(), err)
			continue
		}
		for _, stmt := range ProcessReaderMulti(f, filePath, statementType) {
			if len(stmt.Transactions) > 0 || stmt.Account.AccountNumber != "" {
				statements = append(statements, stmt)
			}
		}
		f.Close()
	}
	return statements, nil
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
	if account.Reconciliable {
		statement.Account.Reconciliable = account.Reconciliable
	}
	return statement
}

// pdfFormats is the order a PDF is tried in when its format is not named.
// MAYBANK_2_CC is tried first, separately, because one PDF can hold several
// cards (see ProcessReaderMulti).
var pdfFormats = []string{"MAYBANK_CASA_AND_MAE", "TNG", "TNG_EMAIL"}

// ProcessReaderMulti reads one PDF or CSV and returns every statement it holds:
// a Maybank 2 card PDF carries one per card, a TnG CSV one per card number.
// statementType names the format ("MAYBANK_2_CC", ...); empty detects it.
// No configuration is consulted: account number, name, type and institution
// come from the document itself.
func ProcessReaderMulti(reader io.Reader, filename string, statementType string) []common.Statement {
	if IsCSVFile(filename) {
		statements, err := ProcessCSVFile(reader, filename, statementType)
		if err != nil {
			log.Printf("Error processing CSV file %s: %v", filename, err)
			return []common.Statement{}
		}
		return statements
	}

	rows, err := common.ExtractRowsFromPDFReader(reader)
	if err != nil || rows == nil || len(*rows) < 1 {
		log.Printf("Error or no rows found in %s: %v", filename, err)
		return []common.Statement{}
	}

	if statementType == "" || statementType == "MAYBANK_2_CC" {
		if statementType == "MAYBANK_2_CC" || mbb_2_cc.HasMultipleCards(rows) {
			if statements := mbb_2_cc.ExtractMulti(filename, rows); len(statements) > 0 {
				return statements
			}
		}
	}

	if statementType != "" {
		result := processStatementByType(filename, rows, common.Account{}, statementType)
		if result.Account.AccountNumber != "" || len(result.Transactions) > 0 {
			return []common.Statement{result}
		}
		return []common.Statement{}
	}

	for _, stmtType := range pdfFormats {
		result := processStatementByType(filename, rows, common.Account{}, stmtType)
		if len(result.Transactions) > 0 || result.Account.AccountNumber != "" {
			return []common.Statement{result}
		}
	}
	return []common.Statement{}
}

// ProcessReader is ProcessReaderMulti for callers that expect one statement:
// it returns the first, or an empty statement.
func ProcessReader(reader io.Reader, filename string, statementType string) common.Statement {
	if statements := ProcessReaderMulti(reader, filename, statementType); len(statements) > 0 {
		return statements[0]
	}
	return common.Statement{}
}

// FileResult is what one file in a folder yielded.
type FileResult struct {
	Path       string
	Statements []common.Statement
	Err        error
}

// ExtractFile reads one statement file (PDF or CSV) and returns every
// statement in it. It is the library entry point: importers call this and
// decide for themselves what to store.
func ExtractFile(path string) ([]common.Statement, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	statements := []common.Statement{}
	for _, stmt := range ProcessReaderMulti(f, path, "") {
		if len(stmt.Transactions) > 0 || stmt.Account.AccountNumber != "" {
			statements = append(statements, stmt)
		}
	}
	if len(statements) == 0 {
		return nil, fmt.Errorf("%s: no statement in a format kwgn reads", filepath.Base(path))
	}
	return statements, nil
}

// ExtractDir runs ExtractFile on every file directly inside dir (sorted by
// name; subdirectories and dotfiles skipped) and reports each file's outcome,
// so one unreadable file does not hide the others.
func ExtractDir(dir string) ([]FileResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	results := []FileResult{}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".pdf" && ext != ".csv" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		statements, err := ExtractFile(path)
		results = append(results, FileResult{Path: path, Statements: statements, Err: err})
	}
	return results, nil
}
