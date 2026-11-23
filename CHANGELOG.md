# Changelog

All notable changes to **invoicer** are documented in this file.

## [0.0.7] - 2026-11-24

Initial open-source release by **Scptric** under the Apache License 2.0.

### Added
- **Pure-Go PDF Extraction Engine (`cmd/invoicer`, `internal/*`)**: Layout-aware PDF parser with CTM coordinate recovery, `/ToUnicode` CMap decoding, and zero CGO dependencies.
- **Broker Parsers**: Built-in support for Telda Securities and Thndr Securities Brokerage E-Invoices, with generic trade confirmation fallback.
- **EGX Symbol Resolution (`config/symbols.json`)**: Offline lookup covering 298 EGX ISINs and company names, plus optional OpenFIGI fallback (`--online-lookup`).
- **Math Reconciliation & Cent Allocation**: Exact integer-cent fee allocation across partial fills and automated check verification (`Total Value ± Total Fees = Grand Total`).
- **CLI Version Flag**: Added `--version` (`invoicer v0.0.7`) injected at build time via `-ldflags`.
- **Next.js App Router Workbench (`web/`)**: Upload UI, reconciliation strip, execution/invoice row switcher, interactive CSV Viewer (Side filter, Broker filter, search), RFC 4180 CSV preview, and portable `src/features/invoicer/` integration module.
- **Releases & Packages Automation**: Added multi-stage `Dockerfile` and `.github/workflows/release.yml` to publish multi-platform binaries to GitHub Releases and container images to GitHub Packages (`ghcr.io/scptric/invoicer`).
