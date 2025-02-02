package extractor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
	"github.com/spf13/viper"
)

func ExecuteAgainstPath(path string) {

	if info, err := os.Stat(path); err == nil && info.IsDir() {

		result := []common.Statement{}

		log.Println("📂 Scanning ", path)
		entries, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range entries {

			statement := processFile(path + e.Name())
			if len(statement.Transactions) > 0 {
				result = append(result, statement)
			} else {
				
			}
		}

		as_json, _ := json.Marshal(result)
		fmt.Println(string(as_json))
	} else {
		log.Println("📄 Scanning ", path)
		// processFile(path)
		result := processFile(path)

		if len(result.Transactions) < 1 {
			emptyJSON := struct{}{}
			jsonBytes, _ := json.Marshal(emptyJSON)
			fmt.Println(string(jsonBytes)) // Outputs: {}
			return
		}

		as_json, _ := json.Marshal(result)
		fmt.Println(string(as_json))
	}
}

func processFile(filePath string) common.Statement {

	// read file contents
	rows, err := common.ExtractRowsFromPDF(filePath)

	if (err != nil) || (len(*rows) < 1) {
		return common.Statement{}
	}

	text := strings.Join(*rows, "\n")
	accounts := viper.Get("accounts").([]interface{})

	// loop accounts to find match
	for _, acc := range accounts {
		accountMap := acc.(map[string]interface{})
        accountRegex := regexp.MustCompile(accountMap["regex_identifier"].(string))
		
		if accountRegex.Match([]byte(text)) {
			account := common.Account{
				AccountNumber: accountMap["number"].(string),
				AccountType: accountMap["type"].(string),
				AccountName: accountMap["name"].(string),
				DebitCredit: accountMap["drcr"].(string),
			}

			// process based on statement
			switch accountMap["statement_config"].(string) {
			case "MAYBANK_CASA_AND_MAE":
				log.Println("\t📄 Extracting MBB_MAE transactions from ", filePath)
				statement := mbb_mae_and_casa.Extract(filePath, rows)
				statement.Account = account
				return statement
			case "MAYBANK_2_CC":
				log.Println("\t📄 Extracting MBB2CC transactions from ", filePath)
				statement := mbb_2_cc.Extract(filePath, rows)
				statement.Account = account
				return statement
			}
		}
	}

	return common.Statement{}
}
