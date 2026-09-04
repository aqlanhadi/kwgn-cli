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
    account_type VARCHAR(100),
    -- No debit_credit column. It recorded an account's natural balance
    -- direction, which account_type already determines: a credit_card is always
    -- 'credit' and a cash account always 'debit'. mbb_2_cc hardcoded the string
    -- rather than deriving it, which is the tell -- two columns, one fact, and a
    -- way for them to disagree. Removed with the "drcr" config key that fed it.
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
    tag TEXT,
    verified BOOLEAN DEFAULT false,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    tags TEXT[] DEFAULT '{}',
    data JSONB DEFAULT '{}',

    UNIQUE(statement_id, sequence)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_statements_account_id ON statements(account_id);
CREATE INDEX IF NOT EXISTS idx_statements_date ON statements(statement_date);
CREATE INDEX IF NOT EXISTS idx_transactions_statement_id ON transactions(statement_id);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference) WHERE reference != '';

-- Unique index for idempotent TNG CSV imports
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique_reference
ON transactions(statement_id, reference) WHERE reference != '';
`

// migrateDDL adds new columns to existing tables and ensures constraints exist.
// This handles the case where tables were created externally (e.g. by Drizzle/Supabase migrations)
// with a different schema than the DDL above.
const migrateDDL = `
-- Add missing columns to transactions.
--
-- ONLY columns kwgn-cli itself writes belong here. This block runs on every
-- import, so anything listed is silently RE-CREATED after a migration drops
-- it. "verified" and "verified_at" were listed here and written by nothing --
-- so a migration that dropped them saw them reappear, empty, on the next
-- import, for months.
--
-- "tags" and "data" are gone too, along with the INSERT that wrote them. Only
-- the TnG CSV extractor ever filled them (row sector, raw source fields) and
-- no TnG data is loaded: this schema is Maybank statements only for now.
-- The extractor still populates common.Transaction.Tags/.Data, so restoring
-- them is this block plus the column list in CreateTransactionsIdempotent --
-- see the note there.
--
-- Before adding a column here, check an extractor actually writes it AND that
-- its data is loaded. If only the web app or a classifier writes it, it
-- belongs in the Drizzle migration alone, and repeating it here just gives the
-- two schemas a way to disagree.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'transactions' AND column_name = 'description') THEN
        ALTER TABLE transactions ADD COLUMN description TEXT;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'transactions' AND column_name = 'tag') THEN
        ALTER TABLE transactions ADD COLUMN tag TEXT;
    END IF;
END $$;

-- Make account_type nullable (may have been NOT NULL in older schemas)
DO $$ BEGIN
    ALTER TABLE accounts ALTER COLUMN account_type DROP NOT NULL;
EXCEPTION
    WHEN others THEN NULL;
END $$;

-- Unique index for idempotent reference handling
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique_reference
ON transactions(statement_id, reference) WHERE reference != '';

-- Ensure constraints exist on tables that may have been created externally
-- without the constraints defined in the DDL above.

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'accounts'::regclass AND contype = 'p') THEN
        ALTER TABLE accounts ADD PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'accounts'::regclass AND contype = 'u') THEN
        ALTER TABLE accounts ADD CONSTRAINT accounts_account_number_key UNIQUE (account_number);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'statements'::regclass AND contype = 'p') THEN
        ALTER TABLE statements ADD PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'statements'::regclass AND conname = 'statements_account_id_statement_date_key') THEN
        ALTER TABLE statements ADD CONSTRAINT statements_account_id_statement_date_key UNIQUE (account_id, statement_date);
    END IF;
EXCEPTION WHEN duplicate_table THEN NULL;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'statements'::regclass AND contype = 'f') THEN
        ALTER TABLE statements ADD CONSTRAINT statements_account_id_fkey
            FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;
    END IF;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'transactions'::regclass AND contype = 'p') THEN
        ALTER TABLE transactions ADD PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'transactions'::regclass AND conname = 'transactions_statement_id_sequence_key') THEN
        ALTER TABLE transactions ADD CONSTRAINT transactions_statement_id_sequence_key UNIQUE (statement_id, sequence);
    END IF;
EXCEPTION WHEN duplicate_table THEN NULL;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'transactions'::regclass AND contype = 'f') THEN
        ALTER TABLE transactions ADD CONSTRAINT transactions_statement_id_fkey
            FOREIGN KEY (statement_id) REFERENCES statements(id) ON DELETE CASCADE;
    END IF;
EXCEPTION WHEN duplicate_object THEN NULL;
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
