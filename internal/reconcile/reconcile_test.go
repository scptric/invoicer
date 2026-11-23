package reconcile

import (
	"math"
	"testing"
	"time"

	"invoicer/internal/model"
)

func TestReconcileSellAndBuy(t *testing.T) {
	sellInv := &model.InvoicePage{
		SourceFile:    "telda.pdf",
		PageNumber:    1,
		Side:          model.SideSell,
		TradeDate:     time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC),
		Fills:         []model.Fill{{Quantity: 1, Price: 301.56, Value: 301.56}, {Quantity: 2, Price: 301.51, Value: 603.02}},
		TotalQuantity: 3,
		TotalValue:    904.58,
		Fees: []model.FeeItem{
			{Name: "Telda Fees", Amount: 2.00},
			{Name: "EGX Fees", Amount: 0.09},
			{Name: "MCDR Fees", Amount: 0.09},
			{Name: "EFSA Fees", Amount: 2.00},
			{Name: "Risk Insurance Fees", Amount: 0.05},
			{Name: "Trading Stamp Fees", Amount: 0.45},
			{Name: "Telda Incentive", Amount: -2.00},
		},
		TotalFees:  2.68,
		GrandTotal: 901.90,
	}
	Reconcile(sellInv)
	if !sellInv.Reconciliation.Passed || len(sellInv.Warnings) != 0 {
		t.Fatalf("expected Sell invoice to pass cleanly, warnings=%v checks=%+v", sellInv.Warnings, sellInv.Reconciliation.Checks)
	}

	buyInv := &model.InvoicePage{
		SourceFile:    "buy.pdf",
		PageNumber:    1,
		Side:          model.SideBuy,
		Fills:         []model.Fill{{Quantity: 10, Price: 50.00, Value: 500.00}},
		TotalQuantity: 10,
		TotalValue:    500.00,
		Fees:          []model.FeeItem{{Name: "Brokerage", Amount: 3.50}},
		TotalFees:     3.50,
		GrandTotal:    503.50,
	}
	Reconcile(buyInv)
	if !buyInv.Reconciliation.Passed || len(buyInv.Warnings) != 0 {
		t.Fatalf("expected Buy invoice to pass cleanly, warnings=%v", buyInv.Warnings)
	}

	badInv := &model.InvoicePage{
		SourceFile:    "bad.pdf",
		PageNumber:    1,
		Side:          model.SideSell,
		Fills:         []model.Fill{{Quantity: 10, Price: 50.00, Value: 500.00}},
		TotalQuantity: 10,
		TotalValue:    500.00,
		Fees:          []model.FeeItem{{Name: "Brokerage", Amount: 3.50}},
		TotalFees:     3.50,
		GrandTotal:    490.00, // mismatch by 6.50
	}
	Reconcile(badInv)
	if badInv.Reconciliation.Passed || len(badInv.Warnings) == 0 {
		t.Fatalf("expected reconciliation failure on GrandTotal mismatch")
	}
}

func TestAllocateCommissionsExactConservation(t *testing.T) {
	// Telda P1: EGAL TotalFees 2.68 across 2 fills -> 1.23 and 1.45
	teldaP1 := &model.InvoicePage{
		Fills: []model.Fill{
			{Quantity: 1, Price: 301.56, Value: 301.56},
			{Quantity: 2, Price: 301.51, Value: 603.02},
		},
		Fees:      []model.FeeItem{{Name: "EFSA Fees", Amount: 2.00}},
		TotalFees: 2.68,
	}
	cTelda := AllocateCommissions(teldaP1)
	if len(cTelda) != 2 || cTelda[0] != 1.23 || cTelda[1] != 1.45 {
		t.Fatalf("Telda P1 allocation = %v, want [1.23 1.45]", cTelda)
	}

	// Thndr P1: BONY TotalFees 6.17 across 2 fills -> 3.64 and 2.53
	thndrP1 := &model.InvoicePage{
		Fills: []model.Fill{
			{Quantity: 190, Price: 4.13, Value: 784.70},
			{Quantity: 110, Price: 4.13, Value: 454.30},
		},
		Fees:      []model.FeeItem{{Name: "FRA Services", Amount: 2.00}},
		TotalFees: 6.17,
	}
	cThndr := AllocateCommissions(thndrP1)
	if len(cThndr) != 2 || cThndr[0] != 3.64 || cThndr[1] != 2.53 {
		t.Fatalf("Thndr P1 allocation = %v, want [3.64 2.53]", cThndr)
	}

	// 3-way even split of 1.00 -> 0.34, 0.33, 0.33 (exact cent sum 1.00)
	threeWay := &model.InvoicePage{
		Fills: []model.Fill{
			{Quantity: 10, Price: 10, Value: 100},
			{Quantity: 10, Price: 10, Value: 100},
			{Quantity: 10, Price: 10, Value: 100},
		},
		TotalFees: 1.00,
	}
	cThree := AllocateCommissions(threeWay)
	if len(cThree) != 3 || cThree[0] != 0.34 || cThree[1] != 0.33 || cThree[2] != 0.33 {
		t.Fatalf("3-way allocation = %v, want [0.34 0.33 0.33]", cThree)
	}
	sum := cThree[0] + cThree[1] + cThree[2]
	if math.Abs(sum-1.00) > 1e-9 {
		t.Fatalf("sum = %v, want 1.00", sum)
	}
}

func TestToCSVRecordsFillAndOrderGranularity(t *testing.T) {
	inv := &model.InvoicePage{
		SourceFile:     "telda.pdf",
		PageNumber:     1,
		Broker:         model.BrokerTelda,
		TradeDate:      time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC),
		RawSymbolCode:  "EGAL",
		ResolvedSymbol: "EGX:EGAL",
		Side:           model.SideSell,
		Fills: []model.Fill{
			{TransactionNo: "TXN000000001", Quantity: 1, Price: 301.56, Value: 301.56},
			{TransactionNo: "TXN000000002", Quantity: 2, Price: 301.51, Value: 603.02},
		},
		TotalQuantity: 3,
		TotalValue:    904.58,
		Fees:          []model.FeeItem{{Name: "EFSA Fees", Amount: 2.00}},
		TotalFees:     2.68,
	}

	fillRows, err := ToCSVRecords(inv, "fill", "00:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(fillRows) != 2 {
		t.Fatalf("expected 2 fill rows, got %d", len(fillRows))
	}
	if *fillRows[0].Commission != 1.23 || *fillRows[1].Commission != 1.45 {
		t.Fatalf("unexpected fill commissions: %v, %v", *fillRows[0].Commission, *fillRows[1].Commission)
	}

	orderRows, err := ToCSVRecords(inv, "order", "09:30:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(orderRows) != 1 {
		t.Fatalf("expected 1 order row, got %d", len(orderRows))
	}
	if orderRows[0].Qty != 3 || *orderRows[0].FillPrice != 301.5267 || *orderRows[0].Commission != 2.68 {
		t.Fatalf("unexpected order row: %+v (fp=%v, comm=%v)", orderRows[0], *orderRows[0].FillPrice, *orderRows[0].Commission)
	}
	if orderRows[0].ClosingTime.Format("2006-01-02 15:04:05") != "2026-11-23 09:30:00" {
		t.Fatalf("unexpected closing time: %s", orderRows[0].ClosingTime)
	}
}
