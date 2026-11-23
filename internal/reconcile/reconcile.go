// Package reconcile mathematically verifies extracted brokerage invoices
// (fills, fees, and grand totals), allocates commissions across fills without
// rounding drift, and converts invoices into accounting CSV records.
package reconcile

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"invoicer/internal/model"
)

const moneyTolerance = 0.02
const qtyTolerance = 0.0001

// Reconcile performs accounting integrity checks on inv, populates inv.Reconciliation,
// and appends human-readable warnings to inv.Warnings for any failed check.
func Reconcile(inv *model.InvoicePage) {
	var checks []model.ReconciliationCheck
	allPassed := true

	addCheck := func(name string, expected, actual, tol float64) {
		diff := math.Abs(expected - actual)
		passed := diff <= tol
		msg := ""
		if !passed {
			allPassed = false
			msg = fmt.Sprintf("%s mismatch on %s page %d: expected %.4f, got %.4f (diff %.4f)",
				name, inv.SourceFile, inv.PageNumber, expected, actual, diff)
			inv.Warnings = append(inv.Warnings, msg)
		}
		checks = append(checks, model.ReconciliationCheck{
			Name:     name,
			Expected: round4(expected),
			Actual:   round4(actual),
			Diff:     round4(diff),
			Passed:   passed,
			Message:  msg,
		})
	}

	var sumQty, sumVal float64
	for i, f := range inv.Fills {
		expectedVal := round2(f.Quantity * f.Price)
		addCheck(fmt.Sprintf("fill_%d_value", i+1), expectedVal, f.Value, moneyTolerance)
		sumQty += f.Quantity
		sumVal += f.Value
	}

	if inv.TotalQuantity > 0 {
		addCheck("total_quantity", sumQty, inv.TotalQuantity, qtyTolerance)
	}
	if inv.TotalValue > 0 {
		addCheck("total_value", sumVal, inv.TotalValue, moneyTolerance)
	}

	effectiveVal := inv.TotalValue
	if effectiveVal == 0 {
		effectiveVal = sumVal
	}

	if len(inv.Fees) > 0 {
		var sumFees float64
		for _, fee := range inv.Fees {
			sumFees += fee.Amount
		}
		addCheck("total_fees", sumFees, inv.TotalFees, moneyTolerance)
	}

	if inv.GrandTotal > 0 {
		expectedGrand := effectiveVal + inv.TotalFees
		if inv.Side == model.SideSell {
			expectedGrand = effectiveVal - inv.TotalFees
		}
		addCheck("grand_total", expectedGrand, inv.GrandTotal, moneyTolerance)
	}

	inv.Reconciliation = model.ReconciliationResult{
		Passed: allPassed,
		Checks: checks,
	}
}

// AllocateCommissions distributes inv.TotalFees across inv.Fills in integer
// cents/piastres using the Largest Remainder Method (Hamilton apportionment),
// after carving out per-fill fixed EFSA/FRA fees when applicable.
func AllocateCommissions(inv *model.InvoicePage) []float64 {
	n := len(inv.Fills)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []float64{round2(inv.TotalFees)}
	}

	totalCents := int(math.Round(inv.TotalFees * 100))
	fixedPerFillCents := 0

	// Detect per-fill regulatory fixed fee (EFSA Fees / FRA Services = 1.00 EGP per execution)
	for _, fee := range inv.Fees {
		low := strings.ToLower(fee.Name)
		if strings.Contains(low, "efsa") || strings.Contains(low, "fra") {
			if math.Abs(fee.Amount-float64(n)) <= 0.01 {
				fixedPerFillCents = 100
				break
			}
		}
	}

	remainingCents := totalCents - fixedPerFillCents*n
	sign := 1
	absRem := remainingCents
	if absRem < 0 {
		sign = -1
		absRem = -absRem
	}

	weights := make([]float64, n)
	var sumWeights float64
	for i, f := range inv.Fills {
		w := math.Abs(f.Value)
		if w == 0 {
			w = math.Abs(f.Quantity * f.Price)
		}
		weights[i] = w
		sumWeights += w
	}
	if sumWeights <= 0 {
		sumWeights = 0
		for i, f := range inv.Fills {
			w := math.Abs(f.Quantity)
			if w <= 0 {
				w = 1
			}
			weights[i] = w
			sumWeights += w
		}
	}

	type remPart struct {
		idx int
		rem float64
	}
	floors := make([]int, n)
	rems := make([]remPart, n)
	allocatedFloorSum := 0

	for i := 0; i < n; i++ {
		exact := float64(absRem) * weights[i] / sumWeights
		fl := int(math.Floor(exact))
		floors[i] = fl
		allocatedFloorSum += fl
		rems[i] = remPart{idx: i, rem: exact - float64(fl)}
	}

	sort.SliceStable(rems, func(i, j int) bool {
		if rems[i].rem != rems[j].rem {
			return rems[i].rem > rems[j].rem
		}
		return rems[i].idx < rems[j].idx
	})

	leftover := absRem - allocatedFloorSum
	for k := 0; k < leftover && k < n; k++ {
		floors[rems[k].idx]++
	}

	out := make([]float64, n)
	for i := 0; i < n; i++ {
		cents := fixedPerFillCents + sign*floors[i]
		out[i] = float64(cents) / 100.0
	}
	return out
}

// ToCSVRecords converts a parsed InvoicePage into accounting CSV records at either
// "fill" granularity (one row per fill) or "order" granularity (one row per page).
func ToCSVRecords(inv *model.InvoicePage, granularity string, defaultTime string) ([]model.CSVRecord, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invoice page")
	}
	g := strings.ToLower(strings.TrimSpace(granularity))
	if g == "" {
		g = "fill"
	}
	if g != "fill" && g != "order" {
		return nil, fmt.Errorf("unsupported granularity %q (must be 'fill' or 'order')", granularity)
	}

	closingTime, err := combineDateAndTime(inv.TradeDate, defaultTime)
	if err != nil {
		return nil, err
	}

	sym := inv.ResolvedSymbol
	if sym == "" {
		sym = inv.RawSymbolCode
	}

	if g == "order" {
		qty := inv.TotalQuantity
		if qty == 0 {
			for _, f := range inv.Fills {
				qty += f.Quantity
			}
		}
		val := inv.TotalValue
		if val == 0 {
			for _, f := range inv.Fills {
				val += f.Value
			}
		}
		avgPrice := inv.AveragePrice
		if avgPrice == 0 && qty > 0 {
			avgPrice = val / qty
		} else if qty > 0 && math.Abs(avgPrice*qty-val) > 0.05 {
			avgPrice = val / qty
		} else if qty > 0 && inv.AveragePrice > 0 {
			// Preserve high-precision weighted average when invoice rounded to 2dp (e.g., 904.58 / 3 = 301.5267)
			avgPrice = val / qty
		}
		fp := round4(avgPrice)
		comm := round2(inv.TotalFees)
		txNos := make([]string, 0, len(inv.Fills))
		for _, f := range inv.Fills {
			if f.TransactionNo != "" {
				txNos = append(txNos, f.TransactionNo)
			}
		}
		return []model.CSVRecord{{
			Symbol:        sym,
			Side:          inv.Side,
			Qty:           round4(qty),
			FillPrice:     &fp,
			Commission:    &comm,
			ClosingTime:   closingTime,
			SourceFile:    inv.SourceFile,
			PageNumber:    inv.PageNumber,
			Broker:        inv.Broker,
			TransactionNo: strings.Join(txNos, "+"),
		}}, nil
	}

	comms := AllocateCommissions(inv)
	records := make([]model.CSVRecord, 0, len(inv.Fills))
	for i, f := range inv.Fills {
		fp := round4(f.Price)
		comm := round2(comms[i])
		records = append(records, model.CSVRecord{
			Symbol:        sym,
			Side:          inv.Side,
			Qty:           round4(f.Quantity),
			FillPrice:     &fp,
			Commission:    &comm,
			ClosingTime:   closingTime,
			SourceFile:    inv.SourceFile,
			PageNumber:    inv.PageNumber,
			Broker:        inv.Broker,
			TransactionNo: f.TransactionNo,
		})
	}
	return records, nil
}

func combineDateAndTime(tradeDate time.Time, defaultTime string) (time.Time, error) {
	tStr := strings.TrimSpace(defaultTime)
	if tStr == "" {
		tStr = "00:00:00"
	}
	parsedTod, err := time.Parse("15:04:05", tStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid default-time %q (expected HH:MM:SS): %w", defaultTime, err)
	}
	return time.Date(
		tradeDate.Year(), tradeDate.Month(), tradeDate.Day(),
		parsedTod.Hour(), parsedTod.Minute(), parsedTod.Second(), 0, time.UTC,
	), nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
