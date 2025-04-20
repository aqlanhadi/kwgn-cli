package tng

import (
	"fmt"
	"log"
	"strings"

	"github.com/aqlanhadi/kwgn/extractor/common"
)

func Extract(path string, rows *[]string) common.Statement {
	pages, err := common.ExtractRowsFromPDF(path)
	if err != nil {
		log.Fatal(err)
	}

	// this becomes pages tho

	for _, row := range *pages {
		// fmt.Println("--------------------------------")
		// fmt.Println(row)
		// split by 5 spaces
		split := strings.Split(row, "     ")
		// fmt.Println(split)

		// ([A-Za-z: &-]+?)\s+(\d{2}\/\d{2}\/\d{4})\s+(\d{2}:\d{2})\s+(.+?)\s+(.+?)\s+(.+?)\s+(.+?)\s+
		for _, s := range split {
			fmt.Println(s)
		}
	}

	return common.Statement{}
}