package mbb_2_cc

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func ExtractStartingBalanceFromText(rows *[]string) (decimal.Decimal, error) {
	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.starting_balance"))

	total := decimal.NewFromFloat(0)

	for _, text := range *rows {

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

func ExtractEndingBalanceFromText(rows *[]string) (decimal.Decimal, error) {
	pattern, err := regexp.Compile(viper.GetString("statement.MAYBANK_2_CC.patterns.ending_balance"))

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

		if strings.HasSuffix(strings.TrimSpace(text), viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix")) {
			stripped = "-" + stripped
		}

		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractStatementDateFromText(rows *[]string) (string, error) {

	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.statement_date"))

	for _, text := range *rows {
		match := pattern.FindString(text)
		if match != "" {
			return match, nil
		}
	}

	return "", nil
}

func ExtractTransactionsFromText(rows *[]string, statement *common.Statement) ([]common.Transaction, error) {

	transactions := []common.Transaction{}
	pattern := regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.transaction"))

	sequence := 0
	total_debit := decimal.Zero
	total_credit := decimal.Zero
	balance := statement.StartingBalance

	for _, text := range *rows {
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

		var drcr string
		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(match[4], "")
		amount, _ := decimal.NewFromString(stripped)
		// If transaction is credit, convert to negative
		if strings.HasSuffix(strings.TrimSpace(match[4]), viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix")) {
			amount = amount.Neg()
			total_credit = total_credit.Add(amount)
			drcr = "credit"
		} else {
			total_debit = total_debit.Add(amount)
			drcr = "debit"
		}

		balance = balance.Add(amount)

		// fmt.Println("Date: ", date, "Amount: ", amount)

		transactions = append(transactions, common.Transaction{
			Sequence:     sequence,
			Date:         date,
			Descriptions: []string{match[3]},
			Type:         drcr,
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

func OrderTransactionsByDate(transactions *[]common.Transaction) {
	slices.SortFunc(*transactions, func(a, b common.Transaction) int { return a.Date.Compare(b.Date) })
}

func RecalculateBalances(statement *common.Statement) {
	transactions := statement.Transactions
	balance := statement.StartingBalance

	for i, transaction := range transactions {
		if i == 0 {
			transactions[i].Balance = balance.Add(transaction.Amount)
		} else {
			transactions[i].Balance = transactions[i-1].Balance.Add(transaction.Amount)
		}
	}

	statement.CalculatedEndingBalance = transactions[len(transactions)-1].Balance
}

func ExtractTotalDebitFromText(text string) (float64, error) {
	return 0, nil
}

func ExtractTotalCreditFromText(text string) (float64, error) {
	return 0, nil
}

// TODO: move to singular check before processFile
func ExtractAccountDetailsFromText(rows *[]string) (common.Account, error) {

	// combine all rows to a string
	text := strings.Join(*rows, "\n")
	accounts := viper.Get("statement.MAYBANK_2_CC.accounts").([]interface{})

	fmt.Println(text)
	for _, account := range accounts {
		accountMap := account.(map[string]interface{})
        accountRegex := regexp.MustCompile(accountMap["regex_identifier"].(string))
		
		if accountRegex.Match([]byte(text)) {
			fmt.Println("Found ", accountMap["name"].(string))
			return common.Account{
				AccountNumber: accountMap["number"].(string),
				AccountType: accountMap["type"].(string),
				AccountName: accountMap["name"].(string),
				DebitCredit: accountMap["drcr"].(string),
			}, nil
		}
	}
	
	panic("no account match")
}