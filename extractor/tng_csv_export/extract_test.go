package tng_csv_export

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestExtractMulti(t *testing.T) {
	// Sample CSV data - a coherent sequence of transactions
	// Starting balance: 100
	// TX1: Usage 10.00 -> Balance 90.00
	// TX2: Usage 5.00 -> Balance 85.00
	// TX3: Reload 50.00 -> Balance 135.00
	// TX4: Usage 15.00 -> Balance 120.00
	csvData := `MFG Number,Trans. No.,Transaction Date/Time,Posted Date,Trans. Type,Sector,Entry Location,Entry SP,Exit Location,Exit SP,Reload Location,Trans. Amount (RM),Balance (RM),Vehicle Class,Device No.,Transaction ID,Vehicle Number
2222222222,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,TOLL,TOLL A,SP_A,TOLL A,SP_A,,10.00,90.00,00,,TX001,
2222222222,2,2025-01-02 10:00:00,2025-01-03 00:00:00,Usage,PARKING,PARK A,SP_B,PARK A,SP_B,,5.00,85.00,00,,TX002,
2222222222,3,2025-01-03 10:00:00,2025-01-04 00:00:00,Reload,INTERNET RELOAD,OTA-TNGD,TD_TNG,OTA-TNGD,TD_TNG,OTA-TNGD,50.00,135.00,00,,TX003,
2222222222,4,2025-01-04 10:00:00,2025-01-05 00:00:00,Usage,RAIL,STATION A,SP_C,STATION B,SP_D,,15.00,120.00,00,,TX004,`

	reader := strings.NewReader(csvData)

	statements, err := ExtractMulti(reader, "test.csv")
	if err != nil {
		t.Fatalf("ExtractMulti failed: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]

	// Check account info
	if stmt.Account.AccountNumber != "2222222222" {
		t.Errorf("Expected account number 2222222222, got %s", stmt.Account.AccountNumber)
	}
	if stmt.Account.AccountName != "TNG_CSV_EXPORT" {
		t.Errorf("Expected account name TNG_CSV_EXPORT, got %s", stmt.Account.AccountName)
	}
	if stmt.Account.AccountType != "" {
		t.Errorf("Expected account type to be empty, got %s", stmt.Account.AccountType)
	}
	if stmt.Account.Reconciliable != false {
		t.Errorf("Expected reconciliable to be false")
	}

	// Check transaction count
	if len(stmt.Transactions) != 4 {
		t.Errorf("Expected 4 transactions, got %d", len(stmt.Transactions))
	}

	// Check balances
	// First (oldest) transaction: 2025-01-01, Amount=10.00, Balance=90.00, Type=Usage
	// Starting balance = 90.00 + 10.00 = 100.00
	expectedStarting := decimal.NewFromFloat(100.00)
	if !stmt.StartingBalance.Equal(expectedStarting) {
		t.Errorf("Expected starting balance %s, got %s", expectedStarting.String(), stmt.StartingBalance.String())
	}

	// Last (newest) transaction: 2025-01-04, Balance=120.00
	expectedEnding := decimal.NewFromFloat(120.00)
	if !stmt.EndingBalance.Equal(expectedEnding) {
		t.Errorf("Expected ending balance %s, got %s", expectedEnding.String(), stmt.EndingBalance.String())
	}

	// Calculated ending balance should match: 100 - 10 - 5 + 50 - 15 = 120
	if !stmt.CalculatedEndingBalance.Equal(stmt.EndingBalance) {
		t.Errorf("Calculated ending balance %s doesn't match ending balance %s",
			stmt.CalculatedEndingBalance.String(), stmt.EndingBalance.String())
	}

	// Verify totals
	expectedTotalDebit := decimal.NewFromFloat(30.00) // 10 + 5 + 15
	if !stmt.TotalDebit.Equal(expectedTotalDebit) {
		t.Errorf("Expected total debit %s, got %s", expectedTotalDebit.String(), stmt.TotalDebit.String())
	}

	expectedTotalCredit := decimal.NewFromFloat(50.00) // 50
	if !stmt.TotalCredit.Equal(expectedTotalCredit) {
		t.Errorf("Expected total credit %s, got %s", expectedTotalCredit.String(), stmt.TotalCredit.String())
	}
}

func TestValidateBalance(t *testing.T) {
	csvData := `MFG Number,Trans. No.,Transaction Date/Time,Posted Date,Trans. Type,Sector,Entry Location,Entry SP,Exit Location,Exit SP,Reload Location,Trans. Amount (RM),Balance (RM),Vehicle Class,Device No.,Transaction ID,Vehicle Number
2222222222,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,TOLL,TOLL A,SP_A,TOLL A,SP_A,,5.00,95.00,00,,TX001,
2222222222,2,2025-01-02 10:00:00,2025-01-03 00:00:00,Reload,INTERNET,RELOAD,SP_B,RELOAD,SP_B,OTA,50.00,145.00,00,,TX002,
2222222222,3,2025-01-03 10:00:00,2025-01-04 00:00:00,Usage,PARKING,PARK A,SP_C,PARK A,SP_C,,10.00,135.00,00,,TX003,`

	reader := strings.NewReader(csvData)

	statements, err := ExtractMulti(reader, "test_balance.csv")
	if err != nil {
		t.Fatalf("ExtractMulti failed: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]

	// Validate balance
	valid, msg := ValidateBalance(stmt)
	if !valid {
		t.Errorf("Balance validation failed: %s", msg)
	}

	// Verify calculation:
	// First: 2025-01-01, Usage 5.00, Balance 95.00 -> Starting = 95 + 5 = 100
	// Then: Reload 50.00 -> 100 + 50 = 150
	// Then: Usage 10.00 -> 150 - 10 = 140? But balance shows 145, then 135
	// Wait, let me recalculate...
	// Actually the CSV rows are sorted by date ascending by ExtractMulti
	// Row 1: Usage 5.00, Balance 95.00 -> Starting = 95 + 5 = 100
	// Row 2: Reload 50.00, Balance 145.00 -> 100 + 50 = 150? No, balance is 145
	// Actually: Starting = 100, after Row 1: 100 - 5 = 95 ✓
	// After Row 2: 95 + 50 = 145 ✓
	// After Row 3: 145 - 10 = 135 ✓
	// So ending = 135, calculated = 100 - 5 + 50 - 10 = 135 ✓

	expectedStart := decimal.NewFromFloat(100.00)
	if !stmt.StartingBalance.Equal(expectedStart) {
		t.Errorf("Expected starting balance %s, got %s", expectedStart.String(), stmt.StartingBalance.String())
	}

	expectedEnd := decimal.NewFromFloat(135.00)
	if !stmt.EndingBalance.Equal(expectedEnd) {
		t.Errorf("Expected ending balance %s, got %s", expectedEnd.String(), stmt.EndingBalance.String())
	}
}

func TestTransactionDataField(t *testing.T) {
	csvData := `MFG Number,Trans. No.,Transaction Date/Time,Posted Date,Trans. Type,Sector,Entry Location,Entry SP,Exit Location,Exit SP,Reload Location,Trans. Amount (RM),Balance (RM),Vehicle Class,Device No.,Transaction ID,Vehicle Number
2222222222,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,TOLL,TOLL ENTRY,SP_A,TOLL EXIT,SP_B,,5.00,95.00,01,DEV123,TX001,ABC1234`

	reader := strings.NewReader(csvData)

	statements, err := ExtractMulti(reader, "test_data.csv")
	if err != nil {
		t.Fatalf("ExtractMulti failed: %v", err)
	}

	if len(statements) != 1 || len(statements[0].Transactions) != 1 {
		t.Fatalf("Expected 1 statement with 1 transaction")
	}

	tx := statements[0].Transactions[0]

	// Check data field contains expected values
	if tx.Data["sector"] != "TOLL" {
		t.Errorf("Expected sector TOLL, got %v", tx.Data["sector"])
	}
	if tx.Data["entry_location"] != "TOLL ENTRY" {
		t.Errorf("Expected entry_location TOLL ENTRY, got %v", tx.Data["entry_location"])
	}
	if tx.Data["exit_location"] != "TOLL EXIT" {
		t.Errorf("Expected exit_location TOLL EXIT, got %v", tx.Data["exit_location"])
	}
	if tx.Data["vehicle_class"] != "01" {
		t.Errorf("Expected vehicle_class 01, got %v", tx.Data["vehicle_class"])
	}
	if tx.Data["device_no"] != "DEV123" {
		t.Errorf("Expected device_no DEV123, got %v", tx.Data["device_no"])
	}
	if tx.Data["vehicle_number"] != "ABC1234" {
		t.Errorf("Expected vehicle_number ABC1234, got %v", tx.Data["vehicle_number"])
	}

	// Check reference is Transaction ID
	if tx.Reference != "TX001" {
		t.Errorf("Expected reference TX001, got %s", tx.Reference)
	}
}

func TestMultipleMFGNumbers(t *testing.T) {
	csvData := `MFG Number,Trans. No.,Transaction Date/Time,Posted Date,Trans. Type,Sector,Entry Location,Entry SP,Exit Location,Exit SP,Reload Location,Trans. Amount (RM),Balance (RM),Vehicle Class,Device No.,Transaction ID,Vehicle Number
1111111111,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,TOLL,TOLL A,SP_A,TOLL A,SP_A,,5.00,95.00,00,,TX001,
2222222222,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,PARKING,PARK A,SP_B,PARK A,SP_B,,3.00,47.00,00,,TX002,
1111111111,2,2025-01-02 10:00:00,2025-01-03 00:00:00,Reload,INTERNET,RELOAD,SP_C,RELOAD,SP_C,OTA,50.00,145.00,00,,TX003,`

	reader := strings.NewReader(csvData)

	statements, err := ExtractMulti(reader, "test_multi.csv")
	if err != nil {
		t.Fatalf("ExtractMulti failed: %v", err)
	}

	if len(statements) != 2 {
		t.Fatalf("Expected 2 statements (one per MFG), got %d", len(statements))
	}

	// Find statements by account number
	var stmt1111, stmt2222 *struct {
		accountNum string
		txCount    int
	}
	for _, stmt := range statements {
		if stmt.Account.AccountNumber == "1111111111" {
			stmt1111 = &struct {
				accountNum string
				txCount    int
			}{stmt.Account.AccountNumber, len(stmt.Transactions)}
		}
		if stmt.Account.AccountNumber == "2222222222" {
			stmt2222 = &struct {
				accountNum string
				txCount    int
			}{stmt.Account.AccountNumber, len(stmt.Transactions)}
		}
	}

	if stmt1111 == nil || stmt1111.txCount != 2 {
		t.Errorf("Expected MFG 1111111111 with 2 transactions")
	}
	if stmt2222 == nil || stmt2222.txCount != 1 {
		t.Errorf("Expected MFG 2222222222 with 1 transaction")
	}
}
