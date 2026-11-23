# How to Run `invoicer`

This guide shows you step by step how to run the **invoicer** tool to turn brokerage PDF invoices into accounting CSV files or JSON data.

---

## 1. Requirements

- **To run the ready-made tool on Windows:** Nothing extra is needed. Use [bin/invoicer.exe](../bin/invoicer.exe).
- **To rebuild from source, cross-compile for Linux/macOS, or run tests:** Install Go (`1.22` or newer).

---

## 2. Quick Start (30 Seconds)

Open PowerShell in the project folder:

```powershell
cd path\to\invoicer
```

Run `invoicer.exe` on a folder containing your brokerage PDF invoices:

```powershell
.\bin\invoicer.exe -v C:\path\to\my-invoices
```

What this does:
1. Scans the folder you passed for all `.pdf` files.
2. Detects whether each page is **Telda** or **Thndr**, converts ISINs to stock tickers (`EGX:...`), and checks the invoice math.
3. Saves the formatted accounting table to `output\transactions.csv`.

Open `output\transactions.csv` to view the result.

---

## 3. Common Everyday Commands

### Extract specific PDF files
```powershell
.\bin\invoicer.exe -v -o output\november.csv "C:\path\to\Trading_Invoice.pdf" "C:\path\to\E-INVOICE.pdf"
```

### Scan a folder and all its subfolders (`--recursive`)
```powershell
.\bin\invoicer.exe -v --recursive -o output\all_trades.csv C:\path\to\my-invoices
```

### Merge new invoices into an existing CSV without duplicates (`--merge`)
This keeps existing `$CASH` (`Deposit`, `Withdrawal`, `Taxes and fees`) and `Dividend` rows and skips any trades already present:

```powershell
.\bin\invoicer.exe -v --merge "samples\reference\CSV Format.csv" -o output\merged.csv C:\path\to\my-invoices
```

> **Tip:** To update your ledger file in place, pass the same path to both `--merge` and `-o`.

### Combine partial fills into one row per invoice page (`-g order`)
By default (`-g fill`), each partial execution becomes its own CSV row. To group fills on the same invoice page into one average-price row:

```powershell
.\bin\invoicer.exe -g order -o output\orders.csv C:\path\to\my-invoices
```

### Output JSON instead of CSV (great for scripts or web apps)
Print pure JSON directly to the terminal (`stdout`) and skip writing a CSV file:

```powershell
.\bin\invoicer.exe --json - -o none "C:\path\to\E-INVOICE.pdf"
```

Or save both a CSV and an audit JSON file:

```powershell
.\bin\invoicer.exe -v -o output\transactions.csv --json output\audit.json C:\path\to\my-invoices
```

### Preview CSV in the terminal without creating a file
```powershell
.\bin\invoicer.exe -o - C:\path\to\my-invoices
```

### Open password-protected PDFs
```powershell
.\bin\invoicer.exe --password YOUR_PASSWORD -v C:\path\to\protected.pdf
```

---

## 4. Handling New or Unknown Stock Symbols

All 298 listed Egyptian Exchange (EGX) stocks are mapped in [config/symbols.json](../config/symbols.json) alongside the built-in resolver. If you ever trade a brand-new stock and see an unresolved ISIN warning, you have two ways to resolve it:

1. **Update the mapping file (offline):**
   - Open [config/symbols.json](../config/symbols.json).
   - Add your ISIN under `"isin"` or company name under `"names"` (for example: `"EGS656M1C010": "BONY"`).
   - `invoicer` loads `config\symbols.json` (or `symbols.json` in the current folder) automatically, or you can pass `-s path\to\symbols.json`.
2. **Automatic online lookup:**
   - Pass `--online-lookup` to query OpenFIGI automatically:
     ```powershell
     .\bin\invoicer.exe --online-lookup -v C:\path\to\my-invoices
     ```

---

## 5. All Available Flags

| Flag | Default | Simple Explanation |
| :--- | :--- | :--- |
| `-o`, `--out` | `output/transactions.csv` | Where to save the CSV (`-` prints to terminal, `none` skips writing CSV) |
| `-m`, `--merge` | *(empty)* | Existing CSV file to merge into (preserves `$CASH` and `Dividend` rows and skips duplicates) |
| `-g`, `--granularity` | `fill` | `fill` = one row per execution; `order` = one combined row per invoice page |
| `-s`, `--symbols` | *(empty)* | Path to a custom `symbols.json` mapping file |
| `--exchange` | `EGX` | Exchange prefix placed before tickers (e.g., `EGX:EGAL`) |
| `--default-time` | `00:00:00` | `HH:MM:SS` time appended when an invoice only has a date |
| `--sort` | `desc` | Row order: `desc` (newest first), `asc` (oldest first), or `none` |
| `--recursive` | `false` | Look inside subfolders when scanning a directory |
| `--password` | *(empty)* | Password for encrypted PDFs |
| `--online-lookup` | `false` | Query OpenFIGI for ISINs not found locally |
| `--json` | *(empty)* | Write JSON report + `records` to a file (or `-` for terminal `stdout`) |
| `--strict` | `false` | Exit with a non-zero error code if any math check or symbol lookup warns |
| `--version` | `false` | Print invoicer version and exit |
| `-v`, `--verbose` | `false` | Show per-page broker, symbol, quantity, fees, and `[OK]`/`[WARN]` status |

---

## 6. Building from Source & Running Tests

### Rebuild on Windows
```powershell
go build -o bin\invoicer.exe ./cmd/invoicer
```

### Cross-compile for a Linux server
```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o bin/invoicer-linux ./cmd/invoicer
```

### Run all automated tests
```powershell
go test -count=1 -v ./...
```

---

## 7. Running the Web UI (`web/`)

A client-side Next.js workbench lives in `web/`.

```powershell
cd web
npm install
npm run dev
```

Then open `http://localhost:3000`.

For a production build:

```powershell
cd web
npm run typecheck
npm run build
npm run start
```

> **Optional environment variables** (if deploying `web/` independently):
> - `INVOICER_BIN_PATH` — full path to `invoicer.exe` or `invoicer-linux`
> - `INVOICER_ROOT_DIR` — path to the folder containing `bin/`, `samples/`, `cmd/`, and `internal/`

---

## 8. Quick Troubleshooting

- **"no PDF files found in inputs"**: Check that you passed the right folder or `.pdf` filename (remember to pass `--recursive` if PDFs are inside subfolders).
- **"cannot write both CSV (-o -) and JSON (--json -) to stdout"**: Pick `-` for only one output stream (for example, `--json - -o none`).
- **"unresolved ISIN/symbol" warning**: Add the ISIN to `config\symbols.json` or re-run with `--online-lookup`.
- **Reconciliation `[WARN]`**: Run with `--json output\audit.json` to inspect the itemized fee and fill checks for that page.
