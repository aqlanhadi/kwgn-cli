package common

import (
	"regexp"
	"time"

	"github.com/shopspring/decimal"
)

var nonNumericRegex = regexp.MustCompile(`[^0-9.]`)

// CleanDecimal parses a string into a decimal.Decimal, removing non-numeric characters
func CleanDecimal(text string) (decimal.Decimal, error) {

	cleanText := nonNumericRegex.ReplaceAllString(text, "")
	if cleanText == "" {
		return decimal.Zero, nil
	}
	amount, err := decimal.NewFromString(cleanText)
	if err != nil {
		return decimal.Zero, err
	}

	return amount, nil
}

// ParseDate parses a date string using a layout, handling common issues
func ParseDate(layout, value string) (time.Time, error) {
	return time.ParseInLocation(layout, value, time.Local)
}

// FixDateYear adjusts the year of a transaction date if it falls in a different year than the statement
func FixDateYear(txDate time.Time, statementDate time.Time) time.Time {
	if txDate.Year() != statementDate.Year() {
		transactionYear := statementDate.Year()
		// If the date is in the future relative to statement month, it means the transaction is in the previous year
		// e.g. Statement Jan 2023, Transaction Dec -> Dec 2022
		if statementDate.Month() < txDate.Month() {
			transactionYear = statementDate.Year() - 1
		}
		return time.Date(transactionYear, txDate.Month(), txDate.Day(), 0, 0, 0, 0, time.Local)
	}
	return txDate
}
