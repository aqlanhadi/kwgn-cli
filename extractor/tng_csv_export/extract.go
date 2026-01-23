package tng_csv_export

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
)

// CSV column indices
const (
	colMFGNumber        = 0
	colTransNo          = 1
	colTransDateTime    = 2
	colPostedDate       = 3
	colTransType        = 4
	colSector           = 5
	colEntryLocation    = 6
	colEntrySP          = 7
	colExitLocation     = 8
	colExitSP           = 9
	colReloadLocation   = 10
	colTransAmount      = 11
	colBalance          = 12
	colVehicleClass     = 13
	colDeviceNo         = 14
	colTransactionID    = 15
	colVehicleNumber    = 16
)

// TNGCSVRow represents a single row from TNG CSV export
type TNGCSVRow struct {
	MFGNumber        string
	TransNo          string
	TransDateTime    time.Time
	PostedDate       time.Time
	TransType        string
	Sector           string
	EntryLocation    string
	EntrySP          string
	ExitLocation     string
	ExitSP           string
	ReloadLocation   string
	TransAmount      decimal.Decimal
	Balance          decimal.Decimal
	VehicleClass     string
	DeviceNo         string
	TransactionID    string
	VehicleNumber    string
}

// ExtractMulti parses a TNG CSV export file and returns multiple statements (one per MFG Number)
func ExtractMulti(reader io.Reader, filename string) ([]common.Statement, error) {
	csvReader := csv.NewReader(reader)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Validate header
	if len(header) < 17 {
		return nil, fmt.Errorf("invalid CSV format: expected at least 17 columns, got %d", len(header))
	}

	// Load Asia/Kuala_Lumpur timezone
	loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		log.Printf("Warning: Could not load KL timezone, using Local: %v", err)
		loc = time.Local
	}

	// Parse all rows and group by MFG Number
	rowsByMFG := make(map[string][]TNGCSVRow)

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Warning: error reading CSV row: %v", err)
			continue
		}

		if len(record) < 17 {
			log.Printf("Warning: skipping row with insufficient columns: %d", len(record))
			continue
		}

		row, err := parseRow(record, loc)
		if err != nil {
			log.Printf("Warning: error parsing row: %v", err)
			continue
		}

		rowsByMFG[row.MFGNumber] = append(rowsByMFG[row.MFGNumber], row)
	}

	if len(rowsByMFG) == 0 {
		return nil, fmt.Errorf("no valid transactions found in CSV")
	}

	// Create a statement for each MFG Number
	var statements []common.Statement
	for mfgNumber, rows := range rowsByMFG {
		stmt := createStatement(mfgNumber, rows, filename)
		statements = append(statements, stmt)
	}

	return statements, nil
}

// parseRow converts a CSV record to TNGCSVRow
func parseRow(record []string, loc *time.Location) (TNGCSVRow, error) {
	var row TNGCSVRow

	row.MFGNumber = strings.TrimSpace(record[colMFGNumber])
	row.TransNo = strings.TrimSpace(record[colTransNo])

	// Parse transaction datetime (format: 2025-09-02 16:48:09)
	transDateTime, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(record[colTransDateTime]), loc)
	if err != nil {
		return row, fmt.Errorf("invalid transaction datetime: %w", err)
	}
	row.TransDateTime = transDateTime

	// Parse posted date (format: 2025-09-03 00:00:00)
	postedDate, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(record[colPostedDate]), loc)
	if err != nil {
		// Not critical, use zero time
		row.PostedDate = time.Time{}
	} else {
		row.PostedDate = postedDate
	}

	row.TransType = strings.TrimSpace(record[colTransType])
	row.Sector = strings.TrimSpace(record[colSector])
	row.EntryLocation = strings.TrimSpace(record[colEntryLocation])
	row.EntrySP = strings.TrimSpace(record[colEntrySP])
	row.ExitLocation = strings.TrimSpace(record[colExitLocation])
	row.ExitSP = strings.TrimSpace(record[colExitSP])
	row.ReloadLocation = strings.TrimSpace(record[colReloadLocation])

	// Parse amount
	amountStr := strings.TrimSpace(record[colTransAmount])
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return row, fmt.Errorf("invalid amount: %w", err)
	}
	row.TransAmount = amount

	// Parse balance
	balanceStr := strings.TrimSpace(record[colBalance])
	balance, err := decimal.NewFromString(balanceStr)
	if err != nil {
		return row, fmt.Errorf("invalid balance: %w", err)
	}
	row.Balance = balance

	row.VehicleClass = strings.TrimSpace(record[colVehicleClass])
	row.DeviceNo = strings.TrimSpace(record[colDeviceNo])
	row.TransactionID = strings.TrimSpace(record[colTransactionID])
	row.VehicleNumber = strings.TrimSpace(record[colVehicleNumber])

	return row, nil
}

// createStatement creates a Statement from grouped TNG CSV rows
func createStatement(mfgNumber string, rows []TNGCSVRow, filename string) common.Statement {
	// Sort by transaction datetime (oldest first)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].TransDateTime.Before(rows[j].TransDateTime)
	})

	statement := common.Statement{
		Source: strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)),
		Account: common.Account{
			AccountNumber: mfgNumber,
			AccountName:   "TNG_CSV_EXPORT",
			AccountType:   "", // Leave empty - user can set via web UI
			DebitCredit:   "",
			Reconciliable: false, // TNG CSV data is never reconciliable
		},
		Transactions: []common.Transaction{},
	}

	var totalDebit, totalCredit decimal.Decimal
	var minDate, maxDate time.Time

	for i, row := range rows {
		// Track date range
		if i == 0 || row.TransDateTime.Before(minDate) {
			minDate = row.TransDateTime
		}
		if i == 0 || row.TransDateTime.After(maxDate) {
			maxDate = row.TransDateTime
		}

		// Determine transaction type
		txType := "debit" // Default: Usage transactions are debits
		if strings.ToLower(row.TransType) == "reload" {
			txType = "credit"
		}

		// Calculate totals
		if txType == "debit" {
			totalDebit = totalDebit.Add(row.TransAmount)
		} else {
			totalCredit = totalCredit.Add(row.TransAmount)
		}

		// Build description
		descriptions := []string{row.TransType}
		if row.Sector != "" {
			descriptions = append(descriptions, row.Sector)
		}
		if row.EntryLocation != "" {
			descriptions = append(descriptions, row.EntryLocation)
		}
		if row.ExitLocation != "" && row.ExitLocation != row.EntryLocation {
			descriptions = append(descriptions, "to "+row.ExitLocation)
		}

		// Create transaction with extra data stored in Data field
		tx := common.Transaction{
			Sequence:     i + 1,
			Date:         row.TransDateTime,
			Descriptions: descriptions,
			Type:         txType,
			Amount:       row.TransAmount,
			Balance:      row.Balance,
			Reference:    row.TransactionID, // Use Transaction ID for idempotency
			Tags:         []string{row.Sector, row.TransType},
			Data: map[string]interface{}{
				"mfg_number":      row.MFGNumber,
				"trans_no":        row.TransNo,
				"posted_date":     row.PostedDate.Format(time.RFC3339),
				"sector":          row.Sector,
				"entry_location":  row.EntryLocation,
				"entry_sp":        row.EntrySP,
				"exit_location":   row.ExitLocation,
				"exit_sp":         row.ExitSP,
				"reload_location": row.ReloadLocation,
				"vehicle_class":   row.VehicleClass,
				"device_no":       row.DeviceNo,
				"vehicle_number":  row.VehicleNumber,
			},
		}

		statement.Transactions = append(statement.Transactions, tx)
	}

	// Set date range
	statement.TransactionStartDate = minDate
	statement.TransactionEndDate = maxDate

	// Calculate starting balance from first (oldest) transaction
	// Starting balance = first transaction's balance +/- amount depending on type
	if len(rows) > 0 {
		firstRow := rows[0]
		if strings.ToLower(firstRow.TransType) == "reload" {
			// For reload: balance_after = balance_before + amount
			// So: balance_before = balance_after - amount
			statement.StartingBalance = firstRow.Balance.Sub(firstRow.TransAmount)
		} else {
			// For usage: balance_after = balance_before - amount
			// So: balance_before = balance_after + amount
			statement.StartingBalance = firstRow.Balance.Add(firstRow.TransAmount)
		}

		// Ending balance is the last (newest) transaction's balance
		lastRow := rows[len(rows)-1]
		statement.EndingBalance = lastRow.Balance
	}

	statement.TotalDebit = totalDebit
	statement.TotalCredit = totalCredit
	statement.Nett = totalCredit.Sub(totalDebit)

	// Calculate expected ending balance
	// StartingBalance + Credits - Debits = CalculatedEndingBalance
	statement.CalculatedEndingBalance = statement.StartingBalance.Add(totalCredit).Sub(totalDebit)

	// Set statement date to the max transaction date
	if !maxDate.IsZero() {
		statement.StatementDate = &maxDate
	}

	return statement
}

// ValidateBalance checks if the calculated ending balance matches the actual ending balance
func ValidateBalance(stmt common.Statement) (bool, string) {
	if stmt.EndingBalance.Equal(stmt.CalculatedEndingBalance) {
		return true, fmt.Sprintf("Balance validated: Starting=%.2f, Ending=%.2f, Calculated=%.2f",
			stmt.StartingBalance.InexactFloat64(),
			stmt.EndingBalance.InexactFloat64(),
			stmt.CalculatedEndingBalance.InexactFloat64())
	}

	diff := stmt.EndingBalance.Sub(stmt.CalculatedEndingBalance)
	return false, fmt.Sprintf("Balance mismatch: Starting=%.2f, Ending=%.2f, Calculated=%.2f, Diff=%.2f",
		stmt.StartingBalance.InexactFloat64(),
		stmt.EndingBalance.InexactFloat64(),
		stmt.CalculatedEndingBalance.InexactFloat64(),
		diff.InexactFloat64())
}
