package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aqlanhadi/kwgn/extractor/common"
)

// StatementExists checks if a statement already exists using natural key
func (db *DB) StatementExists(ctx context.Context, accountID string, statementDate time.Time) (bool, string, error) {
	var id string
	err := db.Pool.QueryRow(ctx, `
		SELECT id FROM statements 
		WHERE account_id = $1 AND statement_date = $2
	`, accountID, statementDate).Scan(&id)

	if err != nil {
		if err.Error() == "no rows in result set" {
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

	err := db.Pool.QueryRow(ctx, `
		INSERT INTO statements (
			account_id, source, statement_date,
			starting_balance, ending_balance, calculated_ending_balance,
			total_credit, total_debit, nett,
			transaction_start_date, transaction_end_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`,
		accountID, stmt.Source, stmtDate,
		stmt.StartingBalance, stmt.EndingBalance, stmt.CalculatedEndingBalance,
		stmt.TotalCredit, stmt.TotalDebit, stmt.Nett,
		txStartDate, txEndDate,
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
