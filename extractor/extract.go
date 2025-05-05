package extractor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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

// createFinalOutput prepares the data structure for JSON marshalling based on flags.
func createFinalOutput(stmt common.Statement, transactionOnly bool, statementOnly bool) interface{} {
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
	if stmt.TransactionsRange != "" {
		outputMap["transactions_range"] = stmt.TransactionsRange
	}

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

func ExecuteAgainstPath(path string, transactionOnly bool, statementOnly bool) {
	startTime := time.Now()
	defer func() {
		log.Printf("Total execution time: %v", time.Since(startTime))
	}()

	if info, err := os.Stat(path); err == nil && info.IsDir() {
		log.Println("Processing directory:", path)
		processedStatements := []common.Statement{}

		entries, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range entries {
			fileStartTime := time.Now()
			log.Printf("Processing file: %s", e.Name())
			statement := processFile(path + e.Name())
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
				outputList = append(outputList, createFinalOutput(stmt, false, statementOnly))
			}
			finalOutput = outputList
		}

		as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
		fmt.Println(string(as_json))

	} else {
		log.Printf("Processing single file: %s", path)
		fileStartTime := time.Now()
		result := processFile(path)

		if len(result.Transactions) < 1 && result.Account.AccountNumber == "" { // Check if anything was found
			log.Printf("No data found in %s (took %v)", path, time.Since(fileStartTime))
			emptyJSON := struct{}{}
			jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
			fmt.Println(string(jsonBytes))
			return
		}

		log.Printf("Processed %s (took %v)", path, time.Since(fileStartTime))

		// Prepare final output based on flags
		finalOutput := createFinalOutput(result, transactionOnly, statementOnly)

		as_json, _ := json.MarshalIndent(finalOutput, "", "  ")
		fmt.Println(string(as_json))
	}
}

func processFile(filePath string) common.Statement {
	startTime := time.Now()
	log.Printf("Extracting rows from PDF: %s", filePath)

	// read file contents
	pdfStartTime := time.Now()
	rows, err := common.ExtractRowsFromPDF(filePath)
	pdfDuration := time.Since(pdfStartTime)

	if (err != nil) || (len(*rows) < 1) {
		log.Printf("Error or no rows found in %s: %v (PDF extraction took %v)", filePath, err, pdfDuration)
		return common.Statement{}
	}

	log.Printf("Successfully extracted %d rows from %s (took %v)", len(*rows), filePath, pdfDuration)
	text := strings.Join(*rows, "\n")
	accounts := viper.Get("accounts").([]interface{})

	// loop accounts to find match
	matchStartTime := time.Now()
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

			// process based on statement
			processStartTime := time.Now()
			switch accountMap["statement_config"].(string) {
			case "MAYBANK_CASA_AND_MAE":
				log.Printf("Processing as MAYBANK_CASA_AND_MAE statement")
				statement := mbb_mae_and_casa.Extract(filePath, rows)
				statement.Account = account
				log.Printf("Statement processing completed (took %v)", time.Since(processStartTime))
				return statement
			case "MAYBANK_2_CC":
				log.Printf("Processing as MAYBANK_2_CC statement")
				statement := mbb_2_cc.Extract(filePath, rows)
				statement.Account = account
				log.Printf("Statement processing completed (took %v)", time.Since(processStartTime))
				return statement
			case "TNG":
				log.Printf("Processing as TNG statement")
				statement := tng.Extract(filePath, rows)
				statement.Account = account
				log.Printf("Statement processing completed (took %v)", time.Since(processStartTime))
				return statement
			}
		}
	}

	log.Printf("No matching account configuration found for %s (total processing took %v)", filePath, time.Since(startTime))
	return common.Statement{}
}
