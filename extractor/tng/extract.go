package tng

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

type config struct {
	Transaction      *regexp.Regexp
	AmountNumbers    *regexp.Regexp
	TransactionDate  string
	DebitSuffix      string
}

func loadConfig() config {
	return config{
		Transaction:     regexp.MustCompile(viper.GetString("statement.TNG.patterns.transaction")),
		AmountNumbers:   regexp.MustCompile(viper.GetString("statement.TNG.patterns.amount_numbers_pattern")),
		TransactionDate: viper.GetString("statement.TNG.patterns.transaction_date"),
		DebitSuffix:     viper.GetString("statement.TNG.patterns.debit_suffix"),
	}
}

func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()
	
	// In the original code, it re-read the file here. Now we use the passed rows.
	// Note: TNG PDF extraction might be tricky if rows are not cleanly separated,
	// but we assume common.ExtractRowsFromPDFReader does a good enough job.
	
	// However, TNG extraction in original code used `common.ExtractRowsFromPDF(path)` which calls `pdf.NewReader`.
	// The rows passed to this function are already extracted.
	
	transactions := []common.Transaction{}
	var totalDebit, totalCredit decimal.Decimal
	sequence := 0
	var minDate, maxDate time.Time
	firstDateSet := false
	
	loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		log.Printf("Warning: Could not load KL location, using Local: %v", err)
		loc = time.Local
	}

	for _, row := range *rows {
		matches := cfg.Transaction.FindAllStringSubmatch(row, -1)

		for _, s := range matches {
			// s indices: 0=full, 1=desc, 2=date, 3=time, 4=loc?, 5=?, 6=?, 7=amount_part
			// Original code:
			// description: s[1]
			// date: s[2], time: s[3]
			// amount is in s[7] (last part usually), parsed with AmountNumbers regex
			
			description := strings.TrimSpace(s[1])
			description = strings.ReplaceAll(description, "  ", " ")

			var descriptions []string
			if strings.HasPrefix(description, "Exit Toll: ") {
				rest := strings.TrimPrefix(description, "Exit Toll: ")
				descriptions = []string{"Exit Toll", rest, s[4]}
			} else {
				descriptions = []string{description, s[4]}
			}

			dateTime, err := time.ParseInLocation(cfg.TransactionDate, s[2]+" "+s[3], loc)
			if err != nil {
				log.Printf("Error parsing date '%s %s': %v", s[2], s[3], err)
				continue
			}

			if !firstDateSet {
				minDate = dateTime
				maxDate = dateTime
				firstDateSet = true
			} else {
				if dateTime.Before(minDate) {
					minDate = dateTime
				}
				if dateTime.After(maxDate) {
					maxDate = dateTime
				}
			}

			amountMatch := cfg.AmountNumbers.FindStringSubmatch(s[7])
			if len(amountMatch) < 3 {
				continue
			}
			
			sign := amountMatch[1]
			val := amountMatch[2]
			amount, _ := decimal.NewFromString(sign + val)
			
			txType := "credit"
			if sign == cfg.DebitSuffix {
				txType = "debit"
				totalDebit = totalDebit.Add(amount)
			} else {
				totalCredit = totalCredit.Add(amount)
			}

			sequence++
			transactions = append(transactions, common.Transaction{
				Sequence:     sequence,
				Date:         dateTime,
				Descriptions: descriptions,
				Type:         txType,
				Amount:       amount,
				Reference:    s[5] + s[6],
			})
		}
	}

	return common.Statement{
		Source:               strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		TransactionStartDate: minDate,
		TransactionEndDate:   maxDate,
		Transactions:         transactions,
		TotalDebit:           totalDebit,
		TotalCredit:          totalCredit,
		Nett:                 totalDebit.Add(totalCredit),
	}
}

