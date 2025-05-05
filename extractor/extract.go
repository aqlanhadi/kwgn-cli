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
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// Helper struct to marshal statement details without transactions
type StatementWithoutTransactions struct {
	Source                  string          `json:"source"`
	StartingBalance         decimal.Decimal `json:"starting_balance"`
	EndingBalance           decimal.Decimal `json:"ending_balance"`
	StatementDate           time.Time       `json:"statement_date"`
	Account                 common.Account  `json:"account"`
	TotalCredit             decimal.Decimal `json:"total_credit"`
	TotalDebit              decimal.Decimal `json:"total_debit"`
	Nett                    decimal.Decimal `json:"nett"`
	CalculatedEndingBalance decimal.Decimal `json:"calculated_ending_balance"`
}

func ExecuteAgainstPath(path string, transactionOnly bool, statementOnly bool) {
	startTime := time.Now()
	defer func() {
		log.Printf("Total execution time: %v", time.Since(startTime))
	}()

	if info, err := os.Stat(path); err == nil && info.IsDir() {
		log.Println("Processing directory:", path)
		result := []common.Statement{}

		entries, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range entries {
			fileStartTime := time.Now()
			log.Printf("Processing file: %s", e.Name())
			statement := processFile(path + e.Name())
			if len(statement.Transactions) > 0 {
				log.Printf("Found %d transactions in %s (took %v)", len(statement.Transactions), e.Name(), time.Since(fileStartTime))
				result = append(result, statement)
			} else {
				log.Printf("No transactions found in %s (took %v)", e.Name(), time.Since(fileStartTime))
			}
		}

		var output interface{}
		if transactionOnly {
			transactionList := []common.Transaction{}
			for _, stmt := range result {
				transactionList = append(transactionList, stmt.Transactions...)
			}
			output = transactionList
		} else if statementOnly {
			statementList := []StatementWithoutTransactions{}
			for _, stmt := range result {
				statementList = append(statementList, StatementWithoutTransactions{
					Source:                  stmt.Source,
					StartingBalance:         stmt.StartingBalance,
					EndingBalance:           stmt.EndingBalance,
					StatementDate:           stmt.StatementDate,
					Account:                 stmt.Account,
					TotalCredit:             stmt.TotalCredit,
					TotalDebit:              stmt.TotalDebit,
					Nett:                    stmt.Nett,
					CalculatedEndingBalance: stmt.CalculatedEndingBalance,
				})
			}
			output = statementList
		} else {
			output = result
		}

		as_json, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(as_json))
	} else {
		log.Printf("Processing single file: %s", path)
		fileStartTime := time.Now()
		result := processFile(path)

		if len(result.Transactions) < 1 {
			log.Printf("No transactions found in %s (took %v)", path, time.Since(fileStartTime))
			emptyJSON := struct{}{}
			jsonBytes, _ := json.MarshalIndent(emptyJSON, "", "  ")
			fmt.Println(string(jsonBytes))
			return
		}

		log.Printf("Found %d transactions in %s (took %v)", len(result.Transactions), path, time.Since(fileStartTime))

		var output interface{}
		if transactionOnly {
			output = result.Transactions
		} else if statementOnly {
			output = StatementWithoutTransactions{
				Source:                  result.Source,
				StartingBalance:         result.StartingBalance,
				EndingBalance:           result.EndingBalance,
				StatementDate:           result.StatementDate,
				Account:                 result.Account,
				TotalCredit:             result.TotalCredit,
				TotalDebit:              result.TotalDebit,
				Nett:                    result.Nett,
				CalculatedEndingBalance: result.CalculatedEndingBalance,
			}
		} else {
			output = result
		}

		as_json, _ := json.MarshalIndent(output, "", "  ")
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
