package postgres

import (
	"context"
	"fmt"

	"github.com/aqlanhadi/kwgn/extractor/common"
)

// GetOrCreateAccount finds an existing account by number or creates a new one.
// Uses INSERT ON CONFLICT to safely handle concurrent access and prevent duplicates.
func (db *DB) GetOrCreateAccount(ctx context.Context, account common.Account) (string, error) {
	var id string

	// Upsert: insert if not exists, update if exists, always return the id.
	// This is atomic and safe against duplicates even without application-level locking.
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO accounts (account_number, account_name, account_type, reconciliable)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_number) DO UPDATE SET
			account_name = EXCLUDED.account_name,
			account_type = CASE WHEN EXCLUDED.account_type != '' THEN EXCLUDED.account_type ELSE accounts.account_type END,
			reconciliable = EXCLUDED.reconciliable,
			updated_at = NOW()
		RETURNING id
	`, account.AccountNumber, account.AccountName, account.AccountType, account.Reconciliable).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("failed to upsert account: %w", err)
	}

	return id, nil
}
