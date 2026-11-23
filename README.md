# invoicer

Pure-Go, zero-CGO smart PDF invoice extraction engine that converts brokerage PDF invoices (Telda Securities, Thndr Securities Brokerage, and generic trade confirmations) into accounting-ready CSV files matching [samples/reference/CSV Format.csv](samples/reference/CSV%20Format.csv) or structured JSON for web apps.

## Documentation

- **[How to Run (`docs/HOW_TO_RUN.md`)](docs/HOW_TO_RUN.md)** — Simple step-by-step guide, copy-paste commands, flags, custom symbols, and building from source.
- **[Project Overview & Next.js Integration (`docs/PROJECT_AND_NEXTJS_INTEGRATION.md`)](docs/PROJECT_AND_NEXTJS_INTEGRATION.md)** — How the engine works and how to plug it into a Next.js upload flow backed by a `transactions` database table.

## Project Structure

```text
invoicer/
├── bin/                   # Compiled executable (bin/invoicer.exe)
├── cmd/invoicer/          # CLI entrypoint (main.go)
├── config/                # Symbol override template (symbols.example.json)
├── docs/                  # User & Next.js integration guides
├── internal/
│   ├── csvio/             # Accounting CSV reader, formatter, merger, deduplicator
│   ├── engine/            # Concurrent PDF discovery and extraction pipeline
│   ├── model/             # Domain models (InvoicePage, CSVRecord, AuditReport)
│   ├── parser/            # Multi-broker parsers (Telda, Thndr, Generic fallback)
│   ├── pdfextract/        # Pure-Go layout-aware PDF parser built on pdfcpu
│   ├── reconcile/         # Math verification & exact integer-cent fee allocation
│   └── symbols/           # EGX ISIN & company-name -> ticker resolver
├── output/                # Default output folder (output/transactions.csv)
└── samples/
    ├── invoices/          # Sample Telda & Thndr PDF invoices
    └── reference/         # Target CSV Format.csv specification
```

## Quick Start

```powershell
# Build binary
go build -o bin\invoicer.exe ./cmd/invoicer

# Extract all sample PDFs into output\transactions.csv
.\bin\invoicer.exe -v samples\invoices

# Merge into an existing accounting CSV without duplicates
.\bin\invoicer.exe -v --merge "samples\reference\CSV Format.csv" -o output\merged.csv samples\invoices

# Emit JSON (for Next.js / APIs) without writing a CSV file
.\bin\invoicer.exe --json - -o none samples\invoices
```
