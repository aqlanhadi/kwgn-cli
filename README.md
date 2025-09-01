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
./kwgn extract -f <folder-or-file> [--transaction-only] [--statement-only] [--statement-type <statement-type>] [--config <config-file>]
```

- `-f, --folder` : Folder or file to scan for statements (required)
- `--transaction-only` : Output only transactions (JSON array)
- `--statement-only` : Output only statement details (no transactions)
- `--statement-type` : Override statement type detection (e.g., MAYBANK_CASA_AND_MAE)
  Possible values: `MAYBANK_CASA_AND_MAE`, `MAYBANK_2_CC`, `TNG`
- `--config` : Path to config file (default: ./.kwgn.yaml)
- `--output` : Output folder (default: .)

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

- `--port, -p` : Port to run the API server (default: 8080)
- `--statement-only` : Default to statement-only output
- `--transaction-only` : Default to transaction-only output

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

---

## Releasing to GitHub

### Automated Releases (Recommended)

1. **Create a git tag:**

   ```sh
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically:**
   - Build binaries for all supported platforms
   - Create release archives (.tar.gz for Unix, .zip for Windows)
   - Create a GitHub release
   - Upload all binaries as release assets

### Manual Releases

If you prefer to release manually or don't want to use GitHub Actions:

1. **Build release binaries:**

   ```sh
   make release-archive VERSION=1.0.0
   ```

2. **Use the release script:**

   ```sh
   # Build and create GitHub release automatically
   ./release.sh v1.0.0 --tag

   # Or just build without creating tag
   ./release.sh v1.0.0
   ```

3. **Manual upload:**
   - Go to GitHub → Releases → Create New Release
   - Upload files from `./bin/archives/`

### Supported Platforms

The build system creates binaries for:

- **Linux:** amd64, 386, arm, arm64
- **macOS:** amd64 (Intel), arm64 (Apple Silicon)

### Build Commands

```sh
# Build for current platform
make build

# Build for specific platform
make build-cross GOOS=linux GOARCH=amd64

# Build for all platforms
make release-build

# Create release archives
make release-archive

# Show all available commands
make help
```

### Version Information

The binary includes version information that can be checked with:

```sh
./kwgn --version
```

---

## License

MIT
