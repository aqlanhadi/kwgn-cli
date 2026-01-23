package tng

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

type config struct {
	Transaction         *regexp.Regexp
	AmountNumbers       *regexp.Regexp
	AccountNumber       *regexp.Regexp
	AccountName         *regexp.Regexp
	StatementDateRegex  *regexp.Regexp
	TransactionDate     string
	DebitSuffix         string
	AccountType         string
	StatementDateFormat string
}

func loadConfig() config {
	return config{
		Transaction:         regexp.MustCompile(viper.GetString("statement.TNG.patterns.transaction")),
		AmountNumbers:       regexp.MustCompile(viper.GetString("statement.TNG.patterns.amount_numbers_pattern")),
		AccountNumber:       regexp.MustCompile(viper.GetString("statement.TNG.patterns.account_number")),
		AccountName:         regexp.MustCompile(viper.GetString("statement.TNG.patterns.account_name")),
		StatementDateRegex:  regexp.MustCompile(viper.GetString("statement.TNG.patterns.statement_date")),
		TransactionDate:     viper.GetString("statement.TNG.patterns.transaction_date"),
		DebitSuffix:         viper.GetString("statement.TNG.patterns.debit_suffix"),
		AccountType:         viper.GetString("statement.TNG.patterns.account_type"),
		StatementDateFormat: viper.GetString("statement.TNG.patterns.statement_date_format"),
	}
}

func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()

	// Join all text for account extraction
	fullText := strings.Join(*rows, "\n")

	statement := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Transactions: []common.Transaction{},
	}

	// Extract account info
	if match := cfg.AccountNumber.FindStringSubmatch(fullText); len(match) > 1 {
		statement.Account.AccountNumber = strings.TrimSpace(match[1])
	}
	// AccountName is now what was previously AccountType (source identifier)
	statement.Account.AccountName = cfg.AccountType
	statement.Account.AccountType = "" // Leave empty - user can set via web UI

	// Extract statement date from transaction period
	loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		log.Printf("Warning: Could not load KL location, using Local: %v", err)
		loc = time.Local
	}
	if match := cfg.StatementDateRegex.FindStringSubmatch(fullText); len(match) > 1 {
		if dt, err := time.ParseInLocation(cfg.StatementDateFormat, strings.TrimSpace(match[1]), loc); err == nil {
			statement.StatementDate = &dt
		}
	}

	transactions := []common.Transaction{}
	var totalDebit, totalCredit decimal.Decimal
	sequence := 0
	var minDate, maxDate time.Time
	firstDateSet := false

	for _, row := range *rows {
		matches := cfg.Transaction.FindAllStringSubmatch(row, -1)

		for _, s := range matches {
			// s indices: 0=full, 1=desc, 2=date, 3=time, 4=loc?, 5=?, 6=?, 7=amount_part
			// Original code:
			// description: s[1]
			// date: s[2], time: s[3]
			// amount is in s[7] (last part usually), parsed with AmountNumbers regex
			
			description := strings.TrimSpace(s[1])
			description = strings.ReplaceAll(description, "  ", " ")

			var descriptions []string
			if strings.HasPrefix(description, "Exit Toll: ") {
				rest := strings.TrimPrefix(description, "Exit Toll: ")
				descriptions = []string{"Exit Toll", rest, s[4]}
			} else {
				descriptions = []string{description, s[4]}
			}

			dateTime, err := time.ParseInLocation(cfg.TransactionDate, s[2]+" "+s[3], loc)
			if err != nil {
				log.Printf("Error parsing date '%s %s': %v", s[2], s[3], err)
				continue
			}

			if !firstDateSet {
				minDate = dateTime
				maxDate = dateTime
				firstDateSet = true
			} else {
				if dateTime.Before(minDate) {
					minDate = dateTime
				}
				if dateTime.After(maxDate) {
					maxDate = dateTime
				}
			}

			amountMatch := cfg.AmountNumbers.FindStringSubmatch(s[7])
			if len(amountMatch) < 3 {
				continue
			}
			
			sign := amountMatch[1]
			val := amountMatch[2]
			amount, _ := decimal.NewFromString(sign + val)
			
			txType := "credit"
			if sign == cfg.DebitSuffix {
				txType = "debit"
				totalDebit = totalDebit.Add(amount)
			} else {
				totalCredit = totalCredit.Add(amount)
			}

			sequence++
			transactions = append(transactions, common.Transaction{
				Sequence:     sequence,
				Date:         dateTime,
				Descriptions: descriptions,
				Type:         txType,
				Amount:       amount,
				Reference:    s[5] + s[6],
			})
		}
	}

	statement.TransactionStartDate = minDate
	statement.TransactionEndDate = maxDate
	statement.Transactions = transactions
	statement.TotalDebit = totalDebit
	statement.TotalCredit = totalCredit
	statement.Nett = totalDebit.Add(totalCredit)

	return statement
}

