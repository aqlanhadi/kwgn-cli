package common

import (
	"time"

	"github.com/shopspring/decimal"
)

type Statement struct {
	Source                  string          `json:"source"`
	StartingBalance         decimal.Decimal `json:"starting_balance,omitempty"`
	EndingBalance           decimal.Decimal `json:"ending_balance,omitempty"`
	StatementDate           *time.Time      `json:"statement_date,omitempty"`
	Account                 Account         `json:"account"`
	Transactions            []Transaction   `json:"transactions"`
	TotalCredit             decimal.Decimal `json:"total_credit"`
	TotalDebit              decimal.Decimal `json:"total_debit"`
	Nett                    decimal.Decimal `json:"nett"`
	TransactionStartDate    time.Time       `json:"transaction_start_date,omitempty"`
	TransactionEndDate      time.Time       `json:"transaction_end_date,omitempty"`
	CalculatedEndingBalance decimal.Decimal `json:"calculated_ending_balance"`
}

type Account struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	AccountType   string `json:"account_type"`
	DebitCredit   string `json:"debit_credit"`
	Reconciliable bool   `json:"reconciliable"`
}

type Transaction struct {
	Sequence     int                    `json:"sequence"`
	Date         time.Time              `json:"date"`
	Descriptions []string               `json:"descriptions"`
	Type         string                 `json:"type"`
	Amount       decimal.Decimal        `json:"amount"`
	Balance      decimal.Decimal        `json:"balance"`
	Reference    string                 `json:"ref"`
	Tags         []string               `json:"tags,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}
