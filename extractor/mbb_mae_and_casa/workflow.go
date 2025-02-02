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

	starting_balance, _ := ExtractStartingBalanceFromText(rows)
	ending_balance, _ := ExtractEndingBalanceFromText(rows)
	statement_date, _ := ExtractStatementDateFromText(rows)
	statement_dt, _ := time.ParseInLocation(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.statement_format"), statement_date, time.Local)
	filename := filepath.Base(path)

	statement := &common.Statement{
		Source: strings.TrimSuffix(filename, filepath.Ext(filename)),
		StartingBalance: starting_balance,
		EndingBalance: ending_balance,
		StatementDate: statement_dt,
		Transactions: []common.Transaction{},
		TotalCredit: decimal.Zero,
		TotalDebit: decimal.Zero,
		Nett: decimal.Zero,
	}

	ExtractTransactionsFromText(rows, statement)
	log.Println("\t\t📅", "Statement Month Year:", statement.StatementDate, statement_date)

	if statement.CalculatedEndingBalance.Cmp(ending_balance) == 0 {
		log.Println("\t\t✔️", "Ending balance matches the calculated ending balance")
	} else {
		log.Println("\t\t❌", "Ending balance does not match the calculated ending balance")
	}
	
	return *statement
}