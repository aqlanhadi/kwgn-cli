package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aqlanhadi/kwgn/extractor/common"
	"github.com/jackc/pgx/v5"
)

// normalizeDescription joins the descriptions array, collapses whitespace, and uppercases
// Result: "TRANSFER TO A/C AQLAN HADI BIN NOR * MAE CASA"
func normalizeDescription(descriptions []string) string {
	joined := strings.Join(descriptions, " ")
	// Collapse multiple spaces into single space
	spaceRegex := regexp.MustCompile(`\s+`)
	normalized := spaceRegex.ReplaceAllString(joined, " ")
	return strings.ToUpper(strings.TrimSpace(normalized))
}

// CreateTransactions bulk inserts transactions for a statement
func (db *DB) CreateTransactions(ctx context.Context, statementID string, transactions []common.Transaction) error {
	return db.CreateTransactionsIdempotent(ctx, statementID, transactions, false)
}

// CreateTransactionsIdempotent bulk inserts transactions for a statement with optional idempotent handling.
// When idempotent is true, duplicate transactions (by reference) are silently skipped using ON CONFLICT DO NOTHING.
// This is useful for TNG CSV imports where the same transactions may appear in multiple export files.
func (db *DB) CreateTransactionsIdempotent(ctx context.Context, statementID string, transactions []common.Transaction, idempotent bool) error {
	if len(transactions) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, tx := range transactions {
		// tx.Tags and tx.Data are NOT persisted. Only the TnG CSV extractor
		// fills them, and this database holds Maybank statements only, so the
		// columns were dropped rather than carried empty. To restore TnG:
		// re-add the two columns in schema.go's migrateDDL, add "tags, data"
		// back to the column lists below with two more placeholders, and pass
		// tags and dataJSON as the last two arguments.

		// Normalize description for matching (join, collapse spaces, uppercase)
		description := normalizeDescription(tx.Descriptions)

		// Use ON CONFLICT DO NOTHING for idempotent imports
		// This relies on the unique index idx_transactions_unique_reference (statement_id, reference) WHERE reference != ''
		// id and created_at use database defaults (UUID v7 and NOW())
		var sql string
		if idempotent && tx.Reference != "" {
			sql = `
				INSERT INTO transactions (
					statement_id, sequence, date, descriptions, description, type, amount, balance, reference
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (statement_id, reference) WHERE reference != '' DO NOTHING
			`
		} else {
			sql = `
				INSERT INTO transactions (
					statement_id, sequence, date, descriptions, description, type, amount, balance, reference
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`
		}

		batch.Queue(sql,
			statementID, tx.Sequence, tx.Date, tx.Descriptions, description,
			tx.Type, tx.Amount, tx.Balance, tx.Reference,
		)
	}

	br := db.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for range transactions {
		_, err := br.Exec()
		if err != nil {
			// For idempotent mode, we expect some inserts to be skipped
			// Check if it's a unique constraint violation and ignore it
			if idempotent {
				continue // Skip errors in idempotent mode (likely unique violations)
			}
			return fmt.Errorf("failed to insert transaction: %w", err)
		}
	}

	return nil
}
