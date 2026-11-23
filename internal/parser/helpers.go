// Package parser detects brokerage invoice formats from layout-extracted PDF
// pages and parses structured transaction, fee, and summary data.
package parser

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"invoicer/internal/model"
	"invoicer/internal/pdfextract"
)

var (
	moneyAmountRegex = regexp.MustCompile(`([+-]?(?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?)`)
	currencyRegex    = regexp.MustCompile(`\b(EGP|USD|EUR|GBP|SAR|AED)\b`)
	dateYMDRegex     = regexp.MustCompile(`\b(\d{4})[-/](\d{2})[-/](\d{2})\b`)
	dateDMYRegex     = regexp.MustCompile(`\b(\d{2})[-/](\d{2})[-/](\d{4})\b`)
	dateTextRegex    = regexp.MustCompile(`\b(\d{1,2}\s+[A-Za-z]{3,9}\s+\d{4}|[A-Za-z]{3,9}\s+\d{1,2},?\s+\d{4})\b`)
	txNoRegex        = regexp.MustCompile(`^[A-Za-z0-9_-]{6,32}$`)
)

// ParseMoney extracts a signed decimal amount and optional 3-letter currency
// code from strings such as "EGP 301.56", "EGP -2.00", "-EGP 2.00",
// "1,239.00 EGP", "(2.00)", or "302".
func ParseMoney(s string) (amount float64, currency string, err error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, "", fmt.Errorf("empty money string")
	}

	if m := currencyRegex.FindStringSubmatch(strings.ToUpper(raw)); len(m) > 1 {
		currency = m[1]
	}

	negative := false
	cleaned := raw
	if strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")") {
		negative = true
		cleaned = strings.TrimSpace(cleaned[1 : len(cleaned)-1])
	}
	if strings.HasPrefix(cleaned, "-") && !strings.HasPrefix(cleaned, "-0") && !strings.Contains(cleaned, " -") {
		// Handle leading negative before currency like "-EGP 2.00"
		if strings.IndexAny(cleaned, "0123456789") > 1 {
			negative = true
			cleaned = strings.TrimSpace(cleaned[1:])
		}
	}

	m := moneyAmountRegex.FindStringSubmatch(cleaned)
	if len(m) < 2 {
		return 0, currency, fmt.Errorf("no numeric amount in %q", s)
	}
	numStr := strings.ReplaceAll(m[1], ",", "")
	val, parseErr := strconv.ParseFloat(numStr, 64)
	if parseErr != nil {
		return 0, currency, fmt.Errorf("parse money %q: %w", s, parseErr)
	}
	if negative && val > 0 {
		val = -val
	}
	return val, currency, nil
}

// ParseQuantity parses a comma-formatted share quantity string (e.g. "190.00", "1,100").
func ParseQuantity(s string) (float64, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if cleaned == "" {
		return 0, fmt.Errorf("empty quantity")
	}
	v, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parse quantity %q: %w", s, err)
	}
	return v, nil
}

// ParseDate finds and parses a trade date from text in ISO, DD/MM/YYYY, or named-month formats.
func ParseDate(s string) (time.Time, bool) {
	if m := dateYMDRegex.FindStringSubmatch(s); len(m) == 4 {
		t, err := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3]))
		if err == nil {
			return t, true
		}
	}
	if m := dateDMYRegex.FindStringSubmatch(s); len(m) == 4 {
		// Prefer DD/MM/YYYY (EGX/Egyptian broker standard); fallback to MM/DD/YYYY if day > 12 invalidates
		dmy := fmt.Sprintf("%s/%s/%s", m[1], m[2], m[3])
		if t, err := time.Parse("02/01/2006", dmy); err == nil {
			return t, true
		}
		if t, err := time.Parse("01/02/2006", dmy); err == nil {
			return t, true
		}
	}
	if m := dateTextRegex.FindStringSubmatch(s); len(m) >= 2 {
		candidate := strings.Join(strings.Fields(m[1]), " ")
		for _, layout := range []string{"2 Jan 2006", "2 January 2006", "Jan 2, 2006", "January 2, 2006", "Jan 2 2006", "January 2 2006"} {
			if t, err := time.Parse(layout, candidate); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// NormalizeSide converts raw transaction side text into model.SideBuy or model.SideSell.
func NormalizeSide(s string) (model.Side, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "buy", "purchase", "bought", "b":
		return model.SideBuy, true
	case "sell", "sale", "sold", "s":
		return model.SideSell, true
	default:
		return "", false
	}
}

func cellCenterX(c pdfextract.TextCell) float64 {
	if c.EndX > c.X {
		return (c.X + c.EndX) / 2.0
	}
	return c.X
}

// assignColumns maps cells in a row (or band) to header column indices using
// midpoints between sorted column header center-X positions.
func assignColumns(cells []pdfextract.TextCell, headerCenters []float64) [][]string {
	cols := make([][]string, len(headerCenters))
	if len(headerCenters) == 0 {
		return cols
	}
	for _, c := range cells {
		txt := strings.TrimSpace(c.Text)
		if txt == "" {
			continue
		}
		cx := cellCenterX(c)
		bestIdx := 0
		bestDist := math.Abs(cx - headerCenters[0])
		for i := 1; i < len(headerCenters); i++ {
			d := math.Abs(cx - headerCenters[i])
			if d < bestDist {
				bestDist = d
				bestIdx = i
			}
		}
		cols[bestIdx] = append(cols[bestIdx], txt)
	}
	return cols
}

// findHeaderRow returns the index of the first VisualRow matching all required
// lowercase substrings (each matched against any cell or the joined row text),
// plus the center-X coordinates corresponding to each requested column keyword.
func findHeaderRow(rows []pdfextract.VisualRow, colKeywords []string) (rowIdx int, centers []float64, ok bool) {
	for idx, r := range rows {
		matched := make([]float64, len(colKeywords))
		allFound := true
		for ki, kw := range colKeywords {
			found := false
			for _, c := range r.Cells {
				if strings.Contains(strings.ToLower(c.Text), kw) {
					matched[ki] = cellCenterX(c)
					found = true
					break
				}
			}
			if !found {
				allFound = false
				break
			}
		}
		if allFound {
			return idx, matched, true
		}
	}
	return -1, nil, false
}

func joinLines(parts []string) string {
	return strings.Join(parts, " ")
}

func isRowMatchingPrefix(r pdfextract.VisualRow, prefixes ...string) bool {
	joined := strings.ToLower(strings.TrimSpace(r.Text()))
	for _, p := range prefixes {
		if strings.HasPrefix(joined, p) {
			return true
		}
	}
	return false
}

// sortCellsByX sorts cells left-to-right.
func sortCellsByX(cells []pdfextract.TextCell) []pdfextract.TextCell {
	out := make([]pdfextract.TextCell, len(cells))
	copy(out, cells)
	sort.Slice(out, func(i, j int) bool {
		return out[i].X < out[j].X
	})
	return out
}
