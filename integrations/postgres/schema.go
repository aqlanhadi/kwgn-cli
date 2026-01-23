package postgres

import (
	"context"
	"fmt"
)

const ddl = `
-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_number VARCHAR(50) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    account_type VARCHAR(100) DEFAULT NULL,
    debit_credit VARCHAR(10) DEFAULT '',
    reconciliable BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(account_number)
);

-- Statements table with natural key (account_id, statement_date)
CREATE TABLE IF NOT EXISTS statements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source VARCHAR(255) NOT NULL,
    statement_date DATE NOT NULL,
    starting_balance NUMERIC(18,2),
    ending_balance NUMERIC(18,2),
    calculated_ending_balance NUMERIC(18,2),
    total_credit NUMERIC(18,2) NOT NULL,
    total_debit NUMERIC(18,2) NOT NULL,
    nett NUMERIC(18,2) NOT NULL,
    transaction_start_date DATE,
    transaction_end_date DATE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Natural key for deduplication
    UNIQUE(account_id, statement_date)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    statement_id UUID NOT NULL REFERENCES statements(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    date DATE NOT NULL,
    descriptions TEXT[] NOT NULL,
    description TEXT,
    type VARCHAR(10) NOT NULL,
    amount NUMERIC(18,2) NOT NULL,
    balance NUMERIC(18,2) NOT NULL,
    reference VARCHAR(255) DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    tags TEXT[] DEFAULT '{}',
    data JSONB DEFAULT '{}',

    -- Prevent duplicate transactions within a statement
    UNIQUE(statement_id, sequence)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_statements_account_id ON statements(account_id);
CREATE INDEX IF NOT EXISTS idx_statements_date ON statements(statement_date);
CREATE INDEX IF NOT EXISTS idx_transactions_statement_id ON transactions(statement_id);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference) WHERE reference != '';

-- Unique index for idempotent TNG CSV imports (prevents duplicate Transaction IDs per account)
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique_reference
ON transactions(statement_id, reference) WHERE reference != '';
`

// migrateDDL adds new columns to existing tables
const migrateDDL = `
-- Add data column if not exists
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'transactions' AND column_name = 'data') THEN
        ALTER TABLE transactions ADD COLUMN data JSONB DEFAULT '{}';
    END IF;
END $$;

-- Add tags column if not exists
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'transactions' AND column_name = 'tags') THEN
        ALTER TABLE transactions ADD COLUMN tags TEXT[] DEFAULT '{}';
    END IF;
END $$;

-- Make account_type nullable
DO $$ BEGIN
    ALTER TABLE accounts ALTER COLUMN account_type DROP NOT NULL;
EXCEPTION
    WHEN others THEN
        -- Column already nullable or doesn't exist, ignore
        NULL;
END $$;

-- Create unique index for idempotent reference handling
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique_reference
ON transactions(statement_id, reference) WHERE reference != '';

-- Add description column if not exists (normalized single string for matching)
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'transactions' AND column_name = 'description') THEN
        ALTER TABLE transactions ADD COLUMN description TEXT;
    END IF;
END $$;
`

// EnsureSchema creates tables if they don't exist and runs migrations
func (db *DB) EnsureSchema(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations for existing tables
	_, err = db.Pool.Exec(ctx, migrateDDL)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
