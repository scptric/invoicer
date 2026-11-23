// Package csvio reads, formats, sorts, deduplicates, merges, and writes
// accounting CSV files matching the workspace CSV Format.csv specification.
package csvio

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"invoicer/internal/model"
)

// Header defines the exact accounting CSV column names and ordering.
var Header = []string{"Symbol", "Side", "Qty", "Fill Price", "Commission", "Closing Time"}

const timeLayout = "2006-01-02 15:04:05"

// FormatNumber rounds v to maxDecimals places and strips trailing fractional zeros
// and decimal points (e.g., 190.00 -> "190", 1.90 -> "1.9", 3.80 -> "3.8").
func FormatNumber(v float64, maxDecimals int) string {
	pow := math.Pow10(maxDecimals)
	rounded := math.Round(v*pow) / pow
	if rounded == 0 {
		return "0"
	}
	s := strconv.FormatFloat(rounded, 'f', maxDecimals, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func formatOptNumber(p *float64, maxDecimals int) string {
	if p == nil {
		return ""
	}
	return FormatNumber(*p, maxDecimals)
}

// RecordRow converts a CSVRecord into its 6 serialized CSV column strings.
func RecordRow(r model.CSVRecord) []string {
	return []string{
		r.Symbol,
		string(r.Side),
		FormatNumber(r.Qty, 4),
		formatOptNumber(r.FillPrice, 4),
		formatOptNumber(r.Commission, 2),
		r.ClosingTime.Format(timeLayout),
	}
}

// ReadCSV reads and parses an existing accounting CSV file.
func ReadCSV(path string) ([]model.CSVRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read csv %q: %w", path, err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = len(Header)

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv %q: %w", path, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	// Validate header
	for i, want := range Header {
		if strings.TrimSpace(rows[0][i]) != want {
			return nil, fmt.Errorf("csv %q header column %d = %q, want %q", path, i, rows[0][i], want)
		}
	}

	out := make([]model.CSVRecord, 0, len(rows)-1)
	for rowIdx, row := range rows[1:] {
		qty, err := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
		if err != nil {
			return nil, fmt.Errorf("csv %q line %d invalid Qty %q: %w", path, rowIdx+2, row[2], err)
		}
		var fpPtr, commPtr *float64
		if s := strings.TrimSpace(row[3]); s != "" {
			fp, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("csv %q line %d invalid Fill Price %q: %w", path, rowIdx+2, row[3], err)
			}
			fpPtr = &fp
		}
		if s := strings.TrimSpace(row[4]); s != "" {
			c, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("csv %q line %d invalid Commission %q: %w", path, rowIdx+2, row[4], err)
			}
			commPtr = &c
		}
		ct, err := time.Parse(timeLayout, strings.TrimSpace(row[5]))
		if err != nil {
			return nil, fmt.Errorf("csv %q line %d invalid Closing Time %q: %w", path, rowIdx+2, row[5], err)
		}
		out = append(out, model.CSVRecord{
			Symbol:      strings.TrimSpace(row[0]),
			Side:        model.Side(strings.TrimSpace(row[1])),
			Qty:         qty,
			FillPrice:   fpPtr,
			Commission:  commPtr,
			ClosingTime: ct,
		})
	}
	return out, nil
}

// WriteCSV writes records in UTF-8 LF CSV format matching CSV Format.csv.
func WriteCSV(w io.Writer, records []model.CSVRecord) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	if err := cw.Write(Header); err != nil {
		return err
	}
	for _, rec := range records {
		if err := cw.Write(RecordRow(rec)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteCSVFile atomically writes records to path.
func WriteCSVFile(path string, records []model.CSVRecord) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %q: %w", dir, err)
		}
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, records); err != nil {
		return err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write csv %q: %w", path, err)
	}
	return nil
}

func compositeKey(r model.CSVRecord) string {
	return strings.Join(RecordRow(r), "|")
}

// MergeAndDeduplicate deduplicates incoming records (both internally by
// (Broker, TransactionNo) and against existing rows by occurrence-counted
// composite key), returning the merged slice and the number of skipped duplicates.
func MergeAndDeduplicate(existing []model.CSVRecord, incoming []model.CSVRecord) (merged []model.CSVRecord, skipped int) {
	seenTx := make(map[string]struct{})
	uniqueIncoming := make([]model.CSVRecord, 0, len(incoming))
	for _, rec := range incoming {
		if rec.TransactionNo != "" {
			txKey := string(rec.Broker) + "|" + rec.TransactionNo
			if _, dup := seenTx[txKey]; dup {
				skipped++
				continue
			}
			seenTx[txKey] = struct{}{}
		}
		uniqueIncoming = append(uniqueIncoming, rec)
	}

	existingCounts := make(map[string]int, len(existing))
	for _, rec := range existing {
		existingCounts[compositeKey(rec)]++
	}

	merged = make([]model.CSVRecord, len(existing), len(existing)+len(uniqueIncoming))
	copy(merged, existing)

	for _, rec := range uniqueIncoming {
		k := compositeKey(rec)
		if existingCounts[k] > 0 {
			existingCounts[k]--
			skipped++
			continue
		}
		merged = append(merged, rec)
	}
	return merged, skipped
}

func recordGroup(r model.CSVRecord) int {
	if r.Symbol == "$CASH" {
		return 1
	}
	if r.Side == model.SideBuy || r.Side == model.SideSell {
		return 0
	}
	return 2
}

// SortRecords orders records according to order ("desc", "asc", or "none")
// preserving the accounting grouping of trades, $CASH rows, and Dividends.
func SortRecords(records []model.CSVRecord, order string) {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "", "desc":
		sort.SliceStable(records, func(i, j int) bool {
			gi, gj := recordGroup(records[i]), recordGroup(records[j])
			if gi != gj {
				return gi < gj
			}
			if !records[i].ClosingTime.Equal(records[j].ClosingTime) {
				return records[i].ClosingTime.After(records[j].ClosingTime)
			}
			return false
		})
	case "asc":
		sort.SliceStable(records, func(i, j int) bool {
			gi, gj := recordGroup(records[i]), recordGroup(records[j])
			if gi != gj {
				return gi < gj
			}
			if !records[i].ClosingTime.Equal(records[j].ClosingTime) {
				return records[i].ClosingTime.Before(records[j].ClosingTime)
			}
			return false
		})
	case "none":
		// preserve original order
	}
}
