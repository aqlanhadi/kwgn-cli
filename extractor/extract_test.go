package extractor

import (
	"testing"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
)

func TestCreateFinalOutput_TransactionOnly(t *testing.T) {
	stmt := common.Statement{
		Source: "test_statement",
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromInt(100)},
			{Sequence: 2, Type: "debit", Amount: decimal.NewFromInt(-50)},
		},
	}

	result := CreateFinalOutput(stmt, true, false)

	transactions, ok := result.([]common.Transaction)
	if !ok {
		t.Fatal("Expected result to be []common.Transaction")
	}

	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
	}
}

func TestCreateFinalOutput_StatementOnly(t *testing.T) {
	stmt := common.Statement{
		Source:          "test_statement",
		StartingBalance: decimal.NewFromInt(100),
		EndingBalance:   decimal.NewFromInt(150),
		TotalCredit:     decimal.NewFromInt(75),
		TotalDebit:      decimal.NewFromInt(-25),
		Nett:            decimal.NewFromInt(50),
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromInt(75)},
			{Sequence: 2, Type: "debit", Amount: decimal.NewFromInt(-25)},
		},
	}

	result := CreateFinalOutput(stmt, false, true)

	outputMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map[string]interface{}")
	}

	// Should have source
	if outputMap["source"] != "test_statement" {
		t.Errorf("Expected source 'test_statement', got '%v'", outputMap["source"])
	}

	// Should NOT have transactions
	if _, exists := outputMap["transactions"]; exists {
		t.Error("Expected no transactions in statement-only output")
	}
}

func TestCreateFinalOutput_WithAccount(t *testing.T) {
	stmt := common.Statement{
		Source: "test_statement",
		Account: common.Account{
			AccountNumber: "123456-789012",
			AccountName:   "JOHN DOE",
			AccountType:   "SAVINGS",
		},
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromInt(100)},
		},
	}

	result := CreateFinalOutput(stmt, false, false)

	outputMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map[string]interface{}")
	}

	// Should have account
	account, exists := outputMap["account"]
	if !exists {
		t.Fatal("Expected account in output")
	}

	acc, ok := account.(common.Account)
	if !ok {
		t.Fatal("Expected account to be common.Account")
	}

	if acc.AccountNumber != "123456-789012" {
		t.Errorf("Expected account number '123456-789012', got '%s'", acc.AccountNumber)
	}
}

func TestCreateFinalOutput_NoAccount(t *testing.T) {
	stmt := common.Statement{
		Source: "test_statement",
		Account: common.Account{
			// All fields empty/default
		},
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromInt(100)},
		},
	}

	result := CreateFinalOutput(stmt, false, false)

	outputMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map[string]interface{}")
	}

	// Should NOT have account when all fields are default
	if _, exists := outputMap["account"]; exists {
		t.Error("Expected no account in output when all fields are default")
	}
}

func TestCreateFinalOutput_ZeroEndingBalance(t *testing.T) {
	stmt := common.Statement{
		Source:                  "test_statement",
		StartingBalance:         decimal.NewFromInt(100),
		EndingBalance:           decimal.Zero,
		CalculatedEndingBalance: decimal.Zero,
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromInt(-100)},
		},
	}

	result := CreateFinalOutput(stmt, false, false)

	outputMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map[string]interface{}")
	}

	// Should have ending_balance even if zero (when transactions exist)
	endingBalance, exists := outputMap["ending_balance"]
	if !exists {
		t.Fatal("Expected ending_balance in output even when zero")
	}

	if endingBalance.(decimal.Decimal).String() != "0" {
		t.Errorf("Expected ending balance '0', got '%v'", endingBalance)
	}
}

func TestCreateFinalOutput_IncludesBalanceFields(t *testing.T) {
	stmt := common.Statement{
		Source:                  "test_statement",
		StartingBalance:         decimal.NewFromFloat(100.50),
		EndingBalance:           decimal.NewFromFloat(150.75),
		CalculatedEndingBalance: decimal.NewFromFloat(150.75),
		TotalCredit:             decimal.NewFromFloat(75.25),
		TotalDebit:              decimal.NewFromFloat(-25.00),
		Nett:                    decimal.NewFromFloat(50.25),
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromFloat(75.25)},
			{Sequence: 2, Type: "debit", Amount: decimal.NewFromFloat(-25.00)},
		},
	}

	result := CreateFinalOutput(stmt, false, false)

	outputMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map[string]interface{}")
	}

	// Check all balance fields exist
	requiredFields := []string{"starting_balance", "ending_balance", "calculated_ending_balance", "total_credit", "total_debit", "nett"}
	for _, field := range requiredFields {
		if _, exists := outputMap[field]; !exists {
			t.Errorf("Expected field '%s' in output", field)
		}
	}
}

func TestCreateFinalOutput_IncludesTransactions(t *testing.T) {
	stmt := common.Statement{
		Source: "test_statement",
		Transactions: []common.Transaction{
			{Sequence: 1, Type: "credit", Amount: decimal.NewFromInt(100)},
			{Sequence: 2, Type: "debit", Amount: decimal.NewFromInt(-50)},
			{Sequence: 3, Type: "credit", Amount: decimal.NewFromInt(25)},
		},
	}

	result := CreateFinalOutput(stmt, false, false)

	outputMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map[string]interface{}")
	}

	transactions, exists := outputMap["transactions"]
	if !exists {
		t.Fatal("Expected transactions in output")
	}

	txs, ok := transactions.([]common.Transaction)
	if !ok {
		t.Fatal("Expected transactions to be []common.Transaction")
	}

	if len(txs) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(txs))
	}
}
