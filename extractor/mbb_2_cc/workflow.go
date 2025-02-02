package mbb_2_cc

import (
	"log"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)



func Extract(path string, rows *[]string) common.Statement {
	starting_balance, _ := ExtractStartingBalanceFromText(rows)
	ending_balance, _ := ExtractEndingBalanceFromText(rows)
	statement_date, _ := ExtractStatementDateFromText(rows)

	statement_dt, _ := time.ParseInLocation(viper.GetString("statement.MAYBANK_2_CC.patterns.statement_format"), statement_date, time.Local)

	statement := common.Statement{
		StartingBalance: starting_balance,
		EndingBalance: ending_balance,
		StatementDate: statement_dt,
		Transactions: []common.Transaction{},
		TotalCredit: decimal.Zero,
		TotalDebit: decimal.Zero,
		Nett: decimal.Zero,
	}

	ExtractTransactionsFromText(rows, &statement)

	OrderTransactionsByDate(&statement.Transactions)
	RecalculateBalances(&statement)
	
	log.Println("\t\t📅", "Statement Month Year:", statement.StatementDate.Month(), statement.StatementDate.Year())

	if statement.CalculatedEndingBalance.Cmp(ending_balance) == 0 {
		log.Println("\t\t✔️", "Ending balance matches the calculated ending balance")
	} else {
		log.Println("\t\t❌", "Ending balance does not match the calculated ending balance")
	}

	return statement
}