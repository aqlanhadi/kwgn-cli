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
	AccountNumber     *regexp.Regexp
	AccountName       *regexp.Regexp
	AccountType       *regexp.Regexp
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
		AccountNumber:     regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.account_number")),
		AccountName:       regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.account_name")),
		AccountType:       regexp.MustCompile(viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.account_type")),
		CreditSuffix:      viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.credit_suffix"),
		DebitAmountSuffix: viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.amount_debit_suffix"),
		DateFormat:        viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.date_format"),
		StatementFormat:   viper.GetString("statement.MAYBANK_CASA_AND_MAE.patterns.statement_format"),
	}
}

func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()

	statement := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Transactions: []common.Transaction{},
	}

	// Join all rows for multiline pattern matching (account info spans multiple lines)
	fullText := strings.Join(*rows, "\n")

	// Extract Account Information from full text
	if statement.Account.AccountNumber == "" {
		if match := cfg.AccountNumber.FindStringSubmatch(fullText); len(match) > 1 {
			statement.Account.AccountNumber = strings.TrimSpace(match[1])
		}
	}
	// AccountName is now what was previously AccountType (e.g., "MAE", "SAVINGS ACCOUNT-I")
	if statement.Account.AccountName == "" {
		if match := cfg.AccountType.FindStringSubmatch(fullText); len(match) > 1 {
			// Check each capture group for non-empty match
			for i := 1; i < len(match); i++ {
				if match[i] != "" {
					statement.Account.AccountName = strings.TrimSpace(match[i])
					break
				}
			}
		}
	}
	statement.Account.AccountType = "" // Leave empty - user can set via web UI

	// Extract Metadata (balances and dates)
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

	for _, line := range *rows {
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
				amount = amount.Abs().Neg()
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
			currentTransaction.Descriptions = append(currentTransaction.Descriptions, strings.TrimSpace(line))
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

	if !statement.CalculatedEndingBalance.Equal(statement.EndingBalance) {
		log.Printf("WARN ending balance mismatch: calculated=%s stated=%s", statement.CalculatedEndingBalance, statement.EndingBalance)
	}

	return statement
}
