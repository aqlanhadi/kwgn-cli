package postgres

import (
	"context"
	"fmt"

	"github.com/aqlanhadi/kwgn/extractor/common"
)

// GetOrCreateAccount finds an existing account by number or creates a new one.
// Uses INSERT ON CONFLICT to safely handle concurrent access and prevent duplicates.
func (db *DB) GetOrCreateAccount(ctx context.Context, account common.Account) (string, error) {
	var id, institution string

	// The extractor states the institution (the statement format is the fact);
	// accounts.institution is NOT NULL with no default, so an extractor that
	// forgets is refused here with a reason rather than by a constraint error.
	if account.Institution == "" {
		return "", fmt.Errorf("account %s: the extractor did not say which institution issues it", account.AccountNumber)
	}

	// Upsert: insert if not exists, update if exists, always return the id.
	// This is atomic and safe against duplicates even without application-level locking.
	// institution is written once, at creation, and never updated: a statement
	// claiming another institution for a known account number is a contradiction
	// to report, not a correction to apply.
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO accounts (account_number, account_name, account_type, institution, reconciliable)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (account_number) DO UPDATE SET
			account_name = EXCLUDED.account_name,
			account_type = CASE WHEN EXCLUDED.account_type != '' THEN EXCLUDED.account_type ELSE accounts.account_type END,
			reconciliable = EXCLUDED.reconciliable,
			updated_at = NOW()
		RETURNING id, institution
	`, account.AccountNumber, account.AccountName, account.AccountType, account.Institution, account.Reconciliable).Scan(&id, &institution)

	if err != nil {
		return "", fmt.Errorf("failed to upsert account: %w", err)
	}
	if institution != account.Institution {
		return "", fmt.Errorf("account %s is stored as %s, but this statement says %s", account.AccountNumber, institution, account.Institution)
	}

	return id, nil
}
