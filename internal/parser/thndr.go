package parser

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"invoicer/internal/model"
	"invoicer/internal/pdfextract"
)

// ThndrParser parses Thndr Securities Brokerage E-INVOICE pages.
type ThndrParser struct{}

func (p *ThndrParser) Name() model.Broker {
	return model.BrokerThndr
}

func (p *ThndrParser) Detect(page *pdfextract.Page) float64 {
	lower := strings.ToLower(page.PlainText())
	score := 0.0
	if strings.Contains(lower, "thndr") {
		score += 0.7
	}
	if strings.Contains(lower, "trading damgha") || strings.Contains(lower, "fra services") || strings.Contains(lower, "brokerage &") {
		score += 0.4
	}
	if strings.Contains(lower, "total quantity") && strings.Contains(lower, "average price") {
		score += 0.3
	}
	return score
}

func (p *ThndrParser) Parse(page *pdfextract.Page, sourceFile string) (*model.InvoicePage, error) {
	rows := page.Rows
	inv := &model.InvoicePage{
		SourceFile: sourceFile,
		PageNumber: page.Number,
		Broker:     model.BrokerThndr,
		Currency:   "EGP",
	}

	secHdrIdx, secCenters, okSec := findHeaderRow(rows, []string{"security name", "symbol code", "transaction type", "average cost"})
	if !okSec {
		return nil, fmt.Errorf("thndr page %d: security header row not found", page.Number)
	}
	txHdrIdx, txCenters, okTx := findHeaderRow(rows, []string{"transaction no", "quantity", "price", "value"})
	if !okTx {
		return nil, fmt.Errorf("thndr page %d: transaction table header not found", page.Number)
	}
	feeHdrIdx, feeCenters, okFee := findHeaderRow(rows, []string{"fees", "amount"})
	if !okFee {
		return nil, fmt.Errorf("thndr page %d: fees table header not found", page.Number)
	}

	// 1. Header metadata (customer, custodian, trade date) above security header
	for i := 0; i < secHdrIdx; i++ {
		for _, c := range rows[i].Cells {
			txt := strings.TrimSpace(c.Text)
			lower := strings.ToLower(txt)
			if txt == "" || lower == "invoice" {
				continue
			}
			if strings.HasPrefix(lower, "custodian:") {
				inv.Custodian = strings.TrimSpace(txt[len("custodian:"):])
				continue
			}
			if dt, ok := ParseDate(txt); ok {
				if inv.TradeDate.IsZero() {
					inv.TradeDate = dt
				}
				continue
			}
			if inv.CustomerName == "" && c.X < 250 {
				inv.CustomerName = txt
			}
		}
	}
	if inv.TradeDate.IsZero() {
		return nil, fmt.Errorf("thndr page %d: trade date not found", page.Number)
	}

	// 2. Multi-line security metadata band (between secHdrIdx and txHdrIdx)
	var secCells []pdfextract.TextCell
	for i := secHdrIdx + 1; i < txHdrIdx; i++ {
		secCells = append(secCells, rows[i].Cells...)
	}
	secCols := assignColumns(secCells, secCenters)
	inv.SecurityName = joinLines(secCols[0])
	inv.RawSymbolCode = strings.TrimSpace(joinLines(secCols[1]))
	if side, ok := NormalizeSide(joinLines(secCols[2])); ok {
		inv.Side = side
	} else {
		return nil, fmt.Errorf("thndr page %d: unrecognized side %q", page.Number, joinLines(secCols[2]))
	}
	if avgCost, cur, err := ParseMoney(joinLines(secCols[3])); err == nil {
		inv.AverageCost = avgCost
		if cur != "" {
			inv.Currency = cur
		}
	}

	// 3. Fills table & Total summary band (between txHdrIdx and feeHdrIdx)
	totHdrIdx := feeHdrIdx
	for i := txHdrIdx + 1; i < feeHdrIdx; i++ {
		lowerRow := strings.ToLower(rows[i].Text())
		if strings.Contains(lowerRow, "total quantity") && strings.Contains(lowerRow, "average price") {
			totHdrIdx = i
			break
		}
		cols := assignColumns(rows[i].Cells, txCenters)
		txNo := strings.TrimSpace(joinLines(cols[0]))
		qtyStr := joinLines(cols[1])
		priceStr := joinLines(cols[2])
		valStr := joinLines(cols[3])
		if txNo == "" || qtyStr == "" || priceStr == "" || valStr == "" {
			continue
		}
		qty, err1 := ParseQuantity(qtyStr)
		price, cur1, err2 := ParseMoney(priceStr)
		val, cur2, err3 := ParseMoney(valStr)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if cur1 != "" {
			inv.Currency = cur1
		} else if cur2 != "" {
			inv.Currency = cur2
		}
		inv.Fills = append(inv.Fills, model.Fill{
			TransactionNo: txNo,
			Quantity:      qty,
			Price:         price,
			Value:         val,
		})
	}
	if len(inv.Fills) == 0 {
		return nil, fmt.Errorf("thndr page %d: no execution fills extracted", page.Number)
	}

	if totHdrIdx < feeHdrIdx {
		_, totCenters, okTot := findHeaderRow(rows[totHdrIdx:totHdrIdx+1], []string{"total quantity", "average price", "total cost"})
		if okTot {
			var totCells []pdfextract.TextCell
			for i := totHdrIdx + 1; i < feeHdrIdx; i++ {
				totCells = append(totCells, rows[i].Cells...)
			}
			totCols := assignColumns(totCells, totCenters)
			if q, err := ParseQuantity(joinLines(totCols[0])); err == nil {
				inv.TotalQuantity = q
			}
			if ap, _, err := ParseMoney(joinLines(totCols[1])); err == nil {
				inv.AveragePrice = ap
			}
			if tv, _, err := ParseMoney(joinLines(totCols[2])); err == nil {
				inv.TotalValue = tv
			}
		}
	}
	if inv.TotalQuantity == 0 || inv.TotalValue == 0 {
		var sumQ, sumV float64
		for _, f := range inv.Fills {
			sumQ += f.Quantity
			sumV += f.Value
		}
		if inv.TotalQuantity == 0 {
			inv.TotalQuantity = sumQ
		}
		if inv.TotalValue == 0 {
			inv.TotalValue = sumV
		}
		if inv.AveragePrice == 0 && inv.TotalQuantity > 0 {
			inv.AveragePrice = inv.TotalValue / inv.TotalQuantity
		}
	}

	// 4. Vertically-paired Fees section (handles 2-line fee names whose amount is vertically centered)
	type amtEntry struct {
		y      float64
		amount float64
		labels []pdfextract.TextCell
	}
	splitX := feeCenters[1] - 40.0
	if feeCenters[1] > feeCenters[0] {
		splitX = (feeCenters[0] + feeCenters[1]) / 2.0
	}

	var amtEntries []amtEntry
	var labelFragments []pdfextract.TextCell

	for i := feeHdrIdx + 1; i < len(rows); i++ {
		if isRowMatchingPrefix(rows[i], "note:") {
			break
		}
		for _, c := range rows[i].Cells {
			txt := strings.TrimSpace(c.Text)
			if txt == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(txt), "note:") {
				break
			}
			if c.X >= splitX {
				if amt, _, err := ParseMoney(txt); err == nil {
					amtEntries = append(amtEntries, amtEntry{
						y:      c.Y,
						amount: amt,
					})
				}
			} else if c.X >= feeCenters[0]-80 {
				labelFragments = append(labelFragments, c)
			}
		}
	}

	if len(amtEntries) > 0 {
		for _, lbl := range labelFragments {
			bestIdx := 0
			bestDist := math.Abs(lbl.Y - amtEntries[0].y)
			for j := 1; j < len(amtEntries); j++ {
				d := math.Abs(lbl.Y - amtEntries[j].y)
				if d < bestDist {
					bestDist = d
					bestIdx = j
				}
			}
			if bestDist <= 25.0 {
				amtEntries[bestIdx].labels = append(amtEntries[bestIdx].labels, lbl)
			}
		}

		for _, entry := range amtEntries {
			sort.Slice(entry.labels, func(i, j int) bool {
				if math.Abs(entry.labels[i].Y-entry.labels[j].Y) > 2.0 {
					return entry.labels[i].Y < entry.labels[j].Y
				}
				return entry.labels[i].X < entry.labels[j].X
			})
			parts := make([]string, 0, len(entry.labels))
			for _, lc := range entry.labels {
				parts = append(parts, strings.TrimSpace(lc.Text))
			}
			label := strings.Join(parts, " ")
			lowerLabel := strings.ToLower(label)
			switch {
			case lowerLabel == "total fees":
				inv.TotalFees = entry.amount
			case lowerLabel == "grand total":
				inv.GrandTotal = entry.amount
			case label != "":
				inv.Fees = append(inv.Fees, model.FeeItem{
					Name:   label,
					Amount: entry.amount,
				})
			}
		}
	}

	return inv, nil
}
