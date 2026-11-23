package pdfextract

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatrixMulAndApply(t *testing.T) {
	s := Matrix{A: 0.75, D: 0.75}
	tr := Translate(10, 20)
	combined := Mul(s, tr)

	x1, y1 := combined.Apply(40, 80)
	x2, y2 := tr.Apply(s.Apply(40, 80))

	if math.Abs(x1-x2) > 1e-9 || math.Abs(y1-y2) > 1e-9 {
		t.Fatalf("Mul(s, tr).Apply = (%v, %v), want (%v, %v)", x1, y1, x2, y2)
	}
	if math.Abs(x1-40) > 1e-9 || math.Abs(y1-80) > 1e-9 {
		t.Fatalf("expected (40, 80), got (%v, %v)", x1, y1)
	}
}

func TestParseCMapAndDecode(t *testing.T) {
	rawCMap := []byte(`
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
1 begincodespacerange
<0000> <ffff>
endcodespacerange
3 beginbfchar
<0003> <0020>
<0024> <0041>
<00FB> <00660069>
endbfchar
2 beginbfrange
<0013> <0015> <0030>
<0030> <0031> [<004D> <004E>]
endbfrange
endcmap
`)
	cm := ParseCMap(rawCMap)
	got, codes := cm.DecodeBytes([]byte{0x00, 0x24, 0x00, 0x03, 0x00, 0x13, 0x00, 0x15, 0x00, 0x30, 0x00, 0x31, 0x00, 0xFB})
	want := "A 02MNfi"
	if got != want {
		t.Fatalf("DecodeBytes = %q, want %q (codes=%v)", got, want, codes)
	}
}

func TestTokenizerLiteralHexAndArrays(t *testing.T) {
	stream := []byte(`% comment line
1 0 0 -1 50 100 Tm
/F1 12 Tf
[(Hello \(world\)) -350 <00410042>] TJ
(Line1\nLine2) Tj
`)
	toks := tokenizeContentStream(stream)
	if len(toks) < 12 {
		t.Fatalf("expected >= 12 tokens, got %d", len(toks))
	}
	var foundTJ bool
	for i, tk := range toks {
		if tk.kind == tokOperator && tk.str == "TJ" {
			foundTJ = true
			arr := toks[i-1]
			if arr.kind != tokArray || len(arr.arr) != 3 {
				t.Fatalf("unexpected TJ array: %+v", arr)
			}
			if string(arr.arr[0].raw) != "Hello (world)" {
				t.Fatalf("escaped parens = %q", string(arr.arr[0].raw))
			}
			if arr.arr[1].kind != tokNumber || arr.arr[1].num != -350 {
				t.Fatalf("unexpected TJ num: %+v", arr.arr[1])
			}
			if string(arr.arr[2].raw) != "\x00A\x00B" {
				t.Fatalf("hex string = %x", arr.arr[2].raw)
			}
		}
	}
	if !foundTJ {
		t.Fatal("TJ operator not found")
	}
}

func TestLayoutKerningAndRowClustering(t *testing.T) {
	spans := []TextSpan{
		{Text: "www", X: 400, Y: 758, EndX: 422, FontSize: 15},
		{Text: ".telda.app", X: 420.6, Y: 758, EndX: 480, FontSize: 15},
		{Text: "TXN000000001", X: 56.7, Y: 309.56, EndX: 150, FontSize: 9},
		{Text: "1", X: 205.2, Y: 310.08, EndX: 211, FontSize: 9},
		{Text: "EGP 301.56", X: 353.7, Y: 310.08, EndX: 405, FontSize: 9},
	}
	rows := buildVisualRows(spans)
	if len(rows) != 2 {
		t.Fatalf("expected 2 visual rows, got %d: %+v", len(rows), rows)
	}
	if len(rows[0].Cells) != 3 {
		t.Fatalf("expected 3 cells in table row, got %d: %+v", len(rows[0].Cells), rows[0].Cells)
	}
	if rows[0].Cells[0].Text != "TXN000000001" || rows[0].Cells[1].Text != "1" || rows[0].Cells[2].Text != "EGP 301.56" {
		t.Fatalf("unexpected row 0 cells: %+v", rows[0].Cells)
	}
	if len(rows[1].Cells) != 1 || rows[1].Cells[0].Text != "www.telda.app" {
		t.Fatalf("expected merged www.telda.app, got %+v", rows[1].Cells)
	}
}

func encodeASCIIAsCIDHex(s string) string {
	b := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		b = append(b, 0x00, s[i])
	}
	return "<" + strings.ToUpper(hex.EncodeToString(b)) + ">"
}

func deflateData(data []byte) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()
	return buf.Bytes()
}

func buildTestPDF(t *testing.T, pageContents []string) string {
	t.Helper()
	cmapText := `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfchar
<0090> <0045>
endbfchar
1 beginbfrange
<0020> <007E> <0020>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end`
	cmapCompressed := deflateData([]byte(cmapText))

	objects := make(map[int][]byte)
	// 1: Catalog, 2: Pages, 3: Font F1, 4: CIDFont, 5: FontDescriptor, 6: ToUnicode
	objects[3] = []byte(`<< /Type /Font /Subtype /Type0 /BaseFont /TestSans /Encoding /Identity-H /DescendantFonts [4 0 R] /ToUnicode 6 0 R >>`)
	objects[4] = []byte(`<< /Type /Font /Subtype /CIDFontType2 /BaseFont /TestSans /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 550 /W [32 [280] 48 57 550] /FontDescriptor 5 0 R >>`)
	objects[5] = []byte(`<< /Type /FontDescriptor /FontName /TestSans /Flags 4 /FontBBox [0 -200 1000 900] /ItalicAngle 0 /Ascent 900 /Descent -200 /CapHeight 700 /StemV 80 >>`)
	objects[6] = append(
		[]byte(fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n", len(cmapCompressed))),
		append(cmapCompressed, []byte("\nendstream")...)...,
	)

	var kids []string
	nextID := 7
	for _, streamBody := range pageContents {
		pageID := nextID
		streamID := nextID + 1
		nextID += 2
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		objects[pageID] = []byte(fmt.Sprintf(
			`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>`,
			streamID,
		))
		comp := deflateData([]byte(streamBody))
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

	outPath := filepath.Join(t.TempDir(), "synthetic.pdf")
	if err := os.WriteFile(outPath, pdf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return outPath
}

func TestExtractSyntheticPDFWithToUnicodeAndCTM(t *testing.T) {
	// Exercises FlateDecode streams, CTM flip+scale, TJ kerning tightening, bfchar + bfrange, and multi-page extraction.
	p1 := strings.Join([]string{
		"q",
		"0.75 0 0 -0.75 0 792 cm",
		"BT /F1 12 Tf 1 0 0 -1 80 120 Tm " + encodeASCIIAsCIDHex("Telda") + " Tj ET",
		"BT /F1 12 Tf 1 0 0 -1 80 160 Tm [" + encodeASCIIAsCIDHex("www") + " 91 " + encodeASCIIAsCIDHex(".telda.app") + "] TJ ET",
		"BT /F1 12 Tf 1 0 0 -1 80 200 Tm <009000470041004C> Tj ET", // uses bfchar <0090>->E + GAL
		"BT /F1 12 Tf 1 0 0 -1 280 200 Tm " + encodeASCIIAsCIDHex("Sell") + " Tj ET",
		"Q",
	}, "\n")

	p2 := strings.Join([]string{
		"q",
		"0.24 0 0 -0.24 0 792 cm",
		"q",
		"3.125 0 0 3.125 300 -7800 cm",
		"BT /F1 12 Tf 1 0 0 -1 50 2620 Tm " + encodeASCIIAsCIDHex("Thndr") + " Tj ET",
		"BT /F1 12 Tf 1 0 0 -1 220 2620 Tm " + encodeASCIIAsCIDHex("EGS656M1C010") + " Tj ET",
		"Q",
		"Q",
	}, "\n")

	pdfPath := buildTestPDF(t, []string{p1, p2})
	doc, err := ExtractFile(pdfPath, "")
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}
	if len(doc.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(doc.Pages))
	}

	p1Text := doc.Pages[0].PlainText()
	for _, want := range []string{"Telda", "www.telda.app", "EGAL", "Sell"} {
		if !strings.Contains(p1Text, want) {
			t.Errorf("page 1 missing %q in:\n%s", want, p1Text)
		}
	}

	p2Text := doc.Pages[1].PlainText()
	for _, want := range []string{"Thndr", "EGS656M1C010"} {
		if !strings.Contains(p2Text, want) {
			t.Errorf("page 2 missing %q in:\n%s", want, p2Text)
		}
	}
	for _, r := range doc.Pages[1].Rows {
		if r.Y < 0 || r.Y > doc.Pages[1].Height {
			t.Errorf("page 2 Y=%.2f outside [0..%.2f]: %q", r.Y, doc.Pages[1].Height, r.Text())
		}
	}
}
