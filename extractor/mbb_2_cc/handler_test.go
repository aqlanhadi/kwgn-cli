package mbb_2_cc

import (
	"testing"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestExtractStartingBalanceFromText(t *testing.T) {
	viper.SetConfigName(".kwgn") // name of config file (without extension)
	viper.SetConfigType("yaml")  // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("../..") // adjust the path as needed to locate the config file
	err := viper.ReadInConfig()  // Find and read the config file
	assert.NoError(t, err)

	tests := []struct {
		path     string
		expected string
	}{
		{"0000000000000000_20231128.pdf", "437.49"},
		{"0000000000000000_20231228.pdf", "3986.67"},
		{"0000000000000000_20240128.pdf", "5164.19"},
		{"0000000000000000_20240228.pdf", "6155.41"},
		{"0000000000000000_20240328.pdf", "9038.53"},
		{"0000000000000000_20240428.pdf", "3810.78"},
		{"0000000000000000_20240528.pdf", "104.14"},
		{"0000000000000000_20240628.pdf", "24.9"},
		{"0000000000000000_20240728.pdf", "1209.27"},
		{"0000000000000000_20240828.pdf", "0"},
		{"0000000000000000_20240928.pdf", "225"},
		{"0000000000000000_20241028.pdf", "0"},
		{"0000000000000000_20241128.pdf", "246.76"},
	}

	for _, test := range tests {
		text, _ := common.ExtractRowsFromPDF(viper.GetString("target") + test.path)
		result, err := ExtractStartingBalanceFromText(text)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, result.String())
	}
}

func TestExtractEndingBalanceFromText(t *testing.T) {
	viper.SetConfigName(".kwgn") // name of config file (without extension)
	viper.SetConfigType("yaml")  // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("../..") // adjust the path as needed to locate the config file
	err := viper.ReadInConfig()  // Find and read the config file
	assert.NoError(t, err)

	tests := []struct {
		path     string
		expected string
	}{
		{"0000000000000000_20231128.pdf", "3986.67"},
		{"0000000000000000_20231228.pdf", "5164.19"},
		{"0000000000000000_20240128.pdf", "6155.41"},
		{"0000000000000000_20240228.pdf", "9038.53"},
		{"0000000000000000_20240328.pdf", "3810.78"},
		{"0000000000000000_20240428.pdf", "104.14"},
		{"0000000000000000_20240528.pdf", "24.9"},
		{"0000000000000000_20240628.pdf", "1209.27"},
		{"0000000000000000_20240728.pdf", "0"},
		{"0000000000000000_20240828.pdf", "225"},
		{"0000000000000000_20240928.pdf", "0"},
		{"0000000000000000_20241028.pdf", "246.76"},
		{"0000000000000000_20241128.pdf", "0"},

	}

	for _, test := range tests {
		text, _ := common.ExtractRowsFromPDF(viper.GetString("target") + test.path)
		result, err := ExtractEndingBalanceFromText(text)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, result.String())
	}
}