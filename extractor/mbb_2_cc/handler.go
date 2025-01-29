package mbb_2_cc

import (
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func Match(fileName string) (bool, error) {
	mbb_2_cc, err := regexp.Compile(viper.GetString("statement.MAYBANK_2_CC.file_regex_pattern"))
	
	if err != nil {
		return false, err
	}

	return mbb_2_cc.Match([]byte(fileName)), nil
}

func ExtractStartingBalanceFromText(rows []string) (decimal.Decimal, error) {
	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.starting_balance"))

	total := decimal.NewFromFloat(0)

	for _, text := range rows {

		match := pattern.Match([]byte(text))

		if !match {
			continue
		}

		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(text, "")

		if strings.HasSuffix(strings.TrimSpace(text), viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix")) {
			stripped = "-" + stripped
		}

		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractEndingBalanceFromText(rows []string) (decimal.Decimal, error) {
	pattern, err := regexp.Compile(viper.GetString("statement.MAYBANK_2_CC.patterns.ending_balance"))

	if err != nil {
		return decimal.Zero, err
	}

	total := decimal.NewFromFloat(0)

	for _, text := range rows {

		match := pattern.Match([]byte(text))

		if !match {
			continue
		}

		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(text, "")

		if strings.HasSuffix(strings.TrimSpace(text), viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix")) {
			stripped = "-" + stripped
		}

		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractStatementDateFromText(rows []string) (string, error) {

	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.statement_date"))

	for _, text := range rows {
		match := pattern.FindString(text)
		if match != "" {
			return match, nil
		}
	}

	return "", nil
}

func ExtractTransactionsFromText(rows []string, statement *common.Statement) ([]common.Transaction, error) {

	transactions := []common.Transaction{}
	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.transaction"))

	sequence := 0
	total_debit := decimal.Zero
	total_credit := decimal.Zero
	balance := statement.StartingBalance

	for _, text := range rows {
		match := pattern.FindStringSubmatch(text)

		if len(match) == 0 {
			continue
		}

		sequence += 1
		
		date, _ := time.ParseInLocation(viper.GetString("statement.MAYBANK_2_CC.patterns.date_format"), match[1], time.Local)
		// Fix CC Date
		if date.Year() != statement.StatementDate.Year() {

			transaction_year := statement.StatementDate.Year()
			// If the date is in the future, it means the transaction is in the previous year
			if statement.StatementDate.Month() < date.Month() {
				transaction_year = statement.StatementDate.Year()-1
			}
				
			date = time.Date(transaction_year, date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
		}

		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(match[4], "")
		amount, _ := decimal.NewFromString(stripped)
		// If transaction is credit, convert to negative
		if strings.HasSuffix(strings.TrimSpace(match[4]), viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix")) {
			amount = amount.Neg()
			total_credit = total_credit.Add(amount)
		} else {
			total_debit = total_debit.Add(amount)
		}

		balance = balance.Add(amount)

		// fmt.Println("Date: ", date, "Amount: ", amount)

		transactions = append(transactions, common.Transaction{
			Sequence:     sequence,
			Date:         date,
			Descriptions: []string{},
			Type:         "",
			Amount:       amount,
			Balance:      balance,
		})
	}

	statement.TotalCredit = total_credit
	statement.TotalDebit = total_debit
	statement.Transactions = transactions
	statement.Nett = total_debit.Add(total_credit)
	statement.CalculatedEndingBalance = balance

	return transactions, nil
}


func ExtractTotalDebitFromText(text string) (float64, error) {
	return 0, nil
}

func ExtractTotalCreditFromText(text string) (float64, error) {
	return 0, nil
}