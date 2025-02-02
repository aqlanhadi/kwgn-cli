package common

import (
	"time"

	"github.com/dslipak/pdf"
	"github.com/shopspring/decimal"
)

type Statement struct {
	Source 			string 			`json:"source"`
	StartingBalance decimal.Decimal `json:"starting_balance"`
	EndingBalance   decimal.Decimal `json:"ending_balance"`
	StatementDate   time.Time       `json:"statement_date"`
	Account		 	Account         `json:"account"`
	Transactions    []Transaction   `json:"transactions"`
	TotalCredit     decimal.Decimal `json:"total_credit"`
	TotalDebit      decimal.Decimal `json:"total_debit"`
	Nett            decimal.Decimal `json:"nett"`
	CalculatedEndingBalance decimal.Decimal `json:"calculated_ending_balance"`
}

type Account struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	AccountType   string `json:"account_type"`
	DebitCredit   string `json:"debit_credit"`
}

type Transaction struct {
    Sequence     int             `json:"sequence"`
    Date         time.Time       `json:"date"`
    Descriptions []string        `json:"descriptions"`
    Type         string          `json:"type"`
    Amount       decimal.Decimal `json:"amount"`
    Balance      decimal.Decimal `json:"balance"`
}

func ExtractRowsFromPDF(path string) (*[]string, error) {
	r, err := pdf.Open(path)
	// remember close file
	if err != nil {
		return nil, err
	}

	extracted_rows := []string{}
	
	for no := 1; no <= r.NumPage(); no++ { // Loop over each page.
		page := r.Page(no)
		rows, _ := page.GetTextByRow()
		for _, row := range rows { // Loop over each row of text in the page.

			// concatenate all text in the row
			var rowText string
			for _, text := range row.Content {
				rowText += text.S + " "
			}

			// fmt.Println("Page", no, "Row", ri, "Text", rowText)
			extracted_rows = append(extracted_rows, rowText)
		}
	}

	return &extracted_rows, nil
}