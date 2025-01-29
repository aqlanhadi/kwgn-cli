package mbb_2_cc

import (
	"log"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)



func Extract(path string) {
	text, err := common.ExtractRowsFromPDF(path)
	if err != nil {
		log.Fatal(err)
	}
	starting_balance, _ := ExtractStartingBalanceFromText(text)
	ending_balance, _ := ExtractEndingBalanceFromText(text)
	statement_date, _ := ExtractStatementDateFromText(text)

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

	ExtractTransactionsFromText(text, &statement)

	log.Println("\t\t📅", "Statement Month Year:", statement.StatementDate.Month(), statement.StatementDate.Year())

	if statement.CalculatedEndingBalance.Cmp(ending_balance) == 0 {
		log.Println("\t\t✔️", "Ending balance matches the calculated ending balance")
	} else {
		log.Println("\t\t❌", "Ending balance does not match the calculated ending balance")
	}

}