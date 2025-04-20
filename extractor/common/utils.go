package common

import (
	"log"
	"strings"
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
	Reference    string          `json:"ref"`
}

func ExtractRowsFromPDF(path string) (*[]string, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}

	// Pre-allocate slice with estimated capacity
	numPages := r.NumPage()
	estimatedRows := numPages * 100 // Assume average 100 rows per page
	extracted_rows := make([]string, 0, estimatedRows)
	
	for no := 1; no <= numPages; no++ {
		page := r.Page(no)
		rows, err := page.GetTextByRow()
		if err != nil {
			log.Printf("Warning: error getting text from page %d: %v", no, err)
			continue
		}

		for _, row := range rows {
			// Use strings.Builder for efficient string concatenation
			var builder strings.Builder
			builder.Grow(len(row.Content) * 20) // Pre-allocate assuming average 20 chars per content

			for i, text := range row.Content {
				builder.WriteString(text.S)
				if i < len(row.Content)-1 {
					builder.WriteByte(' ')
				}
			}

			if builder.Len() > 0 {
				extracted_rows = append(extracted_rows, builder.String())
			}
		}
	}

	return &extracted_rows, nil
}