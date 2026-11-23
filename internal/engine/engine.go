// Package engine orchestrates concurrent PDF discovery, layout extraction,
// multi-broker parsing, symbol resolution, mathematical reconciliation, and
// accounting CSV generation/merging.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"invoicer/internal/csvio"
	"invoicer/internal/model"
	"invoicer/internal/parser"
	"invoicer/internal/pdfextract"
	"invoicer/internal/reconcile"
	"invoicer/internal/symbols"
)

// Config controls extraction, resolution, merging, and formatting behavior.
type Config struct {
	Granularity    string
	ExchangePrefix string
	DefaultTime    string
	SortOrder      string
	SymbolsPath    string
	OnlineLookup   bool
	Password       string
	Recursive      bool
	MergePath      string
	Strict         bool
}

// AuditReport summarizes extraction outcomes, reconciliation checks, and warnings.
type AuditReport struct {
	TotalFiles         int                  `json:"total_files"`
	TotalPages         int                  `json:"total_pages"`
	Brokers            map[model.Broker]int `json:"brokers"`
	ReconciledPassed   int                  `json:"reconciled_passed"`
	ReconciledWarned   int                  `json:"reconciled_warned"`
	TotalRowsExtracted int                  `json:"total_rows_extracted"`
	SkippedDuplicates  int                  `json:"skipped_duplicates"`
	TotalRowsOutput    int                  `json:"total_rows_output"`
	Warnings           []string             `json:"warnings,omitempty"`
	Records            []model.CSVRecord    `json:"records"`
	Invoices           []*model.InvoicePage `json:"invoices"`
}

// Result contains the final accounting CSV records and extraction audit report.
type Result struct {
	Records []model.CSVRecord
	Report  AuditReport
}

// DiscoverPDFs expands any mix of file paths and directory paths into a sorted,
// deduplicated list of PDF file paths.
func DiscoverPDFs(inputs []string, recursive bool) ([]string, error) {
	if len(inputs) == 0 {
		inputs = []string{"."}
	}

	seen := make(map[string]struct{})
	var files []string

	addFile := func(p string) {
		cleaned := filepath.Clean(p)
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			abs = cleaned
		}
		key := abs
		if runtime.GOOS == "windows" {
			key = strings.ToLower(abs)
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		files = append(files, cleaned)
	}

	for _, in := range inputs {
		info, err := os.Stat(in)
		if err != nil {
			return nil, fmt.Errorf("stat input %q: %w", in, err)
		}
		if !info.IsDir() {
			if strings.EqualFold(filepath.Ext(in), ".pdf") {
				addFile(in)
			} else {
				return nil, fmt.Errorf("input file %q is not a .pdf file", in)
			}
			continue
		}

		if recursive {
			err = filepath.WalkDir(in, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".pdf") {
					addFile(path)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk directory %q: %w", in, err)
			}
		} else {
			entries, err := os.ReadDir(in)
			if err != nil {
				return nil, fmt.Errorf("read directory %q: %w", in, err)
			}
			for _, e := range entries {
				if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
					addFile(filepath.Join(in, e.Name()))
				}
			}
		}
	}

	sort.Strings(files)
	return files, nil
}

type fileOutcome struct {
	invoices []*model.InvoicePage
	records  []model.CSVRecord
	err      error
}

// Run discovers PDFs, extracts and reconciles all pages concurrently, resolves
// symbols, deduplicates/merges with any existing CSV, and sorts the output.
func Run(ctx context.Context, inputs []string, cfg Config) (*Result, error) {
	if cfg.Granularity == "" {
		cfg.Granularity = "fill"
	}
	if cfg.DefaultTime == "" {
		cfg.DefaultTime = "00:00:00"
	}
	if cfg.SortOrder == "" {
		cfg.SortOrder = "desc"
	}
	if cfg.ExchangePrefix == "" {
		cfg.ExchangePrefix = "EGX"
	}

	files, err := DiscoverPDFs(inputs, cfg.Recursive)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no PDF files found in inputs")
	}

	resolver, err := symbols.NewResolver(symbols.Config{
		ExchangePrefix: cfg.ExchangePrefix,
		SymbolsPath:    cfg.SymbolsPath,
		OnlineLookup:   cfg.OnlineLookup,
	})
	if err != nil {
		return nil, err
	}
	registry := parser.NewRegistry()

	outcomes := make([]fileOutcome, len(files))
	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan int, len(files))
	for i := range files {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				select {
				case <-ctx.Done():
					outcomes[idx] = fileOutcome{err: ctx.Err()}
					continue
				default:
				}
				outcomes[idx] = processFile(files[idx], cfg, registry, resolver)
			}
		}()
	}
	wg.Wait()

	report := AuditReport{
		TotalFiles: len(files),
		Brokers:    make(map[model.Broker]int),
	}
	var incoming []model.CSVRecord

	for _, out := range outcomes {
		if out.err != nil {
			return nil, out.err
		}
		for _, inv := range out.invoices {
			report.TotalPages++
			report.Brokers[inv.Broker]++
			if inv.Reconciliation.Passed {
				report.ReconciledPassed++
			} else {
				report.ReconciledWarned++
			}
			report.Warnings = append(report.Warnings, inv.Warnings...)
			report.Invoices = append(report.Invoices, inv)
		}
		incoming = append(incoming, out.records...)
	}
	report.TotalRowsExtracted = len(incoming)

	var existing []model.CSVRecord
	if strings.TrimSpace(cfg.MergePath) != "" {
		ex, err := csvio.ReadCSV(cfg.MergePath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		} else {
			existing = ex
		}
	}

	merged, skipped := csvio.MergeAndDeduplicate(existing, incoming)
	csvio.SortRecords(merged, cfg.SortOrder)

	report.SkippedDuplicates = skipped
	report.TotalRowsOutput = len(merged)
	report.Records = merged

	return &Result{
		Records: merged,
		Report:  report,
	}, nil
}

func processFile(path string, cfg Config, registry *parser.Registry, resolver *symbols.Resolver) fileOutcome {
	doc, err := pdfextract.ExtractFile(path, cfg.Password)
	if err != nil {
		return fileOutcome{err: err}
	}

	var out fileOutcome
	for _, page := range doc.Pages {
		inv, err := registry.ParsePage(page, path)
		if err != nil {
			return fileOutcome{err: err}
		}
		sym, warn := resolver.Resolve(inv.RawSymbolCode, inv.SecurityName)
		inv.ResolvedSymbol = sym
		if warn != "" {
			inv.Warnings = append(inv.Warnings, warn)
		}
		reconcile.Reconcile(inv)

		recs, err := reconcile.ToCSVRecords(inv, cfg.Granularity, cfg.DefaultTime)
		if err != nil {
			return fileOutcome{err: err}
		}

		out.invoices = append(out.invoices, inv)
		out.records = append(out.records, recs...)
	}
	return out
}
