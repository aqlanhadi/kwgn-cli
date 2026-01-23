package tng_email

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

type config struct {
	Transaction            *regexp.Regexp
	NextTransaction        *regexp.Regexp
	Asterisk               *regexp.Regexp
	DatetimePattern        *regexp.Regexp
	AccountNumber          *regexp.Regexp
	AccountName            *regexp.Regexp
	DatetimeFormat         string
	DateFormat             string
	AccountType            string
	CreditTransactionTypes []string
}

func loadConfig() config {
	return config{
		Transaction:            regexp.MustCompile(viper.GetString("statement.TNG_EMAIL.patterns.transaction")),
		NextTransaction:        regexp.MustCompile(`\n\d+/\d+/\d{4}`),
		Asterisk:               regexp.MustCompile(`\n\*`),
		DatetimePattern:        regexp.MustCompile(viper.GetString("statement.TNG_EMAIL.patterns.datetime_pattern")),
		AccountNumber:          regexp.MustCompile(viper.GetString("statement.TNG_EMAIL.patterns.account_number")),
		AccountName:            regexp.MustCompile(viper.GetString("statement.TNG_EMAIL.patterns.account_name")),
		DatetimeFormat:         viper.GetString("statement.TNG_EMAIL.patterns.datetime_format"),
		DateFormat:             viper.GetString("statement.TNG_EMAIL.patterns.date_format"),
		AccountType:            viper.GetString("statement.TNG_EMAIL.patterns.account_type"),
		CreditTransactionTypes: strings.Split(viper.GetString("statement.TNG_EMAIL.patterns.credit_transaction_types"), ","),
	}
}

// Extract scans the provided text rows for TNG_EMAIL transaction patterns and returns a structured statement.
func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()
	text := strings.Join(*rows, "\n")

	statement := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Transactions: []common.Transaction{},
	}

	// Extract account info
	if match := cfg.AccountNumber.FindStringSubmatch(text); len(match) > 1 {
		statement.Account.AccountNumber = strings.TrimSpace(match[1])
	}
	// AccountName is now what was previously AccountType (source identifier)
	statement.Account.AccountName = cfg.AccountType
	statement.Account.AccountType = "" // Leave empty - user can set via web UI

	transactions := []common.Transaction{}
	var totalCredit, totalDebit decimal.Decimal

	matchIndices := cfg.Transaction.FindAllStringSubmatchIndex(text, -1)

	for i, matchIndex := range matchIndices {
		// Extract matched groups
		// m[0] is full match, m[1].. are groups
		// We need the strings, not just indices
		fullMatchStr := text[matchIndex[0]:matchIndex[1]]
		m := cfg.Transaction.FindStringSubmatch(fullMatchStr)

		// Determine end of transaction block
		transactionEnd := len(text)
		if i+1 < len(matchIndices) {
			transactionEnd = matchIndices[i+1][0]
		}

		// Search for boundaries in remaining text
		remainingText := text[matchIndex[1]:transactionEnd]
		if loc := cfg.NextTransaction.FindStringIndex(remainingText); loc != nil {
			transactionEnd = matchIndex[1] + loc[0]
		} else if loc := cfg.Asterisk.FindStringIndex(remainingText); loc != nil {
			transactionEnd = matchIndex[1] + loc[0]
		}

		continuation := strings.TrimSpace(text[matchIndex[1]:transactionEnd])

		// Parse DateTime
		var dateTime time.Time
		if dtMatch := cfg.DatetimePattern.FindString(continuation); dtMatch != "" {
			dateTime, _ = time.ParseInLocation(cfg.DatetimeFormat, dtMatch, time.Local)
		}
		if dateTime.IsZero() {
			// Fallback to date from main match
			dateTime, _ = time.ParseInLocation(cfg.DateFormat, m[1], time.Local)
		}

		// Parse Amounts
		amountStr := strings.ReplaceAll(m[6], "RM", "")
		amount, _ := decimal.NewFromString(amountStr)

		balanceStr := strings.ReplaceAll(m[7], "RM", "")
		balance, _ := decimal.NewFromString(balanceStr)

		// Process Continuation
		var contRef []string
		var contDesc []string

		if continuation != "" {
			lines := strings.Split(continuation, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.Split(line, " ")
				if len(parts) > 0 {
					contRef = append(contRef, strings.TrimSpace(parts[0]))
					if len(parts) > 1 {
						contDesc = append(contDesc, strings.TrimSpace(strings.Join(parts[1:], " ")))
					}
				}
			}
		}

		reference := strings.TrimSpace(strings.Join(append([]string{m[4]}, contRef...), ""))
		description := strings.TrimSpace(strings.Join(contDesc, " "))

		txType := "debit"
		if slices.Contains(cfg.CreditTransactionTypes, m[3]) {
			txType = "credit"
			totalCredit = totalCredit.Add(amount)
		} else {
			totalDebit = totalDebit.Add(amount)
		}

		transactions = append(transactions, common.Transaction{
			Sequence:     i + 1,
			Date:         dateTime,
			Descriptions: []string{m[3], m[5], description},
			Type:         txType,
			Amount:       amount,
			Balance:      balance,
			Reference:    reference,
		})
	}

	statement.TotalCredit = totalCredit
	statement.TotalDebit = totalDebit
	statement.Nett = totalCredit.Sub(totalDebit)
	statement.Transactions = transactions

	if len(transactions) > 0 {
		statement.TransactionStartDate = transactions[0].Date
		statement.TransactionEndDate = transactions[len(transactions)-1].Date
		// Use last transaction date as statement date if not set
		if statement.StatementDate == nil {
			statement.StatementDate = &statement.TransactionEndDate
		}
	}

	return statement
}
