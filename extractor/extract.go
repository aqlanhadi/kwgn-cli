package extractor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
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
			}
		}

		as_json, _ := json.Marshal(result)
		fmt.Println(string(as_json))
	} else {
		log.Println("📄 Scanning ", path)
		processFile(path)
		result := processFile(path)
		as_json, _ := json.Marshal(result)
		fmt.Println(string(as_json))
	}
}

func processFile(filePath string) common.Statement {
	if match, err := mbb_mae_and_casa.Match(filePath); err == nil && match {
		log.Println("\t📄 Extracting MBB_MAE transactions from ", filePath)
		// Call the appropriate extraction function here
		return mbb_mae_and_casa.Extract(filePath)
	}
	if match, err := mbb_2_cc.Match(filePath); err == nil && match {
		log.Println("\t📄 Extracting MBB2CC transactions from ", filePath)
		return mbb_2_cc.Extract(filePath)
	}

	return common.Statement{}
}
