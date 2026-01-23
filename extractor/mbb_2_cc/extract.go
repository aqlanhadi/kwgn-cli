package mbb_2_cc

import (
	"log"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

type config struct {
	StartingBalance *regexp.Regexp
	EndingBalance   *regexp.Regexp
	StatementDate   *regexp.Regexp
	Transaction     *regexp.Regexp
	AccountNumber   *regexp.Regexp
	AccountName     *regexp.Regexp
	AccountType     *regexp.Regexp
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
		AccountNumber:   regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.account_number")),
		AccountName:     regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.account_name")),
		AccountType:     regexp.MustCompile(viper.GetString("statement.MAYBANK_2_CC.patterns.account_type")),
		CreditSuffix:    viper.GetString("statement.MAYBANK_2_CC.patterns.credit_suffix"),
		DateFormat:      viper.GetString("statement.MAYBANK_2_CC.patterns.date_format"),
		StatementFormat: viper.GetString("statement.MAYBANK_2_CC.patterns.statement_format"),
	}
}

func Extract(path string, rows *[]string) common.Statement {
	cfg := loadConfig()

	statement := common.Statement{
		Source:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Transactions: []common.Transaction{},
	}

	// Join all text for account extraction
	fullText := strings.Join(*rows, "\n")

	// Extract account info - get first match only
	if match := cfg.AccountNumber.FindStringSubmatch(fullText); len(match) > 2 {
		// match[1] = card type (MASTERCARD/AMEX), match[2] = card number
		statement.Account.AccountNumber = strings.ReplaceAll(match[2], " ", "")
	}
	// AccountName is now what was previously AccountType (e.g., "MAYBANK 2 PLAT MASTERCARD")
	if match := cfg.AccountType.FindStringSubmatch(fullText); len(match) > 1 {
		statement.Account.AccountName = strings.TrimSpace(match[1])
	}
	statement.Account.AccountType = "" // Leave empty - user can set via web UI
	statement.Account.DebitCredit = "credit"

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

	if !statement.CalculatedEndingBalance.Equal(statement.EndingBalance) {
		log.Printf("WARN ending balance mismatch: calculated=%s stated=%s", statement.CalculatedEndingBalance, statement.EndingBalance)
	}

	return statement
}

// cardSection represents a section of the statement for one card
type cardSection struct {
	cardNumber string
	cardType   string
	lines      []string
}

// ExtractMulti extracts multiple statements from a CC PDF (one per card)
func ExtractMulti(path string, rows *[]string) []common.Statement {
	cfg := loadConfig()
	fullText := strings.Join(*rows, "\n")

	// Extract common info (name, statement date) from full text
	var accountName string
	var statementDate *time.Time

	if match := cfg.AccountName.FindStringSubmatch(fullText); len(match) > 1 {
		accountName = strings.TrimSpace(match[1])
	}
	if match := cfg.StatementDate.FindString(fullText); match != "" {
		if dt, err := time.ParseInLocation(cfg.StatementFormat, match, time.Local); err == nil {
			statementDate = &dt
		}
	}

	// Find all card sections using the card header pattern
	// Pattern: "MAYBANK 2 PLAT MASTERCARD    :    5239 0000 0000 0002"
	cardHeaderPattern := regexp.MustCompile(`(MAYBANK 2 (?:PLAT(?:INUM)?|GOLD|CLASSIC)\s+(?:MASTERCARD|AMEX))\s+:\s+(\d{4}\s\d{4}\s\d{4}\s\d{4}|\d{4}\s\d{6}\s\d{5})`)
	subTotalPattern := regexp.MustCompile(`SUB TOTAL/JUMLAH`)

	// Find all card headers
	matches := cardHeaderPattern.FindAllStringSubmatchIndex(fullText, -1)
	if len(matches) == 0 {
		// Fallback to single extraction if no card headers found
		result := Extract(path, rows)
		if result.Account.AccountNumber != "" {
			return []common.Statement{result}
		}
		return []common.Statement{}
	}

	// Split text into card sections
	var sections []cardSection
	for i, match := range matches {
		cardType := fullText[match[2]:match[3]]
		cardNumber := strings.ReplaceAll(fullText[match[4]:match[5]], " ", "")

		// Find the end of this section (next card header or end of text)
		sectionStart := match[1] // End of this header
		sectionEnd := len(fullText)
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0] // Start of next header
		}

		sectionText := fullText[sectionStart:sectionEnd]

		// Find SUB TOTAL to trim the section properly
		if subMatch := subTotalPattern.FindStringIndex(sectionText); subMatch != nil {
			// Include a bit after SUB TOTAL to capture the amount
			endIdx := subMatch[1] + 50
			if endIdx > len(sectionText) {
				endIdx = len(sectionText)
			}
			sectionText = sectionText[:endIdx]
		}

		sections = append(sections, cardSection{
			cardNumber: cardNumber,
			cardType:   strings.TrimSpace(cardType),
			lines:      strings.Split(sectionText, "\n"),
		})
	}

	// Process each section into a statement
	var statements []common.Statement
	baseSource := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	for _, section := range sections {
		stmt := processCardSection(cfg, section, baseSource, accountName, statementDate)
		if stmt.Account.AccountNumber != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// processCardSection processes a single card section into a statement
func processCardSection(cfg config, section cardSection, baseSource string, accountName string, statementDate *time.Time) common.Statement {
	statement := common.Statement{
		Source:        baseSource,
		StatementDate: statementDate,
		Transactions:  []common.Transaction{},
		Account: common.Account{
			AccountNumber: section.cardNumber,
			AccountName:   section.cardType, // Card type (e.g., "MAYBANK 2 PLAT MASTERCARD")
			AccountType:   "",               // Leave empty - user can set via web UI
			DebitCredit:   "credit",
		},
	}

	// Extract starting balance (YOUR PREVIOUS STATEMENT BALANCE)
	for _, text := range section.lines {
		if cfg.StartingBalance.MatchString(text) {
			amount, _ := common.CleanDecimal(text)
			if strings.HasSuffix(strings.TrimSpace(text), cfg.CreditSuffix) {
				amount = amount.Neg()
			}
			statement.StartingBalance = amount
			break
		}
	}

	// Extract ending balance (SUB TOTAL/JUMLAH)
	for _, text := range section.lines {
		if cfg.EndingBalance.MatchString(text) {
			amount, _ := common.CleanDecimal(text)
			if strings.HasSuffix(strings.TrimSpace(text), cfg.CreditSuffix) {
				amount = amount.Neg()
			}
			statement.EndingBalance = amount
			break
		}
	}

	// Extract transactions
	balance := statement.StartingBalance
	sequence := 0

	for _, text := range section.lines {
		match := cfg.Transaction.FindStringSubmatch(text)
		if len(match) == 0 {
			continue
		}

		sequence++
		date, _ := time.ParseInLocation(cfg.DateFormat, match[1], time.Local)

		if statementDate != nil {
			date = common.FixDateYear(date, *statementDate)
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

	// Only log mismatch if we have transactions (otherwise balances might both be 0)
	if len(statement.Transactions) > 0 && !statement.CalculatedEndingBalance.Equal(statement.EndingBalance) {
		log.Printf("WARN [%s] ending balance mismatch: calculated=%s stated=%s", section.cardNumber, statement.CalculatedEndingBalance, statement.EndingBalance)
	}

	return statement
}

// HasMultipleCards checks if the PDF contains multiple credit cards
func HasMultipleCards(rows *[]string) bool {
	fullText := strings.Join(*rows, "\n")
	cardHeaderPattern := regexp.MustCompile(`(MASTERCARD|AMEX)\s+:\s+(\d{4}\s\d{4}\s\d{4}\s\d{4}|\d{4}\s\d{6}\s\d{5})`)
	matches := cardHeaderPattern.FindAllStringSubmatch(fullText, -1)
	
	// Check for unique card numbers
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 2 {
			cardNum := strings.ReplaceAll(match[2], " ", "")
			seen[cardNum] = true
		}
	}
	return len(seen) > 1
}

// countUniqueCards returns the number of unique credit cards in the statement
func countUniqueCards(rows *[]string) int {
	fullText := strings.Join(*rows, "\n")
	cardHeaderPattern := regexp.MustCompile(`(MASTERCARD|AMEX)\s+:\s+(\d{4}\s\d{4}\s\d{4}\s\d{4}|\d{4}\s\d{6}\s\d{5})`)
	matches := cardHeaderPattern.FindAllStringSubmatch(fullText, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 2 {
			cardNum := strings.ReplaceAll(match[2], " ", "")
			seen[cardNum] = true
		}
	}
	return len(seen)
}

// Ensure decimal import is used
var _ = decimal.Zero
