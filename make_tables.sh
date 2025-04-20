#!/bin/bash
# Database connection settings (adjust as needed)
DB_NAME="kewangan_v2"
DB_USER="postgres"
DB_PASS="postgres"
DB_HOST="localhost"
DB_PORT="5432"

psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -W "$DB_PASS" -d "$DB_NAME" << 'EOF'
-- Create the accounts table
CREATE TABLE IF NOT EXISTS accounts (
    number VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    debit_credit VARCHAR(255) NOT NULL
);

-- Create the statements table
CREATE TABLE IF NOT EXISTS statements (
    account VARCHAR(255) NOT NULL,
    source VARCHAR(255) NOT NULL PRIMARY KEY,
    date timestamptz NOT NULL,
    starting_balance NUMERIC(15,2) NOT NULL,
    ending_balance NUMERIC(15,2) NOT NULL,
    total_debit NUMERIC(15,2) NOT NULL,
    total_credit NUMERIC(15,2) NOT NULL,
    nett NUMERIC(15,2) NOT NULL,
    FOREIGN KEY(account) REFERENCES accounts(number)
);

-- Create the transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    source VARCHAR(255) NOT NULL,
    sequence INT4 NOT NULL,
    date timestamptz NOT NULL,
    descriptions TEXT[] NOT NULL,
    type VARCHAR(255) NOT NULL,
    amount NUMERIC(15,2),
    balance NUMERIC(15,2),
    tags TEXT[],
    ref VARCHAR(255) UNIQUE,
    FOREIGN KEY(source) REFERENCES statements(source)
    
);

-- Conditionally add the unique constraint to transactions
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'unique_source_sequence'
          AND conrelid = 'transactions'::regclass
    ) THEN
        ALTER TABLE transactions
            ADD CONSTRAINT unique_source_sequence UNIQUE (source, sequence);
    END IF;
END$$;
EOF

if [ $? -eq 0 ]; then
    echo "Tables and constraints created successfully."
else
    echo "An error occurred while creating tables or constraints."
fi
