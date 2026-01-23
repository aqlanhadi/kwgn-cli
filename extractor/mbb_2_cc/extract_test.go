package mbb_2_cc

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
)

// Mock config for tests - matches the embedded default config structure
const testConfigYAML = `
statement:
  MAYBANK_2_CC:
    patterns:
      credit_suffix: 'CR'
      starting_balance: 'YOUR PREVIOUS STATEMENT BALANCE\s*([\d,]+\.\d+(?:CR)?)'
      ending_balance: 'SUB TOTAL\/JUMLAH\s*([\d,]+\.\d+(?:CR)?)'
      total_credit: 'TOTAL CREDIT THIS MONTH\s*\(JUMLAH KREDIT\)\s*([\d,]+\.\d+)'
      total_debit: 'TOTAL DEBIT THIS MONTH\s*\(JUMLAH DEBIT\)\s*([\d,]+\.\d+)'
      transaction: '(\d{2}\/\d{2})\s+(\d{2}\/\d{2})\s+(.+?)\s+([\d,.]+(?:CR)?)\s*$'
      statement_date: '\d{2}\s(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC)\s\d{2}'
      timezone: 'Asia/Kuala_Lumpur'
      statement_format: '_2 Jan 06'
      date_format: '_2/1'
`

func setupTestConfig() {
	viper.Reset()
	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBufferString(testConfigYAML))
}

// Synthetic test data - mimics real CC statement structure with fake data
// Balance calculation: Starting 100 - 50 (payment) + 75.50 + 25.00 = 150.50
func getTestRowsCC() *[]string {
	rows := []string{
		"STATEMENT OF CREDIT CARD ACCOUNT",
		"PENYATA AKAUN KAD KREDIT",
		" ",
		"    Malayan Banking Berhad (3813-K)",
		"Page/ Halaman                   001 OF",
		"ENCIK JOHN DOE BIN SMITH",
		"Statement Date/ Payment Due Date/",
		"123 JALAN TEST 1/1",
		"Tarikh Penyata  Tarikh Akhir Pembayaran",
		"SECTION 1",
		"12345 TEST CITY",
		"28 NOV 24 18 DEC 24",
		"Menara Maybank",
		"100 Jalan Tun Perak",
		"50050 Kuala Lumpur",
		"Account Number/ Nombor Akaun  Current Balance/ Baki Semasa",
		"1234 5678 9012 3456 0.00 0.00",
		"  YOUR PREVIOUS STATEMENT BALANCE 100.00",
		"Posting Date / Transaction Date / Transaction Description / Amount(RM)",
		"29/10 29/10 PAYMENT RECEIVED 50.00CR",
		"01/11 01/11 ONLINE PURCHASE ABC 75.50",
		"05/11 05/11 RESTAURANT XYZ 25.00",
		"  TOTAL CREDIT THIS MONTH (JUMLAH KREDIT) 50.00",
		"  TOTAL DEBIT THIS MONTH (JUMLAH DEBIT) 100.50",
		"  SUB TOTAL/JUMLAH 150.50",
	}
	return &rows
}

func getTestRowsCCZeroBalance() *[]string {
	rows := []string{
		"STATEMENT OF CREDIT CARD ACCOUNT",
		"28 NOV 24 18 DEC 24",
		"  YOUR PREVIOUS STATEMENT BALANCE 50.00",
		"29/10 29/10 PAYMENT RECEIVED 50.00CR",
		"  TOTAL CREDIT THIS MONTH (JUMLAH KREDIT) 50.00",
		"  TOTAL DEBIT THIS MONTH (JUMLAH DEBIT) 0.00",
		"  SUB TOTAL/JUMLAH 0.00",
	}
	return &rows
}

func TestExtract_StartingBalance(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc_20241128.pdf", rows)

	expected := "100"
	if statement.StartingBalance.String() != expected {
		t.Errorf("Expected starting balance '%s', got '%s'", expected, statement.StartingBalance.String())
	}
}

func TestExtract_EndingBalance(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	expected := "150.5"
	if statement.EndingBalance.String() != expected {
		t.Errorf("Expected ending balance '%s', got '%s'", expected, statement.EndingBalance.String())
	}
}

func TestExtract_EndingBalanceZero(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCCZeroBalance()

	statement := Extract("test_cc.pdf", rows)

	expected := "0"
	if statement.EndingBalance.String() != expected {
		t.Errorf("Expected ending balance '%s', got '%s'", expected, statement.EndingBalance.String())
	}
}

func TestExtract_Transactions(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	if len(statement.Transactions) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(statement.Transactions))
	}
}

func TestExtract_CreditTransaction(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	if len(statement.Transactions) < 1 {
		t.Fatal("Expected at least 1 transaction")
	}

	// Find credit transaction (payment)
	var creditTx *struct {
		Type   string
		Amount string
	}
	for _, tx := range statement.Transactions {
		if tx.Type == "credit" {
			creditTx = &struct {
				Type   string
				Amount string
			}{tx.Type, tx.Amount.String()}
			break
		}
	}

	if creditTx == nil {
		t.Fatal("Expected to find a credit transaction")
	}

	if creditTx.Amount != "-50" {
		t.Errorf("Expected credit amount '-50', got '%s'", creditTx.Amount)
	}
}

func TestExtract_DebitTransaction(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	// Count debit transactions
	debitCount := 0
	for _, tx := range statement.Transactions {
		if tx.Type == "debit" {
			debitCount++
		}
	}

	if debitCount != 2 {
		t.Errorf("Expected 2 debit transactions, got %d", debitCount)
	}
}

func TestExtract_TotalDebitCredit(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	// Total debit should be sum of debit transactions
	expectedDebit := "100.5" // 75.50 + 25.00
	if statement.TotalDebit.String() != expectedDebit {
		t.Errorf("Expected total debit '%s', got '%s'", expectedDebit, statement.TotalDebit.String())
	}

	// Total credit should be negative (payment)
	expectedCredit := "-50"
	if statement.TotalCredit.String() != expectedCredit {
		t.Errorf("Expected total credit '%s', got '%s'", expectedCredit, statement.TotalCredit.String())
	}
}

func TestExtract_StatementDate(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	if statement.StatementDate == nil {
		t.Fatal("Expected statement date to be set")
	}

	expected := "2024-11-28"
	actual := statement.StatementDate.Format("2006-01-02")
	if actual != expected {
		t.Errorf("Expected statement date '%s', got '%s'", expected, actual)
	}
}

func TestExtract_CalculatedEndingBalance(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	// Starting 100 - 50 (credit) + 75.50 + 25.00 = 150.50
	expected := "150.5"
	if statement.CalculatedEndingBalance.String() != expected {
		t.Errorf("Expected calculated ending balance '%s', got '%s'", expected, statement.CalculatedEndingBalance.String())
	}
}

func TestExtract_Nett(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	// Nett = TotalDebit + TotalCredit = 100.5 + (-50) = 50.5
	expected := "50.5"
	if statement.Nett.String() != expected {
		t.Errorf("Expected nett '%s', got '%s'", expected, statement.Nett.String())
	}
}


func TestExtract_Source(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("path/to/0000000000000000_20241128.pdf", rows)

	if statement.Source != "0000000000000000_20241128" {
		t.Errorf("Expected source '0000000000000000_20241128', got '%s'", statement.Source)
	}
}

func TestExtract_TransactionsSortedByDate(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCC()

	statement := Extract("test_cc.pdf", rows)

	if len(statement.Transactions) < 2 {
		t.Skip("Need at least 2 transactions to test sorting")
	}

	// Transactions should be sorted by date
	for i := 1; i < len(statement.Transactions); i++ {
		if statement.Transactions[i].Date.Before(statement.Transactions[i-1].Date) {
			t.Error("Transactions are not sorted by date")
			break
		}
	}
}

func TestExtract_EmptyRows(t *testing.T) {
	setupTestConfig()
	rows := []string{}

	statement := Extract("test.pdf", &rows)

	if len(statement.Transactions) != 0 {
		t.Errorf("Expected 0 transactions for empty rows, got %d", len(statement.Transactions))
	}
}
