package extractor

import (
	"fmt"
	"log"
	"os"

	"github.com/aqlanhadi/kwgn/extractor/mbb_2_cc"
	"github.com/aqlanhadi/kwgn/extractor/mbb_mae_and_casa"
)

func ExecuteAgainstDirectory(dir string) {

	entries, err := os.ReadDir(dir)
    if err != nil {
        log.Fatal(err)
    }
 
    for _, e := range entries {

		if match, err := mbb_mae_and_casa.Match(e.Name()); err == nil && match {
			// Match mbb_mae_casa_pattern
			// fmt.Println("MBB_MAE > ", e.Name())
			continue
		}

		if match, err := mbb_2_cc.Match(e.Name()); err == nil && match {
			fmt.Println("MBB_2CC > ", dir + e.Name())
			mbb_2_cc.Extract(dir + e.Name())
			// continue
			break
		}
		
    }
}
