package pdfextract

import (
	"math"
	"sort"
	"strings"
)

// TextCell represents a horizontally contiguous text fragment (e.g., a single
// column cell or label) inside a VisualRow.
type TextCell struct {
	Text     string
	X        float64
	Y        float64
	EndX     float64
	FontSize float64
	Bold     bool
}

// VisualRow groups TextCells that share the same horizontal baseline band.
type VisualRow struct {
	Y     float64
	Cells []TextCell
}

// Text returns the row's cells joined by single spaces.
func (r VisualRow) Text() string {
	parts := make([]string, 0, len(r.Cells))
	for _, c := range r.Cells {
		if t := strings.TrimSpace(c.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// Page holds the extracted spans and clustered visual rows for one PDF page.
type Page struct {
	Number int
	Width  float64
	Height float64
	Spans  []TextSpan
	Rows   []VisualRow
}

// PlainText renders the entire page line-by-line from clustered VisualRows.
func (p *Page) PlainText() string {
	lines := make([]string, 0, len(p.Rows))
	for _, r := range p.Rows {
		if t := r.Text(); t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n")
}

// Document represents a parsed PDF document and its layout-extracted pages.
type Document struct {
	Path  string
	Pages []*Page
}

// buildVisualRows clusters spans by Y-band, sorts by X within each row, and
// merges adjacent spans into TextCells according to inter-span gap thresholds.
func buildVisualRows(spans []TextSpan) []VisualRow {
	if len(spans) == 0 {
		return nil
	}

	sorted := make([]TextSpan, len(spans))
	copy(sorted, spans)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Y != sorted[j].Y {
			return sorted[i].Y < sorted[j].Y
		}
		return sorted[i].X < sorted[j].X
	})

	var rowClusters [][]TextSpan
	for _, sp := range sorted {
		if len(rowClusters) == 0 {
			rowClusters = append(rowClusters, []TextSpan{sp})
			continue
		}
		lastCluster := &rowClusters[len(rowClusters)-1]
		anchorY := (*lastCluster)[0].Y
		minFS := (*lastCluster)[0].FontSize
		for _, item := range *lastCluster {
			if item.FontSize > 0 && (minFS == 0 || item.FontSize < minFS) {
				minFS = item.FontSize
			}
		}
		if sp.FontSize > 0 && (minFS == 0 || sp.FontSize < minFS) {
			minFS = sp.FontSize
		}
		tol := math.Max(3.0, 0.45*minFS)
		if math.Abs(sp.Y-anchorY) <= tol {
			*lastCluster = append(*lastCluster, sp)
		} else {
			rowClusters = append(rowClusters, []TextSpan{sp})
		}
	}

	rows := make([]VisualRow, 0, len(rowClusters))
	for _, cluster := range rowClusters {
		sort.SliceStable(cluster, func(i, j int) bool {
			return cluster[i].X < cluster[j].X
		})
		cells := mergeSpansIntoCells(cluster)
		if len(cells) == 0 {
			continue
		}
		avgY := 0.0
		for _, c := range cells {
			avgY += c.Y
		}
		avgY /= float64(len(cells))
		rows = append(rows, VisualRow{
			Y:     avgY,
			Cells: cells,
		})
	}
	return rows
}

func mergeSpansIntoCells(spans []TextSpan) []TextCell {
	if len(spans) == 0 {
		return nil
	}
	var cells []TextCell
	cur := TextCell{
		Text:     spans[0].Text,
		X:        spans[0].X,
		Y:        spans[0].Y,
		EndX:     spans[0].EndX,
		FontSize: spans[0].FontSize,
		Bold:     spans[0].Bold,
	}

	for i := 1; i < len(spans); i++ {
		next := spans[i]
		fs := cur.FontSize
		if next.FontSize > 0 && (fs == 0 || next.FontSize < fs) {
			fs = next.FontSize
		}
		if fs == 0 {
			fs = 10
		}

		gap := next.X - cur.EndX
		if gap <= 0.18*fs {
			// Directly contiguous / kerning split within the same word
			cur.Text += next.Text
			if next.EndX > cur.EndX {
				cur.EndX = next.EndX
			}
			cur.Bold = cur.Bold || next.Bold
		} else if gap <= 1.2*fs {
			// Normal inter-word space within the same column cell
			if !strings.HasSuffix(cur.Text, " ") && !strings.HasPrefix(next.Text, " ") {
				cur.Text += " "
			}
			cur.Text += next.Text
			if next.EndX > cur.EndX {
				cur.EndX = next.EndX
			}
			cur.Bold = cur.Bold || next.Bold
		} else {
			if trimmed := strings.TrimSpace(cur.Text); trimmed != "" {
				cur.Text = trimmed
				cells = append(cells, cur)
			}
			cur = TextCell{
				Text:     next.Text,
				X:        next.X,
				Y:        next.Y,
				EndX:     next.EndX,
				FontSize: next.FontSize,
				Bold:     next.Bold,
			}
		}
	}

	if trimmed := strings.TrimSpace(cur.Text); trimmed != "" {
		cur.Text = trimmed
		cells = append(cells, cur)
	}
	return cells
}
