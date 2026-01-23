package postgres

import (
	"context"
	"fmt"

	"github.com/aqlanhadi/kwgn/extractor/common"
)

// GetOrCreateAccount finds an existing account by number or creates a new one
func (db *DB) GetOrCreateAccount(ctx context.Context, account common.Account) (string, error) {
	var id string

	// Try to find existing account
	err := db.Pool.QueryRow(ctx, `
		SELECT id FROM accounts WHERE account_number = $1
	`, account.AccountNumber).Scan(&id)

	if err == nil {
		// Account exists, update fields
		// - Always update account_name (source identifier from extractor)
		// - Only update account_type if non-empty (preserve user-set values)
		// - Update debit_credit and reconciliable if non-empty
		_, err = db.Pool.Exec(ctx, `
			UPDATE accounts
			SET account_name = $1,
			    account_type = CASE WHEN $2::text != '' THEN $2 ELSE account_type END,
			    debit_credit = CASE WHEN $3::text != '' THEN $3 ELSE debit_credit END,
			    reconciliable = $4,
			    updated_at = NOW()
			WHERE id = $5
		`, account.AccountName, account.AccountType, account.DebitCredit, account.Reconciliable, id)
		if err != nil {
			return "", fmt.Errorf("failed to update account: %w", err)
		}
		return id, nil
	}

	// Create new account
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO accounts (account_number, account_name, account_type, debit_credit, reconciliable)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, account.AccountNumber, account.AccountName, account.AccountType, account.DebitCredit, account.Reconciliable).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("failed to create account: %w", err)
	}

	return id, nil
}
