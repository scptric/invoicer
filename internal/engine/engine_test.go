package engine

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invoicer/internal/csvio"
	"invoicer/internal/model"
)

type pdfTextItem struct {
	x    float64
	y    float64
	text string
}

func writeSyntheticInvoicePDF(t *testing.T, outPath string, pages [][]pdfTextItem) {
	t.Helper()

	cmapText := `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0020> <007E> <0020>
endbfrange
endcmap
end`

	deflate := func(b []byte) []byte {
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		_, _ = zw.Write(b)
		_ = zw.Close()
		return buf.Bytes()
	}

	encodeHex := func(s string) string {
		b := make([]byte, 0, len(s)*2)
		for i := 0; i < len(s); i++ {
			b = append(b, 0x00, s[i])
		}
		return "<" + strings.ToUpper(hex.EncodeToString(b)) + ">"
	}

	cmapComp := deflate([]byte(cmapText))
	objects := make(map[int][]byte)
	objects[3] = []byte(`<< /Type /Font /Subtype /Type0 /BaseFont /TestSans /Encoding /Identity-H /DescendantFonts [4 0 R] /ToUnicode 6 0 R >>`)
	objects[4] = []byte(`<< /Type /Font /Subtype /CIDFontType2 /BaseFont /TestSans /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 550 /W [32 [280] 48 57 550] /FontDescriptor 5 0 R >>`)
	objects[5] = []byte(`<< /Type /FontDescriptor /FontName /TestSans /Flags 4 /FontBBox [0 -200 1000 900] /ItalicAngle 0 /Ascent 900 /Descent -200 /CapHeight 700 /StemV 80 >>`)
	objects[6] = append(
		[]byte(fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n", len(cmapComp))),
		append(cmapComp, []byte("\nendstream")...)...,
	)

	var kids []string
	nextID := 7
	for _, items := range pages {
		pageID := nextID
		streamID := nextID + 1
		nextID += 2
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		objects[pageID] = []byte(fmt.Sprintf(
			`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>`,
			streamID,
		))

		var sb strings.Builder
		sb.WriteString("q\n1 0 0 -1 0 792 cm\n")
		for _, it := range items {
			sb.WriteString(fmt.Sprintf("BT /F1 9 Tf 1 0 0 -1 %.2f %.2f Tm %s Tj ET\n", it.x, it.y, encodeHex(it.text)))
		}
		sb.WriteString("Q\n")
		comp := deflate([]byte(sb.String()))
		objects[streamID] = append(
			[]byte(fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n", len(comp))),
			append(comp, []byte("\nendstream")...)...,
		)
	}

	objects[1] = []byte(`<< /Type /Catalog /Pages 2 0 R >>`)
	objects[2] = []byte(fmt.Sprintf(`<< /Type /Pages /Kids [%s] /Count %d >>`, strings.Join(kids, " "), len(kids)))

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, nextID)
	for id := 1; id < nextID; id++ {
		offsets[id] = pdf.Len()
		pdf.WriteString(fmt.Sprintf("%d 0 obj\n", id))
		pdf.Write(objects[id])
		pdf.WriteString("\nendobj\n")
	}
	xrefOffset := pdf.Len()
	pdf.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", nextID))
	for id := 1; id < nextID; id++ {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[id]))
	}
	pdf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", nextID, xrefOffset))

	if err := os.WriteFile(outPath, pdf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEngineEndToEndSyntheticPDFs(t *testing.T) {
	tmpDir := t.TempDir()
	teldaPDF := filepath.Join(tmpDir, "telda_invoice.pdf")
	thndrPDF := filepath.Join(tmpDir, "thndr_invoice.pdf")
	symbolsPath := filepath.Join("..", "..", "config", "symbols.json")
	sampleCSV := filepath.Join("..", "..", "samples", "reference", "CSV Format.csv")

	teldaPage := []pdfTextItem{
		{60, 140, "JANE DOE"},
		{60, 170, "2026-11-23"},
		{60, 210, "Security name:"}, {200, 210, "Symbol code:"}, {350, 210, "Transaction type:"}, {460, 210, "Average cost:"},
		{60, 235, "Egypt Aluminum"}, {200, 235, "EGAL"}, {350, 235, "Sell"}, {460, 235, "EGP 300.63"},
		{60, 275, "Transaction NO:"}, {200, 275, "Quantity"}, {350, 275, "Price"}, {460, 275, "Value"},
		{60, 305, "TXN000000001"}, {200, 305, "1"}, {350, 305, "EGP 301.56"}, {460, 305, "EGP 301.56"},
		{60, 330, "TXN000000002"}, {200, 330, "2"}, {350, 330, "EGP 301.51"}, {460, 330, "EGP 603.02"},
		{200, 380, "Fees"}, {460, 380, "Amount"},
		{200, 405, "Telda Fees"}, {460, 405, "EGP 2.00"},
		{200, 430, "EFSA Fees"}, {460, 430, "EGP 2.00"},
		{200, 455, "Other Fees"}, {460, 455, "EGP 0.68"},
		{200, 480, "Telda Incentive"}, {460, 480, "EGP -2.00"},
		{200, 505, "Total Fees"}, {460, 505, "EGP 2.68"},
		{200, 540, "GRAND TOTAL"}, {460, 540, "EGP 901.90"},
		{420, 720, "www.telda.app"},
	}

	thndrPage := []pdfTextItem{
		{78, 140, "Jane Doe"}, {360, 140, "Custodian: Thndr Technology Holding"},
		{78, 160, "23/11/2026"},
		{80, 195, "Security Name"}, {220, 195, "Symbol Code"}, {330, 195, "Transaction Type"}, {470, 195, "Average Cost"},
		{80, 212, "Bonyan for"},
		{80, 224, "Development and"}, {220, 224, "EGS656M1C010"}, {350, 224, "Sell"}, {480, 224, "4.11 EGP"},
		{80, 236, "Trade"},
		{80, 270, "Transaction No."}, {225, 270, "Quantity"}, {350, 270, "Price"}, {490, 270, "Value"},
		{80, 295, "TXN000000101"}, {230, 295, "190.00"}, {345, 295, "4.13 EGP"}, {475, 295, "784.70 EGP"},
		{80, 315, "TXN000000102"}, {230, 315, "110.00"}, {345, 315, "4.13 EGP"}, {475, 315, "454.30 EGP"},
		{215, 340, "Total Quantity"}, {335, 340, "Average Price"}, {480, 340, "Total Cost"},
		{230, 360, "300.00"}, {345, 360, "4.13 EGP"}, {465, 360, "1,239.00 EGP"},
		{350, 400, "Fees"}, {490, 400, "Amount"},
		{325, 425, "FRA Services"}, {485, 425, "2.00 EGP"},
		{325, 450, "Trading Damgha"}, {485, 450, "0.62 EGP"},
		{325, 475, "Brokerage & Custody Fees"}, {485, 475, "3.55 EGP"},
		{335, 505, "Total Fees"}, {485, 505, "6.17 EGP"},
		{330, 535, "Grand Total"}, {455, 535, "1,232.83 EGP"},
	}

	writeSyntheticInvoicePDF(t, teldaPDF, [][]pdfTextItem{teldaPage})
	writeSyntheticInvoicePDF(t, thndrPDF, [][]pdfTextItem{thndrPage})

	// 1. Fill granularity + JSON records validation
	resFill, err := Run(context.Background(), []string{teldaPDF, thndrPDF}, Config{
		Granularity: "fill",
		SortOrder:   "none",
		SymbolsPath: symbolsPath,
	})
	if err != nil {
		t.Fatalf("Run(fill) failed: %v", err)
	}
	if resFill.Report.TotalFiles != 2 || resFill.Report.TotalPages != 2 {
		t.Fatalf("counts mismatch: %+v", resFill.Report)
	}
	if resFill.Report.ReconciledPassed != 2 || resFill.Report.ReconciledWarned != 0 || len(resFill.Report.Warnings) != 0 {
		t.Fatalf("expected 2/2 reconciled without warnings, got passed=%d warned=%d warnings=%v",
			resFill.Report.ReconciledPassed, resFill.Report.ReconciledWarned, resFill.Report.Warnings)
	}
	if resFill.Report.Brokers[model.BrokerTelda] != 1 || resFill.Report.Brokers[model.BrokerThndr] != 1 {
		t.Fatalf("unexpected broker counts: %+v", resFill.Report.Brokers)
	}
	if len(resFill.Records) != 4 || len(resFill.Report.Records) != 4 {
		t.Fatalf("expected 4 fill rows, got records=%d report.records=%d", len(resFill.Records), len(resFill.Report.Records))
	}

	var buf bytes.Buffer
	if err := csvio.WriteCSV(&buf, resFill.Records); err != nil {
		t.Fatal(err)
	}
	wantFill := strings.Join([]string{
		"Symbol,Side,Qty,Fill Price,Commission,Closing Time",
		"EGX:EGAL,Sell,1,301.56,1.23,2026-11-23 00:00:00",
		"EGX:EGAL,Sell,2,301.51,1.45,2026-11-23 00:00:00",
		"EGX:BONY,Sell,190,4.13,3.64,2026-11-23 00:00:00",
		"EGX:BONY,Sell,110,4.13,2.53,2026-11-23 00:00:00",
	}, "\n") + "\n"
	if buf.String() != wantFill {
		t.Fatalf("fill CSV mismatch:\nGOT:\n%s\nWANT:\n%s", buf.String(), wantFill)
	}

	rawReportJSON, err := json.Marshal(resFill.Report)
	if err != nil {
		t.Fatalf("json.Marshal(Report): %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rawReportJSON, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(Report): %v", err)
	}
	jsonRecs, ok := decoded["records"].([]any)
	if !ok || len(jsonRecs) != 4 {
		t.Fatalf("expected JSON records array of len 4, got %T", decoded["records"])
	}

	// 2. Order granularity
	resOrder, err := Run(context.Background(), []string{teldaPDF, thndrPDF}, Config{
		Granularity: "order",
		SortOrder:   "none",
		SymbolsPath: symbolsPath,
	})
	if err != nil {
		t.Fatalf("Run(order) failed: %v", err)
	}
	if len(resOrder.Records) != 2 {
		t.Fatalf("expected 2 order rows, got %d", len(resOrder.Records))
	}

	// 3. Idempotent merge into copy of CSV Format.csv
	mergedPath := filepath.Join(tmpDir, "merged.csv")
	origBytes, err := os.ReadFile(sampleCSV)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mergedPath, origBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	resMerge1, err := Run(context.Background(), []string{teldaPDF, thndrPDF}, Config{
		Granularity: "fill",
		MergePath:   mergedPath,
		SortOrder:   "desc",
		SymbolsPath: symbolsPath,
	})
	if err != nil {
		t.Fatalf("Run(merge1) failed: %v", err)
	}
	if len(resMerge1.Records) != 13 || resMerge1.Report.SkippedDuplicates != 0 {
		t.Fatalf("merge1 expected 13 rows & 0 skipped, got %d & %d", len(resMerge1.Records), resMerge1.Report.SkippedDuplicates)
	}
	if err := csvio.WriteCSVFile(mergedPath, resMerge1.Records); err != nil {
		t.Fatal(err)
	}

	resMerge2, err := Run(context.Background(), []string{teldaPDF, thndrPDF}, Config{
		Granularity: "fill",
		MergePath:   mergedPath,
		SortOrder:   "desc",
		SymbolsPath: symbolsPath,
	})
	if err != nil {
		t.Fatalf("Run(merge2) failed: %v", err)
	}
	if len(resMerge2.Records) != 13 || resMerge2.Report.SkippedDuplicates != 4 {
		t.Fatalf("merge2 expected 13 rows & 4 skipped duplicates, got %d & %d", len(resMerge2.Records), resMerge2.Report.SkippedDuplicates)
	}
}
