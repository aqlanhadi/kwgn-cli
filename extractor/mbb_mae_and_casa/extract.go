package mbb_mae_and_casa

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/spf13/viper"
)

type config struct {
	StartingBalance   *regexp.Regexp
	EndingBalance     *regexp.Regexp
	StatementDate     *regexp.Regexp
	MainTxLine        *regexp.Regexp
	DescTxLine        *regexp.Regexp
	CreditSuffix      string
	DebitAmountSuffix string
	DateFormat        string
	StatementFormat   string
}

func loadConfig() config {
	return config{
		StartingBalance:   regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.starting_balance")),
		EndingBalance:     regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.ending_balance")),
		StatementDate:     regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.statement_date")),
		MainTxLine:        regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.main_transaction_line")),
		DescTxLine:        regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.description_transaction_line")),
		CreditSuffix:      viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.credit_suffix"),
		DebitAmountSuffix: viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.amount_debit_suffix"),
		DateFormat:        viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.date_format"),
		StatementFormat:   viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.statement_format"),
	}
}

func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()
	startTime := time.Now()
	log.Printf("Starting extraction for MAYBANK_CASA_AND_MAE statement: %s", path)

	statement := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Transactions: []common.Transaction{},
	}

	// Extract Metadata
	for _, text := range *rows {
		if cfg.StartingBalance.MatchString(text) {
			amount, _ := common.CleanDecimal(text)
			if strings.HasSuffix(strings.TrimSpace(text), cfg.CreditSuffix) {
				amount = amount.Neg()
			}
			statement.StartingBalance = statement.StartingBalance.Add(amount)
		}
		if cfg.EndingBalance.MatchString(text) {
			amount, _ := common.CleanDecimal(text)
			if strings.HasSuffix(strings.TrimSpace(text), cfg.CreditSuffix) {
				amount = amount.Neg()
			}
			statement.EndingBalance = statement.EndingBalance.Add(amount)
		}
		if statement.StatementDate == nil {
			if match := cfg.StatementDate.FindString(text); match != "" {
				if dt, err := time.ParseInLocation(cfg.StatementFormat, match, time.Local); err == nil {
					statement.StatementDate = &dt
				}
			}
		}
	}

	// Extract Transactions
	var currentTransaction *common.Transaction
	balance := statement.StartingBalance
	sequence := 0

	for _, text := range *rows {
		line := strings.TrimSpace(text)
		
		// Check Main Transaction Line
		if match := cfg.MainTxLine.FindStringSubmatch(line); match != nil {
			if currentTransaction != nil {
				statement.Transactions = append(statement.Transactions, *currentTransaction)
			}

			sequence++
			
			// Date Parsing
			dateStr := match[1]
			parseLayout := cfg.DateFormat
			if len(strings.Split(dateStr, "/")) == 2 {
				parts := strings.Split(parseLayout, "/")
				if len(parts) >= 2 {
					parseLayout = strings.Join(parts[:2], "/")
				}
			}
			
			date, _ := time.ParseInLocation(parseLayout, dateStr, time.Local)
			if statement.StatementDate != nil {
				date = common.FixDateYear(date, *statement.StatementDate)
			}

			// Amount Parsing
			amount, _ := common.CleanDecimal(match[3])
			txType := "credit"
			if strings.HasSuffix(strings.TrimSpace(match[3]), cfg.DebitAmountSuffix) {
				amount = amount.Neg()
				statement.TotalDebit = statement.TotalDebit.Add(amount)
				txType = "debit"
			} else {
				statement.TotalCredit = statement.TotalCredit.Add(amount)
			}

			balance = balance.Add(amount)

			currentTransaction = &common.Transaction{
				Sequence:     sequence,
				Date:         date,
				Descriptions: []string{strings.TrimSpace(match[2])},
				Type:         txType,
				Amount:       amount,
				Balance:      balance,
			}
			continue
		}

		// Check Description Line
		if currentTransaction != nil && cfg.DescTxLine.MatchString(line) {
			currentTransaction.Descriptions = append(currentTransaction.Descriptions, line)
		}
	}

	if currentTransaction != nil {
		statement.Transactions = append(statement.Transactions, *currentTransaction)
	}

	statement.Nett = statement.TotalDebit.Add(statement.TotalCredit)
	statement.CalculatedEndingBalance = balance

	if len(statement.Transactions) > 0 {
		statement.TransactionStartDate = statement.Transactions[0].Date
		statement.TransactionEndDate = statement.Transactions[len(statement.Transactions)-1].Date
	}

	if statement.CalculatedEndingBalance.Equal(statement.EndingBalance) {
		log.Printf("✓ Ending balance matches")
	} else {
		log.Printf("✗ Ending balance mismatch: Calc=%s vs Stmt=%s", statement.CalculatedEndingBalance, statement.EndingBalance)
	}

	log.Printf("Total MAYBANK_CASA_AND_MAE processing time: %v", time.Since(startTime))
	return statement
}

