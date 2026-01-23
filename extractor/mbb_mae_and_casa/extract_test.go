package mbb_mae_and_casa

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
)

// Mock config for tests - matches the embedded default config structure
const testConfigYAML = `
statement:
  MAYBANK_CASA_AND_MAE:
    patterns:
      starting_balance: 'BEGINNING BALANCE\s*([\d,]+\.\d+)'
      ending_balance: 'ENDING BALANCE\s*:\s*([\d,]+\.\d+)'
      credit_suffix: 'CR'
      statement_date: '(\d{2}\/\d{2}\/\d{2})'
      statement_format: '_2/01/06'
      total_debit: 'TOTAL DEBIT\s*:\s*([\d,]+\.\d+)'
      total_credit: 'TOTAL CREDIT\s*:\s*([\d,]+\.\d+)'
      main_transaction_line: '(\d{2}\/\d{2}(?:\/\d{2})?)(.+?)([\d,]*\.\d+[+-])\s([\d,]*\.\d+(DR)?)'
      description_transaction_line: '(^\s+\S.*)'
      amount_debit_suffix: '-'
      balance_debit_suffix: 'DR'
      date_format: '_2/01/06'
      account_number: '(\d{6}-\d{6}|\d{12})\n(?:.*\n)*?(?:ACCOUNT|NUMBER)'
      account_name: '(\d{2}/\d{2}/\d{2})\n(?:MR / ENCIK |ENCIK |MR |MS |CIK |MADAM )?([A-Z][A-Z\s'']+[A-Z])\nSTATEMENT DATE'
      account_type: '(?:DEPOSITOR\s+([A-Za-z][A-Za-z\-\s]+)|NUMBER\n([A-Za-z][A-Za-z\-\s]+?)\n)'
`

func setupTestConfig() {
	viper.Reset()
	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBufferString(testConfigYAML))
}

// Synthetic test data - mimics real statement structure with fake data
func getTestRowsCASA() *[]string {
	rows := []string{
		" ",
		"Maybank Islamic Berhad (787435-M)",
		"15th Floor, Tower A, Dataran Maybank, 1, Jalan Maarof, 59000 Kuala Lumpur",
		"MUKA/ /PAGE :",
		"000001 IBS TEST BRANCH 頁 1",
		"TARIKH PENYATA",
		":",
		"結單日期",
		"30/11/24",
		"JOHN DOE BIN SMITH",
		"STATEMENT DATE",
		"123 JALAN TEST 1/1 ,SECTION 1",
		"TEST CITY ,12345 ,SELANGOR ,MYS NOMBOR AKAUN",
		"戶號",
		":",
		"123456-789012",
		"ACCOUNT",
		"NUMBER",
		"PROTECTED BY PIDM UP TO RM250,000 FOR EACH DEPOSITOR PERSONAL SAVER-i",
		"戶口進支項",
		"URUSNIAGA AKAUN/ /ACCOUNT TRANSACTIONS",
		"TARIKH MASUK BUTIR URUSNIAGA JUMLAH URUSNIAGA BAKI PENYATA",
		"進支日期 進支項說明 银碼 結單存餘",
		"ENTRY DATE TRANSACTION DESCRIPTION TRANSACTION AMOUNT STATEMENT BALANCE",
		"BEGINNING BALANCE 100.00",
		"01/11/24 TRANSFER IN 50.00+ 150.00",
		"   FROM TEST ACCOUNT",
		"   REF123456",
		"02/11/24 PAYMENT OUT 25.50- 124.50",
		"   TO MERCHANT ABC",
		"   PURCHASE",
		"ENDING BALANCE : 124.50",
		"TOTAL CREDIT : 50.00",
		"TOTAL DEBIT : 25.50",
	}
	return &rows
}

func getTestRowsMAE() *[]string {
	rows := []string{
		" ",
		"Malayan Banking Berhad (3813-K)",
		"14th Floor, Menara Maybank, 100 Jalan Tun Perak, 50050 Kuala Lumpur, Malaysia",
		"TEST BRANCH MAIN",
		"MUKA/ /PAGE :",
		"頁 1",
		"TARIKH PENYATA",
		":",
		"結單日期",
		"15/12/24",
		"MR / ENCIK JANE DOE BINTI AHMAD",
		"STATEMENT DATE",
		"456 JALAN SAMPLE 2/2 ,SECTION 2",
		"SAMPLE CITY ,54321 ,SELANGOR ,MYS NOMBOR AKAUN",
		"戶號",
		":",
		"987654321012",
		"ACCOUNT",
		"NUMBER",
		"MAE",
		"戶口進支項",
		"BEGINNING BALANCE 200.50",
		"01/12/24 DUITNOW TRANSFER 100.00+ 300.50",
		"   FROM FRIEND",
		"02/12/24 PURCHASE 45.00- 255.50",
		"   SHOP XYZ",
		"ENDING BALANCE : 255.50",
		"TOTAL CREDIT : 100.00",
		"TOTAL DEBIT : 45.00",
	}
	return &rows
}

func TestExtract_AccountNumber_CASA(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test_123456-789012_20241130.pdf", rows)

	if statement.Account.AccountNumber != "123456-789012" {
		t.Errorf("Expected account number '123456-789012', got '%s'", statement.Account.AccountNumber)
	}
}

func TestExtract_AccountNumber_MAE(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsMAE()

	statement := Extract("test_987654321012_20241215.pdf", rows)

	if statement.Account.AccountNumber != "987654321012" {
		t.Errorf("Expected account number '987654321012', got '%s'", statement.Account.AccountNumber)
	}
}

func TestExtract_AccountName_CASA(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	if statement.Account.AccountName != "JOHN DOE BIN SMITH" {
		t.Errorf("Expected account name 'JOHN DOE BIN SMITH', got '%s'", statement.Account.AccountName)
	}
}

func TestExtract_AccountName_MAE(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsMAE()

	statement := Extract("test.pdf", rows)

	if statement.Account.AccountName != "JANE DOE BINTI AHMAD" {
		t.Errorf("Expected account name 'JANE DOE BINTI AHMAD', got '%s'", statement.Account.AccountName)
	}
}

func TestExtract_AccountType_CASA(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	if statement.Account.AccountType != "PERSONAL SAVER-i" {
		t.Errorf("Expected account type 'PERSONAL SAVER-i', got '%s'", statement.Account.AccountType)
	}
}

func TestExtract_AccountType_MAE(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsMAE()

	statement := Extract("test.pdf", rows)

	if statement.Account.AccountType != "MAE" {
		t.Errorf("Expected account type 'MAE', got '%s'", statement.Account.AccountType)
	}
}

func TestExtract_StartingBalance(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	expected := "100"
	if statement.StartingBalance.String() != expected {
		t.Errorf("Expected starting balance '%s', got '%s'", expected, statement.StartingBalance.String())
	}
}

func TestExtract_EndingBalance(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	expected := "124.5"
	if statement.EndingBalance.String() != expected {
		t.Errorf("Expected ending balance '%s', got '%s'", expected, statement.EndingBalance.String())
	}
}

func TestExtract_Transactions(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	if len(statement.Transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(statement.Transactions))
	}

	// Check first transaction (credit)
	if len(statement.Transactions) > 0 {
		tx := statement.Transactions[0]
		if tx.Type != "credit" {
			t.Errorf("Expected first transaction type 'credit', got '%s'", tx.Type)
		}
		if tx.Amount.String() != "50" {
			t.Errorf("Expected first transaction amount '50', got '%s'", tx.Amount.String())
		}
	}

	// Check second transaction (debit)
	if len(statement.Transactions) > 1 {
		tx := statement.Transactions[1]
		if tx.Type != "debit" {
			t.Errorf("Expected second transaction type 'debit', got '%s'", tx.Type)
		}
		if tx.Amount.String() != "-25.5" {
			t.Errorf("Expected second transaction amount '-25.5', got '%s'", tx.Amount.String())
		}
	}
}

func TestExtract_TransactionDescriptions(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	if len(statement.Transactions) < 1 {
		t.Fatal("Expected at least 1 transaction")
	}

	tx := statement.Transactions[0]
	if len(tx.Descriptions) < 2 {
		t.Errorf("Expected at least 2 description lines, got %d", len(tx.Descriptions))
	}

	if tx.Descriptions[0] != "TRANSFER IN" {
		t.Errorf("Expected first description 'TRANSFER IN', got '%s'", tx.Descriptions[0])
	}
}

func TestExtract_StatementDate(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	if statement.StatementDate == nil {
		t.Fatal("Expected statement date to be set")
	}

	expected := "2024-11-30"
	actual := statement.StatementDate.Format("2006-01-02")
	if actual != expected {
		t.Errorf("Expected statement date '%s', got '%s'", expected, actual)
	}
}

func TestExtract_CalculatedEndingBalance(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	// Starting balance 100 + 50 (credit) - 25.50 (debit) = 124.50
	expected := "124.5"
	if statement.CalculatedEndingBalance.String() != expected {
		t.Errorf("Expected calculated ending balance '%s', got '%s'", expected, statement.CalculatedEndingBalance.String())
	}
}

func TestExtract_TotalDebitCredit(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	if statement.TotalCredit.String() != "50" {
		t.Errorf("Expected total credit '50', got '%s'", statement.TotalCredit.String())
	}

	if statement.TotalDebit.String() != "-25.5" {
		t.Errorf("Expected total debit '-25.5', got '%s'", statement.TotalDebit.String())
	}
}

func TestExtract_Nett(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("test.pdf", rows)

	// Nett = TotalDebit + TotalCredit = -25.5 + 50 = 24.5
	expected := "24.5"
	if statement.Nett.String() != expected {
		t.Errorf("Expected nett '%s', got '%s'", expected, statement.Nett.String())
	}
}

func TestExtract_Source(t *testing.T) {
	setupTestConfig()
	rows := getTestRowsCASA()

	statement := Extract("path/to/test_statement.pdf", rows)

	if statement.Source != "test_statement" {
		t.Errorf("Expected source 'test_statement', got '%s'", statement.Source)
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
