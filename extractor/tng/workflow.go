package tng

import (
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func Extract(path string, rows *[]string) common.Statement {
	pages, err := common.ExtractRowsFromPDF(path)
	if err != nil {
		log.Fatal(err)
	}

	transactions := []common.Transaction{}

	var total_debit decimal.Decimal
	var total_credit decimal.Decimal
	var sequence_counter int = 1 // Initialize sequence counter

	for _, row := range *pages {
		match := regexp.MustCompile(viper.GetString("statement.TNG.patterns.transaction"))
		matches := match.FindAllStringSubmatch(row, -1)

		for _, s := range matches {
			description := strings.TrimSpace(s[1])
			description = strings.ReplaceAll(description, "  ", " ")
			date := s[2]
			time_str := s[3]
			// convert date to time.Time
			loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
			if err != nil {
				log.Fatal(err)
			}
			dateTime, err := time.ParseInLocation(viper.GetString("statement.TNG.patterns.transaction_date"), date+" "+time_str, loc)
			if err != nil {
				log.Fatal(err)
			}
			amount_numbers_pattern := regexp.MustCompile(viper.GetString("statement.TNG.patterns.amount_numbers_pattern"))
			amount_numbers_match := amount_numbers_pattern.FindAllStringSubmatch(s[7], -1)
			amount_sign := amount_numbers_match[0][1]

			amount := amount_numbers_match[0][2]
			amount_decimal, _ := decimal.NewFromString(amount_sign + amount)

			var tx_type string

			if amount_sign == viper.GetString("statement.TNG.patterns.debit_suffix") {
				tx_type = "debit"
				total_debit = total_debit.Add(amount_decimal)
			} else {
				tx_type = "credit"
				total_credit = total_credit.Add(amount_decimal)
			}

			transactions = append(transactions, common.Transaction{
				Sequence:     sequence_counter, // Use the counter
				Date:         dateTime,
				Descriptions: []string{description, s[4]},
				Type:         tx_type,
				Amount:       amount_decimal,
				Reference:    s[5],
			})
			sequence_counter++ // Increment the counter
		}
	}

	return common.Statement{
		Transactions: transactions,
		TotalDebit:   total_debit,
		TotalCredit:  total_credit,
		Nett:         total_debit.Add(total_credit),
	}
}
