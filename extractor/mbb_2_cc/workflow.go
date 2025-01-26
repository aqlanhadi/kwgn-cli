package mbb_2_cc

import (
	"fmt"
	"log"

	"github.com/aqlanhadi/kwgn/extractor/common"
)



func Extract(path string) {
	log.Println("Extracting ", path)

	log.Println("Extracting text from ", path)
	text, err := common.ExtractTextFromPDF(path)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println("Extracted text: ", text)
	starting_balance, _ := ExtractStartingBalanceFromText(text)
	ending_balance, _ := ExtractEndingBalanceFromText(text)
	fmt.Println("Starting balance: ", starting_balance)
	fmt.Println("Ending balance: ", ending_balance)

}