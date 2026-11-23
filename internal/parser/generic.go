package parser

import (
	"fmt"
	"regexp"
	"strings"

	"invoicer/internal/model"
	"invoicer/internal/pdfextract"
)

var (
	genericISINRegex       = regexp.MustCompile(`\b([A-Z]{2}[A-Z0-9]{9}[0-9])\b`)
	genericSideRegex       = regexp.MustCompile(`(?i)\b(buy|sell|purchase|sale)\b`)
	genericSymbolLineRegex = regexp.MustCompile(`(?i)(?:symbol(?:\s+code)?|ticker|isin|security\s+code)\s*[:\-]?\s*([A-Z0-9.:-]{2,16})`)
	genericSecNameRegex    = regexp.MustCompile(`(?i)(?:security\s+name|stock|company)\s*[:\-]\s*(.+)$`)
)

// GenericBrokerParser provides a layout-agnostic heuristic fallback parser for
// unseen brokerage confirmation or invoice PDFs.
type GenericBrokerParser struct{}

func (p *GenericBrokerParser) Name() model.Broker {
	return model.BrokerGeneric
}

func (p *GenericBrokerParser) Detect(page *pdfextract.Page) float64 {
	lower := strings.ToLower(page.PlainText())
	hasTradeWords := strings.Contains(lower, "invoice") ||
		strings.Contains(lower, "transaction") ||
		strings.Contains(lower, "order") ||
		strings.Contains(lower, "confirmation")
	hasNumericHeader := (strings.Contains(lower, "quantity") || strings.Contains(lower, "qty") || strings.Contains(lower, "shares")) &&
		(strings.Contains(lower, "price") || strings.Contains(lower, "rate"))
	if hasTradeWords && hasNumericHeader {
		return 0.25
	}
	return 0.0
}

func (p *GenericBrokerParser) Parse(page *pdfextract.Page, sourceFile string) (*model.InvoicePage, error) {
	rows := page.Rows
	plain := page.PlainText()

	inv := &model.InvoicePage{
		SourceFile: sourceFile,
		PageNumber: page.Number,
		Broker:     model.BrokerGeneric,
		Currency:   "EGP",
		Warnings:   []string{"parsed using generic heuristic fallback parser; verify extracted fields"},
	}

	// 1. Date, Side, Currency, Symbol/ISIN
	if dt, ok := ParseDate(plain); ok {
		inv.TradeDate = dt
	} else {
		return nil, fmt.Errorf("generic page %d: no trade date found", page.Number)
	}

	if m := genericSideRegex.FindStringSubmatch(plain); len(m) > 1 {
		if side, ok := NormalizeSide(m[1]); ok {
			inv.Side = side
		}
	}
	if inv.Side == "" {
		return nil, fmt.Errorf("generic page %d: buy/sell side not found", page.Number)
	}

	if m := currencyRegex.FindStringSubmatch(strings.ToUpper(plain)); len(m) > 1 {
		inv.Currency = m[1]
	}

	if m := genericISINRegex.FindStringSubmatch(plain); len(m) > 1 {
		inv.RawSymbolCode = m[1]
	} else if m := genericSymbolLineRegex.FindStringSubmatch(plain); len(m) > 1 {
		inv.RawSymbolCode = strings.TrimSpace(m[1])
	}

	for _, r := range rows {
		if m := genericSecNameRegex.FindStringSubmatch(r.Text()); len(m) > 1 {
			inv.SecurityName = strings.TrimSpace(m[1])
			break
		}
	}

	// 2. Dynamic execution table header detection
	hdrIdx, colKind, centers := findGenericTableHeader(rows)
	if hdrIdx >= 0 {
		for i := hdrIdx + 1; i < len(rows); i++ {
			lower := strings.ToLower(rows[i].Text())
			if strings.Contains(lower, "total") || strings.Contains(lower, "fees") || strings.Contains(lower, "commission") {
				break
			}
			cols := assignColumns(rows[i].Cells, centers)
			var f model.Fill
			var hasQty, hasPrice, hasVal bool
			for ci, kind := range colKind {
				valStr := strings.TrimSpace(joinLines(cols[ci]))
				if valStr == "" {
					continue
				}
				switch kind {
				case "tx":
					if txNoRegex.MatchString(valStr) {
						f.TransactionNo = valStr
					}
				case "qty":
					if q, err := ParseQuantity(valStr); err == nil {
						f.Quantity = q
						hasQty = true
					}
				case "price":
					if pr, _, err := ParseMoney(valStr); err == nil {
						f.Price = pr
						hasPrice = true
					}
				case "value":
					if v, _, err := ParseMoney(valStr); err == nil {
						f.Value = v
						hasVal = true
					}
				}
			}
			if hasQty && hasPrice {
				if !hasVal {
					f.Value = f.Quantity * f.Price
				}
				if f.TransactionNo == "" {
					f.TransactionNo = fmt.Sprintf("P%d-ROW%d", page.Number, len(inv.Fills)+1)
				}
				inv.Fills = append(inv.Fills, f)
			}
		}
	}

	if len(inv.Fills) == 0 {
		return nil, fmt.Errorf("generic page %d: unable to extract execution fills", page.Number)
	}

	for _, f := range inv.Fills {
		inv.TotalQuantity += f.Quantity
		inv.TotalValue += f.Value
	}
	if inv.TotalQuantity > 0 {
		inv.AveragePrice = inv.TotalValue / inv.TotalQuantity
	}

	// 3. Extract Total Fees / Commission and Grand Total
	for _, r := range rows {
		lower := strings.ToLower(r.Text())
		if len(r.Cells) < 2 {
			continue
		}
		amtStr := r.Cells[len(r.Cells)-1].Text
		amt, _, err := ParseMoney(amtStr)
		if err != nil {
			continue
		}
		if (strings.Contains(lower, "total fees") || strings.Contains(lower, "total commission") || strings.HasPrefix(lower, "commission")) && inv.TotalFees == 0 {
			inv.TotalFees = amt
			inv.Fees = append(inv.Fees, model.FeeItem{Name: "Total Fees", Amount: amt})
		} else if (strings.Contains(lower, "grand total") || strings.Contains(lower, "net total") || strings.Contains(lower, "net amount")) && inv.GrandTotal == 0 {
			inv.GrandTotal = amt
		}
	}
	if inv.GrandTotal == 0 {
		if inv.Side == model.SideSell {
			inv.GrandTotal = inv.TotalValue - inv.TotalFees
		} else {
			inv.GrandTotal = inv.TotalValue + inv.TotalFees
		}
	}

	return inv, nil
}

func findGenericTableHeader(rows []pdfextract.VisualRow) (idx int, kinds []string, centers []float64) {
	for i, r := range rows {
		var k []string
		var c []float64
		hasQty, hasPrice := false, false
		for _, cell := range sortCellsByX(r.Cells) {
			low := strings.ToLower(strings.TrimSpace(cell.Text))
			switch {
			case strings.Contains(low, "transaction") || strings.Contains(low, "order no") || strings.Contains(low, "ref"):
				k = append(k, "tx")
				c = append(c, cellCenterX(cell))
			case strings.Contains(low, "quantity") || low == "qty" || strings.Contains(low, "shares"):
				k = append(k, "qty")
				c = append(c, cellCenterX(cell))
				hasQty = true
			case strings.Contains(low, "fill price") || strings.Contains(low, "price") || strings.Contains(low, "rate"):
				k = append(k, "price")
				c = append(c, cellCenterX(cell))
				hasPrice = true
			case strings.Contains(low, "value") || strings.Contains(low, "amount") || strings.Contains(low, "cost") || strings.Contains(low, "gross"):
				k = append(k, "value")
				c = append(c, cellCenterX(cell))
			}
		}
		if hasQty && hasPrice {
			return i, k, c
		}
	}
	return -1, nil, nil
}
