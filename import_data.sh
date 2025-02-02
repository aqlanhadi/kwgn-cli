#!/bin/bash
set -euo pipefail

# Directory with files and PostgreSQL connection string
directory="/mnt/c/Users/aqlan/Downloads/Consolidated Statements/"
PG_CONN="postgresql://postgres:postgres@localhost:5432/kewangan_v2"

# Loop through each file in the directory
for file in "$directory"/*; do
  if [ -f "$file" ]; then

    # Extract JSON using your tool (kwgn extract)
    json=$(kwgn extract -f "$file")

    # Skip if JSON is empty (i.e. {} has no keys)
    if [ "$(echo "$json" | jq 'keys | length')" -eq 0 ]; then
      echo "Skipping $file (empty JSON)"
      continue
    fi

    echo "Processing $file ..."

    # Extract account and statement values in one go
    ACCOUNT_NUMBER=$(echo "$json" | jq -r '.account.account_number')
    ACCOUNT_NAME=$(echo "$json" | jq -r '.account.account_name')
    ACCOUNT_TYPE=$(echo "$json" | jq -r '.account.account_type')
    DEBIT_CREDIT=$(echo "$json" | jq -r '.account.debit_credit')

    SOURCE=$(echo "$json" | jq -r '.source')
    STATEMENT_DATE=$(echo "$json" | jq -r '.statement_date')
    STARTING_BALANCE=$(echo "$json" | jq -r '.starting_balance')
    ENDING_BALANCE=$(echo "$json" | jq -r '.ending_balance')
    TOTAL_DEBIT=$(echo "$json" | jq -r '.total_debit')
    TOTAL_CREDIT=$(echo "$json" | jq -r '.total_credit')
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
    psql "$PG_CONN" <<EOF
DELETE FROM transactions WHERE source = '$SOURCE';
EOF

    # Build a bulk INSERT value list for transactions.
    # For each transaction we will:
    # - Escape single quotes in descriptions (by doubling them)
    # - Format the descriptions as a PostgreSQL array literal (using single quotes)
    transactions_bulk=""
    while IFS= read -r txn; do
      SEQUENCE=$(echo "$txn" | jq -r '.sequence')
      TXN_DATE=$(echo "$txn" | jq -r '.date')
      TYPE=$(echo "$txn" | jq -r '.type')
      AMOUNT=$(echo "$txn" | jq -r '.amount')
      BALANCE=$(echo "$txn" | jq -r '.balance')

      # Process the descriptions array:
      # For each description, escape internal single quotes and wrap with single quotes.
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
  fi
done
