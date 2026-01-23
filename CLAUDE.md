# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**kwgn** is a financial statement extraction utility for Malaysian bank statements. It extracts structured transaction data from PDF statements and provides both CLI and HTTP API interfaces.

**Key capabilities:**
- Extract account details, balances, and transactions from PDF statements and CSV exports
- Support for multiple Malaysian banks/services (Maybank CASA/MAE, Maybank Credit Card, TNG eWallet, TNG CSV Export)
- Dual interface: Command-line for batch processing, HTTP API for integrations
- PostgreSQL integration for storing extracted data
- Cross-platform Go binary (Linux, macOS, Windows)

## Build & Test Commands

### Build
```bash
# Build for current platform
go build -o kwgn .
# or
make build

# Build for Linux (commonly used for deployment)
make build-linux

# Cross-platform builds available in Makefile
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests for specific extractor
go test ./extractor/mbb_mae_and_casa/
go test ./extractor/mbb_2_cc/

# Run tests with verbose output
go test -v ./...
```

### Running the Application
```bash
# Extract from PDF files
./kwgn extract -f <file-or-folder> [--transaction-only] [--statement-only] [--config .kwgn.yaml]

# Extract from TNG CSV export files (auto-detects CSV format)
./kwgn extract -f <tng-csv-file.csv>

# Or explicitly specify the type
./kwgn extract -f <tng-csv-file.csv> --type TNG_CSV_EXPORT

# Start API server
./kwgn serve --port 8080

# Import extracted data to PostgreSQL (supports PDF and CSV, auto-detects format)
./kwgn import -f <file> --db-url <connection-string>

# Import TNG CSV with explicit type (optional, auto-detection works)
./kwgn import -f <tng-csv-file.csv> --db-url <connection-string> --type TNG_CSV_EXPORT
```

### Docker
```bash
# Build image
docker build -t kwgn .

# Run API server
docker run -p 8080:8080 kwgn serve
```

## Architecture Overview

### High-Level Structure

The codebase follows a **modular extractor pattern** where each bank/statement type has its own isolated extraction logic:

```
Entry Points (cmd/)
    ↓
Main Orchestrator (extractor/extract.go)
    ↓
Statement-Type Detection → Route to Specific Extractor
    ↓                              ↓
PDF Text Extraction        Bank-Specific Parsers
(common/utils.go)          (mbb_*/, tng/, tng_email/)
    ↓                              ↓
Row-based Text → Regex Pattern Matching → Structured Data
    ↓
Output Formatting (transaction_only, statement_only, or full)
```

### Key Architectural Components

1. **extractor/extract.go** - Main orchestrator
   - Routes PDFs to appropriate extractors based on config or auto-detection
   - Handles output formatting (transaction-only, statement-only, full)
   - Manages batch processing of directories

2. **extractor/common/** - Shared utilities and types
   - `types.go`: Core data structures (Statement, Transaction, Account)
   - `utils.go`: PDF text extraction using dslipak/pdf library
   - `parsing.go`: Date and decimal parsing helpers

3. **Statement-Specific Extractors** (mbb_mae_and_casa/, mbb_2_cc/, tng/, tng_email/, tng_csv_export/)
   - Each extractor is self-contained with its own `extract.go`
   - PDF extractors use regex patterns from YAML config to parse statement-specific formats
   - CSV extractors (tng_csv_export) parse structured CSV data directly
   - Returns standardized `common.Statement` structure

4. **cmd/** - CLI commands using Cobra
   - `root.go`: Configuration loading (Viper), version info
   - `extract.go`: Extract command implementation
   - `serve.go`: HTTP API server
   - `import.go`: PostgreSQL import functionality

5. **api/server.go** - HTTP API server
   - POST /extract: Upload PDF and extract data
   - GET /health: Health check endpoint

6. **integrations/postgres/** - Database integration
   - Connection pooling with pgx
   - Schema management for accounts and transactions
   - Import operations from extracted statements

### Data Flow

**PDF Processing Pipeline:**
1. PDF file → `common.ExtractRowsFromPDFReader()` → Array of text rows
2. Statement type detection (config regex match OR --statement-type override OR auto-detect)
3. Route to specific extractor (e.g., `mbb_mae_and_casa.Extract()`)
4. Extractor uses regex patterns from config to parse:
   - Account info (number, name, type)
   - Balances (starting, ending)
   - Individual transactions (date, description, amount, type, balance)
5. Build `common.Statement` with transactions
6. Format output based on flags (full/transaction-only/statement-only)
7. Return as JSON

### Configuration System

The application uses **Viper** for hierarchical configuration:

**Priority order:** CLI flags → YAML config file → embedded defaults

**Config file location:** `.kwgn.yaml` (or specified via --config)

**Structure:**
- `accounts[]`: Account definitions with regex identifiers to match statements
- `statement.<TYPE>.patterns`: Regex patterns for each statement type
  - `starting_balance`, `ending_balance`: Balance extraction patterns
  - `main_transaction_line`: Primary transaction line pattern
  - Additional patterns for multi-line transactions, dates, amounts

**Critical detail:** Each statement extractor relies heavily on its configured regex patterns. When adding support for new statement formats or fixing parsing issues, the patterns in the YAML config are the primary lever.

## Important Implementation Details

### Financial Data Precision

**CRITICAL:** Always use `decimal.Decimal` (from shopspring/decimal) for monetary amounts, never `float64`. Financial calculations require exact decimal precision.

Example:
```go
import "github.com/shopspring/decimal"

amount := decimal.NewFromFloat(100.50)  // ✓ Correct
balance := decimal.Zero                  // ✓ Correct
total := amount.Add(balance)             // ✓ Correct

var wrong float64 = 100.50               // ✗ Never for money
```

### Adding a New Statement Extractor

To support a new bank or statement format:

1. Create new directory: `extractor/<bank_name>/`
2. Implement `extract.go` with function signature:
   ```go
   func Extract(rows []string, account common.Account) (common.Statement, error)
   ```
3. Add statement type to config template in `cmd/root.go`
4. Import and register in `extractor/extract.go`
5. Add regex patterns to `.kwgn.yaml` config
6. Create test file: `extractor/<bank_name>/extract_test.go`

### Pattern Matching Strategy

All extractors follow a similar pattern:
1. Extract account metadata from header rows
2. Locate starting balance using regex
3. Identify transaction section boundaries
4. Parse each transaction line (date, description, amount, type, balance)
5. Handle multi-line descriptions (continuation patterns)
6. Calculate totals and validate ending balance

**Testing approach:** Each extractor should have tests with real statement samples (anonymized) to validate pattern matching.

### API Integration Points

The codebase is designed to be extended with additional integrations:

- **Current:** PostgreSQL (integrations/postgres/)
- **Potential:** S3 storage, webhook notifications, cloud storage, etc.

Integration pattern:
1. Create `integrations/<service>/` directory
2. Import `extractor/common` types
3. Implement service-specific logic
4. Wire up via new CLI command in `cmd/`

### Testing Philosophy

- **Extractor tests** (`extractor/*/extract_test.go`): Use real PDF samples to validate end-to-end extraction
- **Parsing tests** (`extractor/common/parsing_test.go`): Unit tests for date/decimal parsing
- **API tests** (`api/server_test.go`): HTTP endpoint validation

When fixing extraction bugs, add test cases with the problematic PDF pattern.

## Common Development Workflows

### Debugging Extraction Issues

1. Use `--text-only` flag to see raw extracted text:
   ```bash
   ./kwgn extract -f statement.pdf --text-only
   ```

2. Compare raw text against regex patterns in `.kwgn.yaml`

3. Test pattern changes without rebuilding:
   - Modify `.kwgn.yaml`
   - Re-run extraction (config is loaded at runtime)

### Adding New Features to Existing Extractors

1. Read the extractor file first: `extractor/<bank>/extract.go`
2. Understand the regex patterns in `.kwgn.yaml` for that statement type
3. Modify pattern in config OR modify extraction logic
4. Add test case to `extractor/<bank>/extract_test.go`
5. Run: `go test ./extractor/<bank>/`

## Project-Specific Conventions

- **Error handling:** Log and continue for individual file failures in batch processing (don't fail entire batch)
- **Date formats:** Use `time.Time` with RFC3339 for JSON serialization
- **Balance validation:** Compare `ending_balance` (from statement) vs `calculated_ending_balance` (starting + transactions) to catch parsing errors
- **Regex patterns:** Use named capture groups for clarity when possible
- **Statement types:** Use uppercase with underscores (e.g., `MAYBANK_CASA_AND_MAE`, `TNG_EMAIL`, `TNG_CSV_EXPORT`)

## TNG CSV Export Extractor

The `TNG_CSV_EXPORT` extractor handles CSV files exported from TNG eWallet's transaction history feature.

### Features
- **Auto-detection**: CSV files are automatically detected by their header signature (no need to specify `--type`)
- Parses TNG CSV export format with columns: MFG Number, Trans. No., Transaction Date/Time, etc.
- Groups transactions by MFG Number (card number), creating one statement per card
- Stores additional CSV fields in the `data` JSONB column (sector, entry/exit locations, vehicle info)
- Uses Transaction ID as reference for idempotent imports
- Calculates and validates balances automatically

### Idempotent Import Handling
TNG CSV exports can contain overlapping transaction data across multiple files. The import system handles this by:
1. Using a sentinel date (2000-01-01) for all TNG CSV statements to ensure all transactions go to the same statement per account
2. Using `ON CONFLICT DO NOTHING` on the unique (statement_id, reference) index
3. Setting `reconciliable = false` for TNG CSV accounts

### Transaction Data Column
The `data` JSONB column stores additional TNG-specific fields:
```json
{
  "mfg_number": "2222222222",
  "trans_no": "62894",
  "posted_date": "2025-11-29T00:00:00+08:00",
  "sector": "PARKING",
  "entry_location": "THE MET CORPORATE TOWER",
  "entry_sp": "TC_A18",
  "exit_location": "THE MET CORPORATE TOWER",
  "exit_sp": "TC_A18",
  "reload_location": "",
  "vehicle_class": "00",
  "device_no": "",
  "vehicle_number": ""
}
```

## Dependencies of Note

- **dslipak/pdf**: PDF text extraction (row-based reading)
- **shopspring/decimal**: Exact decimal arithmetic for financial data
- **jackc/pgx/v5**: PostgreSQL driver with connection pooling
- **spf13/cobra**: CLI framework
- **spf13/viper**: Configuration management (YAML, flags, env vars)

## Repository Context

- **Language:** Go 1.24.0
- **Primary interfaces:** CLI (extract, serve, import commands) + HTTP API
- **Target deployment:** Docker containers or standalone binaries
- **Supported platforms:** Linux (amd64, arm64), macOS (Intel, Apple Silicon)
