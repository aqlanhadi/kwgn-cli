package mbb_2_cc

import (
	"log"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/spf13/viper"
)

type config struct {
	StartingBalance *regexp.Regexp
	EndingBalance   *regexp.Regexp
	StatementDate   *regexp.Regexp
	Transaction     *regexp.Regexp
	CreditSuffix    string
	DateFormat      string
	StatementFormat string
}

func loadConfig() config {
	return config{
		StartingBalance: regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.starting_balance")),
		EndingBalance:   regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.ending_balance")),
		StatementDate:   regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.statement_date")),
		Transaction:     regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.transaction")),
		CreditSuffix:    viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix"),
		DateFormat:      viper.GetString("statement.MAYBANK_2_CC.patterns.date_format"),
		StatementFormat: viper.GetString("statement.MAYBANK_2_CC.patterns.statement_format"),
	}
}

func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()
	startTime := time.Now()
	log.Printf("Starting extraction for MAYBANK_2_CC statement: %s", path)

	statement := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Transactions: []common.Transaction{},
	}

	// Extract metadata and balance
	for _, text := range *rows {
		// Starting Balance
		if cfg.StartingBalance.MatchString(text) {
			amount, _ := common.CleanDecimal(text)
			if strings.HasSuffix(strings.TrimSpace(text), cfg.CreditSuffix) {
				amount = amount.Neg()
			}
			statement.StartingBalance = statement.StartingBalance.Add(amount)
		}

		// Ending Balance
		if cfg.EndingBalance.MatchString(text) {
			amount, _ := common.CleanDecimal(text)
			if strings.HasSuffix(strings.TrimSpace(text), cfg.CreditSuffix) {
				amount = amount.Neg()
			}
			statement.EndingBalance = statement.EndingBalance.Add(amount)
		}

		// Statement Date
		if statement.StatementDate == nil {
			if match := cfg.StatementDate.FindString(text); match != "" {
				if dt, err := time.ParseInLocation(cfg.StatementFormat, match, time.Local); err == nil {
					statement.StatementDate = &dt
				}
			}
		}
	}

	log.Printf("Extracted metadata: Date=%v, Start=%s, End=%s", statement.StatementDate, statement.StartingBalance, statement.EndingBalance)

	// Extract Transactions
	balance := statement.StartingBalance
	sequence := 0

	for _, text := range *rows {
		match := cfg.Transaction.FindStringSubmatch(text)
		if len(match) == 0 {
			continue
		}

		sequence++
		date, _ := time.ParseInLocation(cfg.DateFormat, match[1], time.Local)

		if statement.StatementDate != nil {
			date = common.FixDateYear(date, *statement.StatementDate)
		}

		amount, _ := common.CleanDecimal(match[4])

		txType := "debit"
		if strings.HasSuffix(strings.TrimSpace(match[4]), cfg.CreditSuffix) {
			amount = amount.Neg()
			statement.TotalCredit = statement.TotalCredit.Add(amount)
			txType = "credit"
		} else {
			statement.TotalDebit = statement.TotalDebit.Add(amount)
		}

		balance = balance.Add(amount)

		statement.Transactions = append(statement.Transactions, common.Transaction{
			Sequence:     sequence,
			Date:         date,
			Descriptions: []string{match[3]},
			Type:         txType,
			Amount:       amount,
			Balance:      balance,
		})
	}

	// Post-processing
	statement.Nett = statement.TotalDebit.Add(statement.TotalCredit)
	statement.CalculatedEndingBalance = balance

	if len(statement.Transactions) > 0 {
		slices.SortFunc(statement.Transactions, func(a, b common.Transaction) int { return a.Date.Compare(b.Date) })

		// Recalculate balances after sort
		runningBalance := statement.StartingBalance
		for i := range statement.Transactions {
			statement.Transactions[i].Sequence = i + 1
			runningBalance = runningBalance.Add(statement.Transactions[i].Amount)
			statement.Transactions[i].Balance = runningBalance
		}
		statement.CalculatedEndingBalance = runningBalance

		statement.TransactionStartDate = statement.Transactions[0].Date
		statement.TransactionEndDate = statement.Transactions[len(statement.Transactions)-1].Date
	}

	if statement.CalculatedEndingBalance.Equal(statement.EndingBalance) {
		log.Printf("✓ Ending balance matches")
	} else {
		log.Printf("✗ Ending balance mismatch: Calc=%s vs Stmt=%s", statement.CalculatedEndingBalance, statement.EndingBalance)
	}

	log.Printf("Total MAYBANK_2_CC processing time: %v", time.Since(startTime))
	return statement
}
