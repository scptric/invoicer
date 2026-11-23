package csvio

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"invoicer/internal/model"
)

func floatPtr(v float64) *float64 {
	return &v
}

func TestRoundTripCSVFormatSample(t *testing.T) {
	samplePath := filepath.Join("..", "..", "samples", "reference", "CSV Format.csv")
	origBytes, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatalf("read CSV Format.csv: %v", err)
	}

	records, err := ReadCSV(samplePath)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(records) != 9 {
		t.Fatalf("expected 9 records, got %d", len(records))
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, records); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	// Normalize trailing newline difference (sample file has no trailing \n)
	got := strings.TrimRight(buf.String(), "\n")
	want := strings.TrimRight(strings.ReplaceAll(string(origBytes), "\r\n", "\n"), "\n")
	if got != want {
		t.Fatalf("round-trip mismatch:\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		v   float64
		dec int
		out string
	}{
		{190.00, 4, "190"},
		{1.90, 4, "1.9"},
		{22.60, 4, "22.6"},
		{302.00, 4, "302"},
		{3.80, 2, "3.8"},
		{1.15, 2, "1.15"},
		{2.68, 2, "2.68"},
		{2.00, 2, "2"},
		{-0.0, 2, "0"},
		{301.526666, 4, "301.5267"},
	}
	for _, tc := range tests {
		if got := FormatNumber(tc.v, tc.dec); got != tc.out {
			t.Errorf("FormatNumber(%v, %d) = %q, want %q", tc.v, tc.dec, got, tc.out)
		}
	}
}

func TestMergeDeduplicateAndSort(t *testing.T) {
	t1 := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 11, 12, 29, 36, 0, time.UTC)
	tCash := time.Date(2026, 11, 22, 12, 29, 56, 0, time.UTC)
	tDiv := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	existing := []model.CSVRecord{
		{Symbol: "EGX:AMES", Side: model.SideSell, Qty: 12, FillPrice: floatPtr(50.25), Commission: floatPtr(1.45), ClosingTime: t2},
		{Symbol: "$CASH", Side: model.SideTaxesAndFees, Qty: 2.4, ClosingTime: tCash},
		{Symbol: "EGX:EGAL", Side: model.SideDividend, Qty: 48, ClosingTime: tDiv},
	}

	incoming := []model.CSVRecord{
		{Symbol: "EGX:EGAL", Side: model.SideSell, Qty: 1, FillPrice: floatPtr(301.56), Commission: floatPtr(1.23), ClosingTime: t1, Broker: model.BrokerTelda, TransactionNo: "N1"},
		// Duplicate incoming TransactionNo (e.g., same PDF supplied twice)
		{Symbol: "EGX:EGAL", Side: model.SideSell, Qty: 1, FillPrice: floatPtr(301.56), Commission: floatPtr(1.23), ClosingTime: t1, Broker: model.BrokerTelda, TransactionNo: "N1"},
		// Already in existing
		{Symbol: "EGX:AMES", Side: model.SideSell, Qty: 12, FillPrice: floatPtr(50.25), Commission: floatPtr(1.45), ClosingTime: t2, Broker: model.BrokerTelda, TransactionNo: "N2"},
	}

	merged, skipped := MergeAndDeduplicate(existing, incoming)
	if skipped != 2 {
		t.Fatalf("skipped = %d, want 2", skipped)
	}
	if len(merged) != 4 {
		t.Fatalf("merged len = %d, want 4", len(merged))
	}

	SortRecords(merged, "desc")
	if merged[0].Symbol != "EGX:EGAL" || merged[0].Side != model.SideSell {
		t.Errorf("expected newest trade first, got %+v", merged[0])
	}
	if merged[1].Symbol != "EGX:AMES" {
		t.Errorf("expected older trade second, got %+v", merged[1])
	}
	if merged[2].Symbol != "$CASH" {
		t.Errorf("expected $CASH third, got %+v", merged[2])
	}
	if merged[3].Side != model.SideDividend {
		t.Errorf("expected Dividend last, got %+v", merged[3])
	}
}
