# CLAUDE.md

Guidance for Claude Code in this repository.

## What this is

**kwgn** extracts structured data from Malaysian financial statements, and does nothing else.
It is a Go module (`github.com/aqlanhadi/kwgn`) with three faces:

- a **library**, `extractor.ExtractFile` / `extractor.ExtractDir`, which kwgn-ledger imports;
- a **CLI**: `kwgn extract` (JSON to stdout), `kwgn serve` (HTTP `POST /extract`), and `kwgn tng` (portal sync);
- an **HTTP API** (`api/server.go`).

Storing, classifying and checking the data belongs to **kwgn-ledger** (`../kwgn-ledger`). There is
no database code here; the old Postgres `import` command was removed on 2026-11-24.

Formats: Maybank CASA/MAE PDFs (`mbb_mae_and_casa`), Maybank 2 card PDFs, one statement per card
(`mbb_2_cc`), TnG eWallet PDFs (`tng`), TnG email statements (`tng_email`), TnG card CSV exports
(`tng_csv_export`).

## Commands

```bash
go build -o kwgn .            # or: make build / make build-linux
go test ./...
./kwgn extract -f <file-or-folder> [--transaction-only|--statement-only] [--statement-type MAYBANK_2_CC]
./kwgn extract -f statement.pdf --text-only     # raw PDF text, for debugging a pattern
./kwgn serve --port 8080
./kwgn tng login | sync | status | logout
```

## Layout

```
extractor/extract.go     dispatch: ExtractFile, ExtractDir, ProcessReaderMulti, CLI output shaping
extractor/common/        types.go (Statement, Account, Transaction), patterns.go (every format's regexes),
                         PDF row extraction, decimal/date helpers
extractor/<format>/      one package per statement format, each with Extract(...)
integrations/tng/        TnG portal client + on-disk row store (~/kwgn-data/tng/*.json)
api/                     HTTP server
cmd/                     cobra commands
```

**No configuration file.** A statement format's patterns are facts about the issuer's layout, so they
live in `extractor/common/patterns.go` and are read through `common.Pattern(format, key)`. Account
number, product name (`AccountName` is the product the statement prints, e.g. `MAE`), `AccountType`,
`Institution` and `BalanceBasis` all come from the document.

Detection without `--statement-type`: CSVs by header; PDFs try `MAYBANK_2_CC` (multi-card) first, then
`MAYBANK_CASA_AND_MAE`, `TNG`, `TNG_EMAIL`, taking the first that yields an account or rows.

## Rules

- **Money is `decimal.Decimal`** (shopspring), never `float64`.
- **Sign convention** is baked into `amount` by account type: cash debit negative / credit positive;
  card debit positive / credit negative, so `starting + total_debit + total_credit = ending` holds for both.
- A change to an extractor must keep the output of the real statement folder byte-identical unless the
  change is meant to alter it. Check by extracting the folder with the old and new binary and `cmp`
  the JSON; kwgn-ledger's layer-1 checks are the second line.
- Statement types are upper snake case (`MAYBANK_CASA_AND_MAE`).

## Adding a statement format

1. `extractor/<format>/extract.go` with `Extract(path string, rows *[]string) common.Statement`
   (set `Account.Institution`, `Account.AccountType` and `BalanceBasis`).
2. Its patterns under `statement.<FORMAT>.patterns` in `extractor/common/patterns.go`.
3. Register it in `processStatementByType` and, if it should be auto-detected, `pdfFormats`.
4. Tests in `extractor/<format>/extract_test.go`, with anonymised text rows.

## TNG CSV Export Extractor

The `TNG_CSV_EXPORT` extractor handles CSV files exported from TNG eWallet's transaction history feature.

### Features
- **Auto-detection**: CSV files are automatically detected by their header signature (no need to specify `--type`)
- Parses TNG CSV export format with columns: MFG Number, Trans. No., Transaction Date/Time, etc.
- Groups transactions by MFG Number (card number), creating one statement per card
- Carries extra CSV fields in `Transaction.Data` (sector, entry/exit locations, vehicle info)
- Uses Transaction ID as `ref`
- Calculates and validates balances automatically

### Overlapping exports
TNG CSV exports overlap across files. Every TNG CSV statement uses the sentinel date 2000-01-01 so an importer can dedupe on the transaction reference (the Transaction ID).

### Transaction `data`
`Transaction.Data` carries the extra TNG fields:
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

## TNG Portal Sync (`kwgn tng`)

`integrations/tng` pulls history straight from the Touch 'n Go consumer portal's private APIs (read from the portal's JavaScript, not published; expect breakage).

```bash
./kwgn tng login    # prints a QR in the terminal (--open for an image); scan it in the TNG eWallet app
./kwgn tng sync     # merges into ~/kwgn-data/tng/{ewallet,card,rfid}.json; shows the login QR first if not signed in
./kwgn tng status   # shows when the login expires, and which days are covered
```

- **Coverage** (`~/kwgn-data/tng/coverage.json`, `integrations/tng/coverage.go`): the days each source was fetched completely (rows received = count the portal reported). The newest 14 days are never counted (late posting; the next sync confirms them). The eWallet keeps about 13 months (measured 2026-10-12), so eWallet days older than 365 days at fetch time are not counted, and a gap older than that is reported `LOST`; card/RFID retention is unknown and not applied. Coverage means "fetched everything listed", not "proven": the card's running balance proves completeness, the eWallet needs a balance from the app.

- **Login lasts 30 minutes** (access token 5 min, renewed automatically with the refresh token; the refresh token cannot be extended). Unattended sync is not possible: each sync session starts with a QR scan. The session file lives in the OS config dir (`~/Library/Application Support/kwgn/tng-session.json`), mode 0600.
- **Two sites.** eWallet: `api-eportal.tngdigital.com.my/v1/`, `Authorization: <token>`, 90-day windows, 10 rows per page, ~2 years back (422 beyond). Card + RFID: `consumer2.touchngo.com.my/mx/api/v1/`, entered by SSO via `auth/tngd/bind/verify` → `sso-login-authenticate`; year windows, 1000-row pages, history back to 2021.
- **The card portal issues its token once per eWallet login.** A second SSO answers `000` with no token, so the token is saved in the session file.
- **Rows are stored raw and merged by key** (`walletRef`; `cardMfgNo|txSeqNo|txId`; RFID `id`). The eWallet drops rows older than ~13 months, so the on-disk files are the only long-term record; never overwrite them with a fresh pull.
- kwgn-ledger reads these files through `tng.LoadRows`. RFID tolls also appear in the eWallet history, and card reloads appear on both the eWallet and card side, so only the eWallet is a ledger there.

## Dependencies

dslipak/pdf (PDF text), shopspring/decimal (money), spf13/cobra + viper (CLI flags; viper also parses
the embedded pattern table privately).
