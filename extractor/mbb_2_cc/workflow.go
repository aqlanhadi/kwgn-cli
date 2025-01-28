package mbb_2_cc

import (
	"fmt"
	"log"

	"github.com/aqlanhadi/kwgn/extractor/common"
)



func Extract(path string) {
	log.Println("Extracting ", path)

	log.Println("Extracting text from ", path)
	text, err := common.ExtractRowsFromPDF(path)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println("Extracted text: ", text)
	starting_balance, _ := ExtractStartingBalanceFromText(text)
	ending_balance, _ := ExtractEndingBalanceFromText(text)
	statement_date, _ := ExtractStatementDateFromText(text)
	// cleaned_text, _ := CleanupText(text)
	fmt.Println("Starting balance: ", starting_balance)
	fmt.Println("Ending balance: ", ending_balance)
	fmt.Println("Statement date: ", statement_date)
	// fmt.Println("Cleaned text: ", cleaned_text)

}