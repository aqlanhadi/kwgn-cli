package mbb_2_cc

import (
	"regexp"
	"strings"

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

func ExtractTotalDebitFromText(text string) (float64, error) {
	return 0, nil
}

func ExtractTotalCreditFromText(text string) (float64, error) {
	return 0, nil
}