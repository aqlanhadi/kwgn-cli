package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

// A folder must yield every statement a file holds, as a single-file extract
// and kwgn-ledger (through ExtractFile) do. Before ProcessDir the folder loop kept one statement
// per file, so a Maybank 2 PDF's Amex rows landed on the Mastercard and a
// multi-card TnG CSV was dropped.
func TestProcessDir_FileWithSeveralStatements(t *testing.T) {
	dir := t.TempDir()
	csv := `MFG Number,Trans. No.,Transaction Date/Time,Posted Date,Trans. Type,Sector,Entry Location,Entry SP,Exit Location,Exit SP,Reload Location,Trans. Amount (RM),Balance (RM),Vehicle Class,Device No.,Transaction ID,Vehicle Number
1111111111,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,TOLL,TOLL A,SP_A,TOLL A,SP_A,,5.00,95.00,00,,TX001,
2222222222,1,2025-01-01 10:00:00,2025-01-02 00:00:00,Usage,PARKING,PARK A,SP_B,PARK A,SP_B,,3.00,47.00,00,,TX002,
1111111111,2,2025-01-02 10:00:00,2025-01-03 00:00:00,Reload,INTERNET,RELOAD,SP_C,RELOAD,SP_C,OTA,50.00,145.00,00,,TX003,`
	if err := os.WriteFile(filepath.Join(dir, "cards.csv"), []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := ProcessDir(dir, "")
	if err != nil {
		t.Fatalf("ProcessDir: %v", err)
	}

	f, err := os.Open(filepath.Join(dir, "cards.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want := ProcessReaderMulti(f, filepath.Join(dir, "cards.csv"), "")

	if len(want) != 2 {
		t.Fatalf("fixture should hold 2 statements, single-file path gave %d", len(want))
	}
	if len(got) != len(want) {
		t.Fatalf("folder gave %d statements, single file gives %d", len(got), len(want))
	}
	txns := map[string]int{}
	for _, s := range got {
		txns[s.Account.AccountNumber] = len(s.Transactions)
	}
	for _, s := range want {
		if txns[s.Account.AccountNumber] != len(s.Transactions) {
			t.Errorf("account %s: folder has %d transactions, single file %d",
				s.Account.AccountNumber, txns[s.Account.AccountNumber], len(s.Transactions))
		}
	}
}
