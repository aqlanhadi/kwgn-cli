package mbb_mae_and_casa

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func Extract(path string, rows *[]string) common.Statement {
	startTime := time.Now()
	log.Printf("Starting extraction for MAYBANK_CASA_AND_MAE statement: %s", path)

	balanceStartTime := time.Now()
	starting_balance, _ := ExtractStartingBalanceFromText(rows)
	log.Printf("Extracted starting balance: %s (took %v)", starting_balance.String(), time.Since(balanceStartTime))

	endBalanceStartTime := time.Now()
	ending_balance, _ := ExtractEndingBalanceFromText(rows)
	log.Printf("Extracted ending balance: %s (took %v)", ending_balance.String(), time.Since(endBalanceStartTime))

	dateStartTime := time.Now()
	statement_date, _ := ExtractStatementDateFromText(rows)
	log.Printf("Extracted statement date: %s (took %v)", statement_date, time.Since(dateStartTime))

	statement_dt, _ := time.ParseInLocation(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.statement_format"), statement_date, time.Local)
	filename := filepath.Base(path)

	statement := &common.Statement{
		Source:          strings.TrimSuffix(filename, filepath.Ext(filename)),
		StartingBalance: starting_balance,
		EndingBalance:   ending_balance,
		StatementDate:   &statement_dt,
		Transactions:    []common.Transaction{},
		TotalCredit:     decimal.Zero,
		TotalDebit:      decimal.Zero,
		Nett:            decimal.Zero,
	}

	log.Printf("Extracting transactions from statement")
	transStartTime := time.Now()
	ExtractTransactionsFromText(rows, statement)
	log.Printf("Transaction extraction completed (took %v)", time.Since(transStartTime))

	log.Printf("Statement Month Year: %s %d", statement.StatementDate.Month(), statement.StatementDate.Year())

	if statement.CalculatedEndingBalance.Cmp(ending_balance) == 0 {
		log.Printf("✓ Ending balance matches the calculated ending balance")
	} else {
		log.Printf("✗ Ending balance (%s) does not match the calculated ending balance (%s)",
			ending_balance.String(), statement.CalculatedEndingBalance.String())
	}

	log.Printf("Total MAYBANK_CASA_AND_MAE processing time: %v", time.Since(startTime))
	return *statement
}
