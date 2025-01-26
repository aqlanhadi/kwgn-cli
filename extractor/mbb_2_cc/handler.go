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

func ExtractStartingBalanceFromText(text string) (decimal.Decimal, error) {
	pattern, err := regexp.Compile(viper.GetString("statement.MAYBANK_2_CC.patterns.starting_balance"))

	if err != nil {
		return decimal.Zero, err
	}

	total := decimal.NewFromFloat(0)

	for _, match := range pattern.FindAllStringSubmatch(text, -1) {

		is_credit := strings.HasSuffix(match[1], viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix"))
		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(match[1], "")

		if is_credit {
			stripped = "-" + stripped
		}

		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractEndingBalanceFromText(text string) (decimal.Decimal, error) {
	pattern, err := regexp.Compile(viper.GetString("statement.MAYBANK_2_CC.patterns.ending_balance"))

	if err != nil {
		return decimal.Zero, err
	}

	total := decimal.NewFromFloat(0)

	for _, match := range pattern.FindAllStringSubmatch(text, -1) {

		is_credit := strings.HasSuffix(match[1], viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix"))
		stripped := regexp.MustCompile(`[^0-9.]`).ReplaceAllString(match[1], "")

		if is_credit {
			stripped = "-" + stripped
		}
		
		in_decimal, _ := decimal.NewFromString(stripped)
		total = total.Add(in_decimal)
	}

	return total, nil
}

func ExtractTotalDebitFromText(text string) (float64, error) {
	return 0, nil
}

func ExtractTotalCreditFromText(text string) (float64, error) {
	return 0, nil
}