package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"invoicer/internal/csvio"
	"invoicer/internal/engine"
)

var version = "0.0.7"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var (
		outPath      string
		mergePath    string
		granularity  string
		symbolsPath  string
		exchange     string
		defaultTime  string
		sortOrder    string
		recursive    bool
		password     string
		onlineLookup bool
		jsonPath     string
		strict       bool
		showVersion  bool
		verbose      bool
	)

	fs := flag.NewFlagSet("invoicer", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&outPath, "o", "output/transactions.csv", "output CSV file path ('-' for stdout, 'none' to skip)")
	fs.StringVar(&outPath, "out", "output/transactions.csv", "output CSV file path ('-' for stdout, 'none' to skip)")
	fs.StringVar(&mergePath, "m", "", "existing accounting CSV file to merge and deduplicate against")
	fs.StringVar(&mergePath, "merge", "", "existing accounting CSV file to merge and deduplicate against")
	fs.StringVar(&granularity, "g", "fill", "output row granularity: 'fill' or 'order'")
	fs.StringVar(&granularity, "granularity", "fill", "output row granularity: 'fill' or 'order'")
	fs.StringVar(&symbolsPath, "s", "", "path to custom symbols.json ISIN/name->ticker overrides")
	fs.StringVar(&symbolsPath, "symbols", "", "path to custom symbols.json ISIN/name->ticker overrides")
	fs.StringVar(&exchange, "exchange", "EGX", "exchange prefix for resolved symbols")
	fs.StringVar(&defaultTime, "default-time", "00:00:00", "default HH:MM:SS time when PDF only specifies a date")
	fs.StringVar(&sortOrder, "sort", "desc", "output sort order: 'desc', 'asc', or 'none'")
	fs.BoolVar(&recursive, "recursive", false, "recursively scan input directories for PDFs")
	fs.StringVar(&password, "password", "", "PDF password for encrypted invoices")
	fs.BoolVar(&onlineLookup, "online-lookup", false, "query OpenFIGI for ISINs missing from built-in/custom maps")
	fs.StringVar(&jsonPath, "json", "", "write detailed JSON audit & records report to file ('-' for stdout)")
	fs.BoolVar(&strict, "strict", false, "exit with non-zero status on any reconciliation or symbol warning")
	fs.BoolVar(&showVersion, "version", false, "print invoicer version and exit")
	fs.BoolVar(&verbose, "v", false, "print per-page extraction and reconciliation details")
	fs.BoolVar(&verbose, "verbose", false, "print per-page extraction and reconciliation details")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: invoicer [flags] <pdf-or-dir>...\n\n")
		fmt.Fprintf(stderr, "Smart PDF invoice extraction engine producing accounting-ready CSVs.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if showVersion {
		fmt.Fprintf(stdout, "invoicer v%s\n", strings.TrimPrefix(version, "v"))
		return 0
	}

	outTrim := strings.TrimSpace(outPath)
	jsonTrim := strings.TrimSpace(jsonPath)
	if outTrim == "-" && jsonTrim == "-" {
		fmt.Fprintf(stderr, "error: cannot write both CSV (-o -) and JSON (--json -) to stdout at the same time\n")
		return 2
	}

	inputs := fs.Args()
	if len(inputs) == 0 {
		inputs = []string{"."}
	}

	cfg := engine.Config{
		Granularity:    granularity,
		ExchangePrefix: exchange,
		DefaultTime:    defaultTime,
		SortOrder:      sortOrder,
		SymbolsPath:    symbolsPath,
		OnlineLookup:   onlineLookup,
		Password:       password,
		Recursive:      recursive,
		MergePath:      mergePath,
		Strict:         strict,
	}

	res, err := engine.Run(context.Background(), inputs, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	logOut := stdout
	if outTrim == "-" || jsonTrim == "-" {
		logOut = stderr
	}

	skipCSV := outTrim == "" || strings.EqualFold(outTrim, "none")
	if !skipCSV {
		if outTrim == "-" {
			if err := csvio.WriteCSV(stdout, res.Records); err != nil {
				fmt.Fprintf(stderr, "error writing csv to stdout: %v\n", err)
				return 1
			}
		} else {
			if err := csvio.WriteCSVFile(outTrim, res.Records); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		}
	}

	if jsonTrim != "" {
		data, err := json.MarshalIndent(res.Report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "error encoding audit json: %v\n", err)
			return 1
		}
		payload := append(data, '\n')
		if jsonTrim == "-" {
			if _, err := stdout.Write(payload); err != nil {
				fmt.Fprintf(stderr, "error writing json to stdout: %v\n", err)
				return 1
			}
		} else {
			if dir := filepath.Dir(jsonTrim); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					fmt.Fprintf(stderr, "error creating json directory: %v\n", err)
					return 1
				}
			}
			if err := os.WriteFile(jsonTrim, payload, 0o644); err != nil {
				fmt.Fprintf(stderr, "error writing json report %q: %v\n", jsonTrim, err)
				return 1
			}
		}
	}

	if verbose {
		for _, inv := range res.Report.Invoices {
			status := "OK"
			if !inv.Reconciliation.Passed {
				status = "WARN"
			}
			fmt.Fprintf(logOut, "  [%s] %s p.%d (%s): %s %s qty=%s fees=%s total=%s\n",
				status, filepath.Base(inv.SourceFile), inv.PageNumber, inv.Broker,
				inv.ResolvedSymbol, inv.Side,
				csvio.FormatNumber(inv.TotalQuantity, 4),
				csvio.FormatNumber(inv.TotalFees, 2),
				csvio.FormatNumber(inv.GrandTotal, 2),
			)
		}
	}

	for _, w := range res.Report.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}

	fmt.Fprintf(logOut, "Processed %d file(s), %d page(s) [reconciled: %d passed, %d warned] -> %d CSV row(s)",
		res.Report.TotalFiles, res.Report.TotalPages,
		res.Report.ReconciledPassed, res.Report.ReconciledWarned,
		res.Report.TotalRowsOutput,
	)
	if res.Report.SkippedDuplicates > 0 {
		fmt.Fprintf(logOut, " (%d duplicate(s) skipped)", res.Report.SkippedDuplicates)
	}
	if !skipCSV && outTrim != "-" {
		fmt.Fprintf(logOut, " written to %s", outTrim)
	}
	fmt.Fprintln(logOut)

	if strict && (res.Report.ReconciledWarned > 0 || len(res.Report.Warnings) > 0) {
		fmt.Fprintf(stderr, "error: strict mode failed due to reconciliation or extraction warnings\n")
		return 1
	}
	return 0
}
