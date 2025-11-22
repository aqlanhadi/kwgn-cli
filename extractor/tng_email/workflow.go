package tng_email

import (
	"log"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// Extract scans the provided text rows for TNG_EMAIL transaction patterns and returns a structured statement.
func Extract(path string, rows *[]string) common.Statement {
	text := strings.Join(*rows, "\n")

	pattern := viper.GetString("statement.TNG_EMAIL.patterns.transaction")
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("TNG_EMAIL regex compile error: %v", err)
		return common.Statement{
			Source: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		}
	}

	transactions := []common.Transaction{}

	// Use FindAllStringSubmatchIndex to get positions for extracting continuation lines
	matchIndices := re.FindAllStringSubmatchIndex(text, -1)
	nextTransactionRe := regexp.MustCompile(`\n\d+/\d+/\d{4}`)
	asteriskRe := regexp.MustCompile(`\n\*`)

	var total_credit decimal.Decimal
	var total_debit decimal.Decimal

	for i, matchIndex := range matchIndices {
		// Extract the matched groups
		m := re.FindStringSubmatch(text[matchIndex[0]:matchIndex[1]])

		// Find where this transaction ends (start of next transaction or asterisk)
		transactionEnd := len(text)
		if i+1 < len(matchIndices) {
			transactionEnd = matchIndices[i+1][0]
		}

		// Look for next transaction pattern or asterisk in the remaining text
		remainingText := text[matchIndex[1]:transactionEnd]
		if nextMatch := nextTransactionRe.FindStringIndex(remainingText); nextMatch != nil {
			transactionEnd = matchIndex[1] + nextMatch[0]
		} else if asteriskMatch := asteriskRe.FindStringIndex(remainingText); asteriskMatch != nil {
			transactionEnd = matchIndex[1] + asteriskMatch[0]
		}

		// Extract continuation lines (everything after the match until transaction end)
		continuation := strings.TrimSpace(text[matchIndex[1]:transactionEnd])

		// Parse datetime from continuation or fallback to date from match
		var dateTime time.Time
		datetimePattern := viper.GetString("statement.TNG_EMAIL.patterns.datetime_pattern")
		datetimeFormat := viper.GetString("statement.TNG_EMAIL.patterns.datetime_format")

		if dtRe, err := regexp.Compile(datetimePattern); err == nil {
			if dtMatch := dtRe.FindString(continuation); dtMatch != "" {
				dateTime, _ = time.ParseInLocation(datetimeFormat, dtMatch, time.Local)
			}
		}

		if dateTime.IsZero() {
			dateTime, _ = time.ParseInLocation(viper.GetString("statement.TNG_EMAIL.patterns.date_format"), m[1], time.Local)
		}

		amountStr := strings.ReplaceAll(m[6], "RM", "")
		amount, _ := decimal.NewFromString(amountStr)

		balanceStr := strings.ReplaceAll(m[7], "RM", "")
		balance, _ := decimal.NewFromString(balanceStr)

		// Process continuation lines for reference and description
		var contRef []string
		var contDesc []string
		if continuation != "" {
			contSplitted := strings.Split(continuation, "\n")
			for _, cont := range contSplitted {
				cont = strings.TrimSpace(cont)
				if cont == "" {
					continue
				}
				subSplit := strings.Split(cont, " ")
				if len(subSplit) > 0 {
					contRef = append(contRef, strings.TrimSpace(subSplit[0]))
					if len(subSplit) > 1 {
						contDesc = append(contDesc, strings.TrimSpace(strings.Join(subSplit[1:], " ")))
					}
				}
			}
		}

		reference := strings.TrimSpace(strings.Join(append([]string{m[4]}, contRef...), ""))
		description := strings.TrimSpace(strings.Join(contDesc, " "))

		txTypeList := strings.Split(viper.GetString("statement.TNG_EMAIL.patterns.credit_transaction_types"), ",")
		txType := "debit"
		if slices.Contains(txTypeList, m[3]) {
			txType = "credit"
			total_credit = total_credit.Add(amount)
		} else {
			total_debit = total_debit.Add(amount)
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

	stmt := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		TotalCredit:  total_credit,
		TotalDebit:   total_debit,
		Nett:         total_credit.Sub(total_debit),
		Transactions: transactions,
	}

	if len(transactions) > 0 {
		stmt.TransactionStartDate = transactions[0].Date
		stmt.TransactionEndDate = transactions[len(transactions)-1].Date
	}

	return stmt
}
