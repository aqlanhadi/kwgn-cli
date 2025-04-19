#!/bin/bash
set -euo pipefail

# Check if directory argument is provided
if [ -z "${1:-}" ]; then
  echo "Usage: $0 <directory>"
  exit 1
fi

# Directory with files (from argument) and PostgreSQL connection string
directory="$1"
PG_CONN="postgresql://postgres:postgres@localhost:5432/kewangan_v2"

# Export the connection string so it's available in subshells
export PG_CONN

# Function to process a single file
process_file() {
  local file="$1"
  echo "Processing $file ..."

  # Extract JSON using your tool (kwgn extract)
  local json
  json=$(kwgn extract -f "$file")

  # Skip if JSON is empty (i.e. {} has no keys)
  if [ "$(echo "$json" | jq 'keys | length')" -eq 0 ]; then
    echo "Skipping $file (empty JSON)"
    return
  fi

  # Extract account and statement values in one go
  local ACCOUNT_NUMBER
  ACCOUNT_NUMBER=$(echo "$json" | jq -r '.account.account_number')
  local ACCOUNT_NAME
  ACCOUNT_NAME=$(echo "$json" | jq -r '.account.account_name')
  local ACCOUNT_TYPE
  ACCOUNT_TYPE=$(echo "$json" | jq -r '.account.account_type')
  local DEBIT_CREDIT
  DEBIT_CREDIT=$(echo "$json" | jq -r '.account.debit_credit')

  local SOURCE
  SOURCE=$(echo "$json" | jq -r '.source')
  local STATEMENT_DATE
  STATEMENT_DATE=$(echo "$json" | jq -r '.statement_date')
  local STARTING_BALANCE
  STARTING_BALANCE=$(echo "$json" | jq -r '.starting_balance')
  local ENDING_BALANCE
  ENDING_BALANCE=$(echo "$json" | jq -r '.ending_balance')
  local TOTAL_DEBIT
  TOTAL_DEBIT=$(echo "$json" | jq -r '.total_debit')
  local TOTAL_CREDIT
  TOTAL_CREDIT=$(echo "$json" | jq -r '.total_credit')
  local NETT
  NETT=$(echo "$json" | jq -r '.nett')

  # Upsert Account
  psql "$PG_CONN" <<EOF
INSERT INTO accounts (number, name, type, debit_credit)
VALUES ('$ACCOUNT_NUMBER', '$ACCOUNT_NAME', '$ACCOUNT_TYPE', '$DEBIT_CREDIT')
ON CONFLICT (number) DO UPDATE
SET name = EXCLUDED.name,
    type = EXCLUDED.type,
    debit_credit = EXCLUDED.debit_credit;
EOF

  # Upsert Statement
  psql "$PG_CONN" <<EOF
INSERT INTO statements (account, source, date, starting_balance, ending_balance, total_debit, total_credit, nett)
VALUES ('$ACCOUNT_NUMBER', '$SOURCE', '$STATEMENT_DATE', $STARTING_BALANCE, $ENDING_BALANCE, $TOTAL_DEBIT, $TOTAL_CREDIT, $NETT)
ON CONFLICT (source) DO UPDATE
SET date = EXCLUDED.date,
    starting_balance = EXCLUDED.starting_balance,
    ending_balance = EXCLUDED.ending_balance,
    total_debit = EXCLUDED.total_debit,
    total_credit = EXCLUDED.total_credit,
    nett = EXCLUDED.nett;
EOF

  # Delete any existing transactions for the given source (to avoid duplicates)
  # IMPORTANT: Running DELETE concurrently might require careful consideration
  # depending on transaction isolation levels and potential deadlocks.
  psql "$PG_CONN" <<EOF
DELETE FROM transactions WHERE source = '$SOURCE';
EOF

  # Build a bulk INSERT value list for transactions.
  local transactions_bulk=""
  local SEQUENCE TXN_DATE TYPE AMOUNT BALANCE DESCRIPTIONS FORMATTED_DESCRIPTIONS row
  while IFS= read -r txn; do
    SEQUENCE=$(echo "$txn" | jq -r '.sequence')
    TXN_DATE=$(echo "$txn" | jq -r '.date')
    TYPE=$(echo "$txn" | jq -r '.type')
    AMOUNT=$(echo "$txn" | jq -r '.amount')
    BALANCE=$(echo "$txn" | jq -r '.balance')

    # Process the descriptions array: escape single quotes and format as PG array
    DESCRIPTIONS=$(echo "$txn" | jq -r '.descriptions | map("'"'"'" + (. | gsub("'"'"'" ; "''")) + "'"'"'" ) | join(",")')
    FORMATTED_DESCRIPTIONS="ARRAY[$DESCRIPTIONS]"

    # Create a row for the bulk insert
    row="('$SOURCE', $SEQUENCE, '$TXN_DATE', $FORMATTED_DESCRIPTIONS::text[], '$TYPE', $AMOUNT, $BALANCE)"
    if [ -z "$transactions_bulk" ]; then
      transactions_bulk="$row"
    else
      transactions_bulk="$transactions_bulk, $row"
    fi
  done < <(jq -c '.transactions[]' <<< "$json")

  # If there are transaction rows, perform a bulk upsert
  if [ -n "$transactions_bulk" ]; then
    psql "$PG_CONN" <<EOF
INSERT INTO transactions (source, sequence, date, descriptions, type, amount, balance)
VALUES $transactions_bulk
ON CONFLICT (source, sequence) DO UPDATE
SET date = EXCLUDED.date,
    descriptions = EXCLUDED.descriptions,
    type = EXCLUDED.type,
    amount = EXCLUDED.amount,
    balance = EXCLUDED.balance;
EOF
  fi

  echo "Data import completed for $file"
}

# Export the function so it's available to parallel
export -f process_file

# Find all files (not directories) in the target directory and process them in parallel
# Use --line-buffer to ensure output from different jobs is not interleaved randomly.
# Use -j $(nproc) to use all available processors, or specify a number e.g. -j 4
# Use --halt now,fail=1 to stop all jobs if one fails.
echo "Starting parallel import from directory: $directory"
find "$directory" -maxdepth 1 -type f -print0 | parallel --null --line-buffer -j $(nproc) --halt now,fail=1 process_file {}

echo "Parallel import finished."

# Note: Ensure GNU parallel is installed (e.g., sudo apt install parallel)
# Consider potential database contention with concurrent writes/deletes.
# Adjust -j parameter based on system resources and database capacity. 