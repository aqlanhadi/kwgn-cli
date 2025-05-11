# kwgn

A utility to extract structured data from Malaysian financial statement PDFs. Supports both command-line and HTTP API usage.

---

## Installation

Clone the repo and build:

```sh
git clone <your-repo-url>
cd kwgn-cli
go build -o kwgn
```

---

## CLI Usage

Extract statements from a folder or file:

```sh
./kwgn extract -f <folder-or-file> [--transaction-only] [--statement-only] [--config <config-file>]
```

- `-f, --folder`         : Folder or file to scan for statements (required)
- `--transaction-only`   : Output only transactions (JSON array)
- `--statement-only`     : Output only statement details (no transactions)
- `--config`             : Path to config file (default: ./.kwgn.yaml)
- `--output`             : Output folder (default: .)

**Example:**

```sh
./kwgn extract -f ./statements --transaction-only
```

---

## API Usage

Start the API server:

```sh
./kwgn serve [--port 8080] [--statement-only] [--transaction-only]
```

- `--port, -p`           : Port to run the API server (default: 8080)
- `--statement-only`     : Default to statement-only output
- `--transaction-only`   : Default to transaction-only output

### POST /extract

Accepts a PDF file upload and returns extracted data as JSON.

- **Endpoint:** `POST /extract`
- **Form field:** `file` (the PDF file)
- **Optional form/query params:**
  - `statement_only=true` (overrides default)
  - `transaction_only=true` (overrides default)

**Example using curl:**

```sh
curl -F "file=@/path/to/statement.pdf" \
     -F "statement_only=true" \
     http://localhost:8080/extract
```

Or with query params:

```sh
curl -F "file=@/path/to/statement.pdf" \
     "http://localhost:8080/extract?transaction_only=true"
```

---

## Output

- By default, outputs both statement details and transactions as JSON.
- Use `--transaction-only` or `transaction_only=true` to get only transactions.
- Use `--statement-only` or `statement_only=true` to get only statement details (no transactions).

---

## Configuration

- Uses a YAML config file (default: `.kwgn.yaml`).
- See sample config for account and statement patterns.

---

## License

MIT 