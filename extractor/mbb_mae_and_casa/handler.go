package mbb_mae_and_casa

import (
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func Match(fileName string) (bool, error) {
	mbb_mae_casa_file_regex, err := regexp.Compile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.file_regex_pattern"))
	
	if err != nil {
		return false, err
	}

	return mbb_mae_casa_file_regex.Match([]byte(fileName)), nil
}

func ExtractStartingBalanceFromText(rows *[]string) (decimal.Decimal, error) {
	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.starting_balance"))

	total := decimal.NewFromFloat(0)

	for _, text := range *rows {

		match := pattern.Match([]byte(text))

		if !match {
			continue
		}

		// fmt.Println("Match: ", text)

		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(text, "")

		if strings.HasSuffix(strings.TrimSpace(text), viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.credit_suffix")) {
			stripped = "-" + stripped
		}

		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractEndingBalanceFromText(rows *[]string) (decimal.Decimal, error) {
	pattern, err := regexp.Compile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.ending_balance"))

	if err != nil {
		return decimal.Zero, err
	}

	total := decimal.NewFromFloat(0)

	for _, text := range *rows {

		match := pattern.Match([]byte(text))

		if !match {
			continue
		}

		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(text, "")

		if strings.HasSuffix(strings.TrimSpace(text), viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.credit_suffix")) {
			stripped = "-" + stripped
		}

		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractStatementDateFromText(rows *[]string) (string, error) {

	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.statement_date"))

	for _, text := range *rows {
		match := pattern.FindString(text)
		if match != "" {
			// fmt.Println("Match: ", match)
			return match, nil
		}
	}

	return "", nil
}

func ExtractTransactionsFromText(rows *[]string, statement *common.Statement) ([]common.Transaction, error) {
	
	mainTransactionRegex := regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.main_transaction_line"))
	descTransactionRegex := regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.description_transaction_line"))

	keptRows := []string{}
	
	for _, row := range *rows {
		if mainTransactionRegex.Match([]byte(row)) || descTransactionRegex.Match([]byte(row)) {
			keptRows = append(keptRows, row)
			// fmt.Println("matched:\t", row)
			continue
		}
		// fmt.Println("unmatched:\t", row)
	}

	transactions := []common.Transaction{}
	var currentTransaction *common.Transaction
	sequence := 0
	total_debit := decimal.Zero
	total_credit := decimal.Zero
	balance := statement.StartingBalance

	for _, line := range keptRows {
		
		line = strings.TrimSpace(line) // Clean up whitespace
		if mainTransactionRegex.MatchString(line) {
			// If a new transaction is found, store the previous one
			if currentTransaction != nil {
				transactions = append(transactions, *currentTransaction)
			}

			sequence++
			// Extract details from regex
			match := mainTransactionRegex.FindStringSubmatch(line)
			date, _ := time.ParseInLocation(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.date_format"), match[1], time.Local)
			// Complete Date
			if date.Year() != statement.StatementDate.Year() {

				transaction_year := statement.StatementDate.Year()
				// If the date is in the future, it means the transaction is in the previous year
				if statement.StatementDate.Month() < date.Month() {
					transaction_year = statement.StatementDate.Year()-1
				}
					
				date = time.Date(transaction_year, date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
			}

			stripped_amount := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(match[3], "")
			amount := decimal.RequireFromString(stripped_amount)

			// If transaction is debit, convert to negative
			if strings.HasSuffix(strings.TrimSpace(match[3]), viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.amount_debit_suffix")) {
				amount = amount.Neg()
				total_debit = total_debit.Add(amount)

			} else {
				total_credit = total_credit.Add(amount)
			}

			balance = balance.Add(amount)

			// // If balance is overdrawn, convert to negative
			// if strings.HasSuffix(strings.TrimSpace(match[3]), viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.balance_overdrawn_suffix")) {
			// 	balance = balance.Neg()
			// }
			
			initialDescription := strings.TrimSpace(match[2])
			currentTransaction = &common.Transaction{
				Sequence: sequence,
				Date: date,
				Descriptions: []string{initialDescription},
				Type: "",
				Amount: amount,
				Balance: balance,
			}



		} else if currentTransaction != nil {
			// Append description lines
			currentTransaction.Descriptions = append(currentTransaction.Descriptions, strings.TrimSpace(line))
		}
	}

	// Append last transaction
	if currentTransaction != nil {
		transactions = append(transactions, *currentTransaction)
	}

	statement.Transactions = transactions
	statement.TotalCredit = total_credit
	statement.TotalDebit = total_debit
	statement.Transactions = transactions
	statement.Nett = total_debit.Add(total_credit)
	statement.CalculatedEndingBalance = balance

	return transactions, nil
}