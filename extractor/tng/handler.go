package tng

import (
	"regexp"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/spf13/viper"
)

func Match(fileName string) (bool, error) {
	tng_file_regex, err := regexp.Compile(viper.GetString("statement.TNG.file_regex_pattern"))
	
	if err != nil {
		return false, err
	}

	return tng_file_regex.Match([]byte(fileName)), nil
}

func ExtractTransactionsFromText(rows *[]string) ([]common.Transaction, error) {
	return nil, nil
}

