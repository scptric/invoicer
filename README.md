# invoicer

[![Releases](https://img.shields.io/badge/Releases-v0.0.7-18181b)](https://github.com/scptric/invoicer/releases)
[![Packages](https://img.shields.io/badge/Packages-ghcr.io%2Fscptric%2Finvoicer-18181b)](https://github.com/scptric/invoicer/pkgs/container/invoicer)
[![License](https://img.shields.io/badge/License-Apache_2.0-18181b)](LICENSE)

Pure-Go, zero-CGO smart PDF invoice extraction engine by **Scptric** that converts brokerage PDF E-Invoices (Telda Securities, Thndr Securities Brokerage, and generic trade confirmations) into accounting-ready CSV files matching [samples/reference/CSV Format.csv](samples/reference/CSV%20Format.csv) or structured JSON for web apps.

## Documentation

- **[How to Run (`docs/HOW_TO_RUN.md`)](docs/HOW_TO_RUN.md)** — Simple step-by-step guide, copy-paste commands, flags, custom symbols, and building from source.
- **[Project Overview & Next.js Integration (`docs/PROJECT_AND_NEXTJS_INTEGRATION.md`)](docs/PROJECT_AND_NEXTJS_INTEGRATION.md)** — How the engine works and how to plug it into a Next.js upload flow backed by a `transactions` database table.
- **[Changelog (`CHANGELOG.md`)](CHANGELOG.md)** — Release history and notes (`v0.0.7`).

## Releases & Packages

- **[Releases](https://github.com/scptric/invoicer/releases)** — Download prebuilt `invoicer` CLI archives (`v0.0.7`) for Windows (`amd64`), Linux (`amd64`, `arm64`), and macOS (`amd64`, `arm64`), including SHA-256 checksums and `config/symbols.json`.
- **[Packages](https://github.com/scptric/invoicer/pkgs/container/invoicer)** — Pull and run the multi-stage container image hosting both the compiled Go engine and the Next.js workbench:

```bash
docker run --rm -p 3000:3000 ghcr.io/scptric/invoicer:0.0.7
```

## Project Structure

```text
invoicer/
├── .github/workflows/     # GitHub Releases (binaries) & Packages (GHCR) CI
├── bin/                   # Local compiled executable (gitignored)
├── cmd/invoicer/          # CLI entrypoint (main.go)
├── config/                # EGX ISIN & company-name -> ticker registry (symbols.json)
├── docs/                  # User & Next.js integration guides
├── internal/
│   ├── csvio/             # Accounting CSV reader, formatter, merger, deduplicator
│   ├── engine/            # Concurrent PDF discovery and extraction pipeline
│   ├── model/             # Domain models (InvoicePage, CSVRecord, AuditReport)
│   ├── parser/            # Multi-broker parsers (Telda, Thndr, Generic fallback)
│   ├── pdfextract/        # Pure-Go layout-aware PDF parser built on pdfcpu
│   ├── reconcile/         # Math verification & exact integer-cent fee allocation
│   └── symbols/           # EGX ISIN & company-name -> ticker resolver
├── output/                # Default output folder (gitignored CSV/JSON exports)
├── samples/
│   └── reference/         # Target synthetic CSV Format.csv specification
├── web/                   # Next.js App Router client UI & portable workbench
│   └── src/
│       ├── app/api/invoicer/   # Node.js Route Handlers calling invoicer binary
│       ├── features/invoicer/  # Portable drop-in UI module (InvoicerWorkbench)
│       └── lib/                # Go binary server runner helper
├── Dockerfile             # Multi-stage Go + Next.js production container image
└── LICENSE                # Apache License 2.0 (Copyright 2026 Scptric)
```

## Quick Start

### CLI Usage

```powershell
# Build binary
go build -o bin\invoicer.exe ./cmd/invoicer

# Check version
.\bin\invoicer.exe --version

# Extract all PDFs in a folder into output\transactions.csv
.\bin\invoicer.exe -v C:\path\to\invoices

# Merge into an existing accounting CSV without duplicates
.\bin\invoicer.exe -v --merge "samples\reference\CSV Format.csv" -o output\merged.csv C:\path\to\invoices

# Emit JSON (for Next.js / APIs) without writing a CSV file
.\bin\invoicer.exe --json - -o none C:\path\to\invoices
```

### Web UI Workbench (`web/`)

```powershell
cd web
npm install
npm run dev
```

Open `http://localhost:3000` in your browser. To embed the UI into another Next.js App Router project, copy `web/src/features/invoicer/`, `web/src/lib/invoicer-server.ts`, and `web/src/app/api/invoicer/`, then render `<InvoicerWorkbench />`.

## License

Copyright 2026 Scptric. Licensed under the [Apache License, Version 2.0](LICENSE).
