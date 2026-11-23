package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/jackc/pgx/v5"
)

// StatementExists checks if a statement already exists using natural key
func (db *DB) StatementExists(ctx context.Context, accountID string, statementDate time.Time) (bool, string, error) {
	var id string
	err := db.Pool.QueryRow(ctx, `
		SELECT id FROM statements
		WHERE account_id = $1 AND statement_date = $2
	`, accountID, statementDate).Scan(&id)

	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to check statement: %w", err)
	}

	return true, id, nil
}

// CreateStatement inserts a new statement
func (db *DB) CreateStatement(ctx context.Context, accountID string, stmt common.Statement) (string, error) {
	var id string

	var stmtDate *time.Time
	if stmt.StatementDate != nil {
		stmtDate = stmt.StatementDate
	}

	var txStartDate, txEndDate *time.Time
	if !stmt.TransactionStartDate.IsZero() {
		txStartDate = &stmt.TransactionStartDate
	}
	if !stmt.TransactionEndDate.IsZero() {
		txEndDate = &stmt.TransactionEndDate
	}

	// Opening and closing balances are stored only when the issuer printed
	// them. A not_printed statement stores NULL: a balance worked out from the
	// rows would make accounting_check §1 pass by construction.
	var startBal, endBal any
	switch stmt.BalanceBasis {
	case "printed":
		startBal, endBal = stmt.StartingBalance, stmt.EndingBalance
	case "not_printed":
		startBal, endBal = nil, nil
	default:
		return "", fmt.Errorf("statement %s: balance basis %q, want printed or not_printed (the extractor must say which)", stmt.Source, stmt.BalanceBasis)
	}

	err := db.Pool.QueryRow(ctx, `
		INSERT INTO statements (
			account_id, source, statement_date,
			starting_balance, ending_balance, calculated_ending_balance,
			total_credit, total_debit, nett,
			transaction_start_date, transaction_end_date, balance_basis
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`,
		accountID, stmt.Source, stmtDate,
		startBal, endBal, stmt.CalculatedEndingBalance,
		stmt.TotalCredit, stmt.TotalDebit, stmt.Nett,
		txStartDate, txEndDate, stmt.BalanceBasis,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("failed to create statement: %w", err)
	}

	return id, nil
}

// DeleteStatement removes a statement and its transactions (cascade)
func (db *DB) DeleteStatement(ctx context.Context, statementID string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM statements WHERE id = $1`, statementID)
	if err != nil {
		return fmt.Errorf("failed to delete statement: %w", err)
	}
	return nil
}
