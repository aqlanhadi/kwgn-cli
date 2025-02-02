package tng_web

import (
	"fmt"
	"log"

	"github.com/aqlanhadi/kwgn/extractor/common"
)

func Extract(path string) common.Statement {
	text, err := common.ExtractRowsFromPDF(path)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(text)

	return common.Statement{}
}