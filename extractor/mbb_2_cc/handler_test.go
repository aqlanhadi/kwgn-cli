package mbb_2_cc

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	viper.SetConfigName(".kwgn") // name of config file (without extension)
	viper.SetConfigType("yaml")  // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("../..") // adjust the path as needed to locate the config file
	err := viper.ReadInConfig()  // Find and read the config file
	assert.NoError(t, err)

	tests := []struct {
		fileName string
		expected bool
	}{
		{"114013-315457_20231130.pdf", false},
		{"0398121207523300_20231228.pdf", true},
		{"024162342_20231231.pdf", false},
		{"514169996465_20240731.pdf", false},
	}

	for _, test := range tests {
		result, err := Match(test.fileName)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, result)
	}
}