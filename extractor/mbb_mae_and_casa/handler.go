package mbb_mae_and_casa

import (
	"regexp"

	"github.com/spf13/viper"
)

func Match(fileName string) (bool, error) {
	mbb_mae_casa_file_regex, err := regexp.Compile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.file_regex_pattern"))
	
	if err != nil {
		return false, err
	}

	return mbb_mae_casa_file_regex.Match([]byte(fileName)), nil
}
