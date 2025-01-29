package extractor

import (
	"log"
	"os"

	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
)

func ExecuteAgainstPath(path string) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		log.Println("📂 Scanning ", path)
		entries, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, e := range entries {
			processFile(path + e.Name())
		}
	} else {
		log.Println("📄 Scanning ", path)
		processFile(path)
	}
}

func processFile(filePath string) {
	if match, err := mbb_mae_and_casa.Match(filePath); err == nil && match {
		log.Println("\t📄 Extracting MBB_MAE transactions from ", filePath)
		// Call the appropriate extraction function here
		return
	}
	if match, err := mbb_2_cc.Match(filePath); err == nil && match {
		log.Println("\t📄 Extracting MBB2CC transactions from ", filePath)
		mbb_2_cc.Extract(filePath)
	}
}
