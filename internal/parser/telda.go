package parser

import (
	"fmt"
	"strings"

	"invoicer/internal/model"
	"invoicer/internal/pdfextract"
)

// TeldaParser parses Telda Securities brokerage invoice pages.
type TeldaParser struct{}

func (p *TeldaParser) Name() model.Broker {
	return model.BrokerTelda
}

func (p *TeldaParser) Detect(page *pdfextract.Page) float64 {
	lower := strings.ToLower(page.PlainText())
	score := 0.0
	if strings.Contains(lower, "www.telda.app") {
		score += 0.6
	}
	if strings.Contains(lower, "telda fees") || strings.Contains(lower, "telda incentive") {
		score += 0.5
	}
	if strings.Contains(lower, "transaction no:") && strings.Contains(lower, "symbol code:") {
		score += 0.3
	}
	return score
}

func (p *TeldaParser) Parse(page *pdfextract.Page, sourceFile string) (*model.InvoicePage, error) {
	rows := page.Rows
	inv := &model.InvoicePage{
		SourceFile: sourceFile,
		PageNumber: page.Number,
		Broker:     model.BrokerTelda,
		Currency:   "EGP",
	}

	// 1. Locate key table headers first so we can bound header metadata above them
	secHdrIdx, secCenters, okSec := findHeaderRow(rows, []string{"security name", "symbol code", "transaction type", "average cost"})
	if !okSec {
		return nil, fmt.Errorf("telda page %d: security header row not found", page.Number)
	}
	txHdrIdx, txCenters, okTx := findHeaderRow(rows, []string{"transaction no", "quantity", "price", "value"})
	if !okTx {
		return nil, fmt.Errorf("telda page %d: transaction table header not found", page.Number)
	}
	feeHdrIdx, feeCenters, okFee := findHeaderRow(rows, []string{"fees", "amount"})
	if !okFee {
		return nil, fmt.Errorf("telda page %d: fees table header not found", page.Number)
	}

	// 2. Customer name & Trade date above security header
	for i := 0; i < secHdrIdx; i++ {
		line := strings.TrimSpace(rows[i].Text())
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "securities invoice" || lower == "invoice" || lower == "securities" {
			continue
		}
		if dt, ok := ParseDate(line); ok {
			if inv.TradeDate.IsZero() {
				inv.TradeDate = dt
			}
			continue
		}
		if inv.CustomerName == "" && len(rows[i].Cells) == 1 {
			inv.CustomerName = line
		}
	}
	if inv.TradeDate.IsZero() {
		return nil, fmt.Errorf("telda page %d: trade date not found", page.Number)
	}

	// 3. Security metadata band (between secHdrIdx and txHdrIdx)
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
		return nil, fmt.Errorf("telda page %d: unrecognized side %q", page.Number, joinLines(secCols[2]))
	}
	if avgCost, cur, err := ParseMoney(joinLines(secCols[3])); err == nil {
		inv.AverageCost = avgCost
		if cur != "" {
			inv.Currency = cur
		}
	}

	// 4. Fills table (between txHdrIdx and feeHdrIdx)
	for i := txHdrIdx + 1; i < feeHdrIdx; i++ {
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
		return nil, fmt.Errorf("telda page %d: no execution fills extracted", page.Number)
	}

	for _, f := range inv.Fills {
		inv.TotalQuantity += f.Quantity
		inv.TotalValue += f.Value
	}
	if inv.TotalQuantity > 0 {
		inv.AveragePrice = inv.TotalValue / inv.TotalQuantity
	}

	// 5. Fees & Grand Total (starting after feeHdrIdx)
	for i := feeHdrIdx + 1; i < len(rows); i++ {
		cols := assignColumns(rows[i].Cells, feeCenters)
		label := strings.TrimSpace(joinLines(cols[0]))
		amtStr := strings.TrimSpace(joinLines(cols[1]))
		if label == "" || amtStr == "" {
			continue
		}
		lowerLabel := strings.ToLower(label)
		if strings.Contains(lowerLabel, "telda.app") {
			continue
		}
		amt, _, err := ParseMoney(amtStr)
		if err != nil {
			continue
		}
		switch {
		case lowerLabel == "total fees":
			inv.TotalFees = amt
		case lowerLabel == "grand total":
			inv.GrandTotal = amt
		default:
			inv.Fees = append(inv.Fees, model.FeeItem{
				Name:   label,
				Amount: amt,
			})
		}
	}

	return inv, nil
}
