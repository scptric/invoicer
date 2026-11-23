package parser

import (
	"math"
	"testing"

	"invoicer/internal/model"
	"invoicer/internal/pdfextract"
)

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func makeCell(text string, x, endX, y float64, bold bool) pdfextract.TextCell {
	return pdfextract.TextCell{Text: text, X: x, EndX: endX, Y: y, FontSize: 9, Bold: bold}
}

func TestParseTeldaSampleInvoice(t *testing.T) {
	p1 := &pdfextract.Page{
		Number: 1,
		Width:  595.3,
		Height: 841.9,
		Rows: []pdfextract.VisualRow{
			{Y: 95.1, Cells: []pdfextract.TextCell{makeCell("Securities", 157, 245, 97, false), makeCell("Invoice", 439, 535, 93, false)}},
			{Y: 149.4, Cells: []pdfextract.TextCell{makeCell("JANE DOE", 56.7, 392, 149.4, true)}},
			{Y: 171.9, Cells: []pdfextract.TextCell{makeCell("2026-11-23", 56.7, 152, 171.9, true)}},
			{Y: 216.3, Cells: []pdfextract.TextCell{
				makeCell("Security name:", 59.7, 128, 216.3, false),
				makeCell("Symbol code:", 208.2, 269, 216.3, false),
				makeCell("Transaction type:", 356.7, 434, 216.3, false),
				makeCell("Average cost:", 449.1, 510, 216.3, false),
			}},
			{Y: 241.8, Cells: []pdfextract.TextCell{
				makeCell("Egypt Aluminum", 59.7, 143, 241.8, true),
				makeCell("EGAL", 208.2, 234, 241.8, true),
				makeCell("Sell", 358.9, 377, 241.8, true),
				makeCell("EGP 300.63", 449.1, 507, 241.8, true),
			}},
			{Y: 283.1, Cells: []pdfextract.TextCell{
				makeCell("Transaction NO:", 59.7, 130, 283.1, false),
				makeCell("Quantity", 208.2, 247, 283.1, false),
				makeCell("Price", 356.7, 378, 283.1, false),
				makeCell("Value", 449.1, 473, 283.1, false),
			}},
			{Y: 309.9, Cells: []pdfextract.TextCell{
				makeCell("TXN000000001", 56.7, 125, 309.9, true),
				makeCell("1", 205.2, 211, 309.9, true),
				makeCell("EGP 301.56", 353.7, 411, 309.9, true),
				makeCell("EGP 301.56", 446.1, 504, 309.9, true),
			}},
			{Y: 338.4, Cells: []pdfextract.TextCell{
				makeCell("TXN000000002", 56.7, 125, 338.4, true),
				makeCell("2", 205.2, 211, 338.4, true),
				makeCell("EGP 301.51", 353.7, 411, 338.4, true),
				makeCell("EGP 603.02", 446.1, 504, 338.4, true),
			}},
			{Y: 380.6, Cells: []pdfextract.TextCell{makeCell("Fees", 208.2, 228, 380.6, false), makeCell("Amount", 449.1, 484, 380.6, false)}},
			{Y: 405.7, Cells: []pdfextract.TextCell{makeCell("Telda Fees", 205.2, 251, 405.7, false), makeCell("EGP 2.00", 446.1, 491, 405.7, true)}},
			{Y: 430.5, Cells: []pdfextract.TextCell{makeCell("EGX Fees", 205.2, 247, 430.5, false), makeCell("EGP 0.09", 446.1, 491, 430.5, true)}},
			{Y: 455.2, Cells: []pdfextract.TextCell{makeCell("MCDR Fees", 205.2, 255, 455.2, false), makeCell("EGP 0.09", 446.1, 491, 455.2, true)}},
			{Y: 480.0, Cells: []pdfextract.TextCell{makeCell("EFSA Fees", 205.2, 251, 480.0, false), makeCell("EGP 2.00", 446.1, 491, 480.0, true)}},
			{Y: 504.7, Cells: []pdfextract.TextCell{makeCell("Risk Insurance Fees", 205.2, 294, 504.7, false), makeCell("EGP 0.05", 446.1, 491, 504.7, true)}},
			{Y: 529.5, Cells: []pdfextract.TextCell{makeCell("Trading Stamp Fees", 205.2, 293, 529.5, false), makeCell("EGP 0.45", 446.1, 491, 529.5, true)}},
			{Y: 554.2, Cells: []pdfextract.TextCell{makeCell("Telda Incentive", 205.2, 272, 554.2, false), makeCell("EGP -2.00", 446.1, 495, 554.2, true)}},
			{Y: 579.0, Cells: []pdfextract.TextCell{makeCell("Total Fees", 205.2, 249, 579.0, false), makeCell("EGP 2.68", 446.1, 491, 579.0, true)}},
			{Y: 613.6, Cells: []pdfextract.TextCell{makeCell("GRAND TOTAL", 205.2, 288, 613.6, true), makeCell("EGP 901.90", 446.1, 513, 613.6, true)}},
			{Y: 758.1, Cells: []pdfextract.TextCell{makeCell("www.telda.app", 424.4, 535, 758.1, false)}},
		},
	}

	reg := NewRegistry()
	inv1, err := reg.ParsePage(p1, "telda.pdf")
	if err != nil {
		t.Fatalf("ParsePage(1): %v", err)
	}
	if inv1.Broker != model.BrokerTelda {
		t.Errorf("broker = %q, want telda", inv1.Broker)
	}
	if inv1.CustomerName != "JANE DOE" {
		t.Errorf("customer = %q", inv1.CustomerName)
	}
	if inv1.TradeDate.Format("2006-01-02") != "2026-11-23" {
		t.Errorf("date = %s", inv1.TradeDate.Format("2006-11-02"))
	}
	if inv1.SecurityName != "Egypt Aluminum" || inv1.RawSymbolCode != "EGAL" || inv1.Side != model.SideSell {
		t.Errorf("security mismatch: name=%q sym=%q side=%q", inv1.SecurityName, inv1.RawSymbolCode, inv1.Side)
	}
	if !almostEqual(inv1.AverageCost, 300.63, 1e-6) {
		t.Errorf("avgCost = %v", inv1.AverageCost)
	}
	if len(inv1.Fills) != 2 {
		t.Fatalf("fills len = %d, want 2", len(inv1.Fills))
	}
	if inv1.Fills[0].TransactionNo != "TXN000000001" || inv1.Fills[0].Quantity != 1 || !almostEqual(inv1.Fills[0].Price, 301.56, 1e-6) {
		t.Errorf("fill[0] = %+v", inv1.Fills[0])
	}
	if inv1.Fills[1].TransactionNo != "TXN000000002" || inv1.Fills[1].Quantity != 2 || !almostEqual(inv1.Fills[1].Price, 301.51, 1e-6) {
		t.Errorf("fill[1] = %+v", inv1.Fills[1])
	}
	if len(inv1.Fees) != 7 {
		t.Errorf("fees count = %d, want 7", len(inv1.Fees))
	}
	var foundIncentive bool
	for _, fi := range inv1.Fees {
		if fi.Name == "Telda Incentive" && almostEqual(fi.Amount, -2.00, 1e-6) {
			foundIncentive = true
		}
	}
	if !foundIncentive {
		t.Errorf("missing negative Telda Incentive fee: %+v", inv1.Fees)
	}
	if !almostEqual(inv1.TotalFees, 2.68, 1e-6) || !almostEqual(inv1.GrandTotal, 901.90, 1e-6) {
		t.Errorf("totals mismatch: totalFees=%v grandTotal=%v", inv1.TotalFees, inv1.GrandTotal)
	}
}

func TestParseThndrMultiLineSecurityAndWrappedFees(t *testing.T) {
	page := &pdfextract.Page{
		Number: 1,
		Width:  612,
		Height: 792,
		Rows: []pdfextract.VisualRow{
			{Y: 101.3, Cells: []pdfextract.TextCell{makeCell("Invoice", 449.2, 534, 101.3, true)}},
			{Y: 150.0, Cells: []pdfextract.TextCell{
				makeCell("Jane Doe", 78, 239, 150.0, false),
				makeCell("Custodian: Thndr Technology Holding", 366.1, 534, 150.0, false),
			}},
			{Y: 160.5, Cells: []pdfextract.TextCell{makeCell("23/11/2026", 78, 130, 160.5, false)}},
			{Y: 195.8, Cells: []pdfextract.TextCell{
				makeCell("Security Name", 81.8, 148, 195.8, false),
				makeCell("Symbol Code", 219.3, 278.7, 195.8, false),
				makeCell("Transaction Type", 325.5, 400.5, 195.8, false),
				makeCell("Average Cost", 470.4, 530.2, 195.8, false),
			}},
			{Y: 213.8, Cells: []pdfextract.TextCell{makeCell("Bonyan for", 81.8, 137.2, 213.8, true)}},
			{Y: 224.3, Cells: []pdfextract.TextCell{
				makeCell("Development and", 81.8, 171.3, 224.3, true),
				makeCell("EGS656M1C010", 209.3, 288.7, 224.3, true),
				makeCell("Sell", 353.6, 372.4, 224.3, true),
				makeCell("4.11 EGP", 484.8, 530.2, 224.3, true),
			}},
			{Y: 234.8, Cells: []pdfextract.TextCell{makeCell("Trade", 81.8, 110.0, 234.8, true)}},
			{Y: 275.3, Cells: []pdfextract.TextCell{
				makeCell("Transaction No.", 81.8, 151.0, 275.3, false),
				makeCell("Quantity", 229.5, 268.4, 275.3, false),
				makeCell("Price", 352.0, 374.0, 275.3, false),
				makeCell("Value", 505.5, 530.2, 275.3, false),
			}},
			{Y: 293.1, Cells: []pdfextract.TextCell{
				makeCell("TXN000000101", 81.8, 123.1, 293.1, true),
				makeCell("190.00", 231.6, 266.4, 293.1, true),
				makeCell("4.13 EGP", 340.3, 385.7, 293.1, true),
				makeCell("784.70 EGP", 472.3, 530.2, 293.1, true),
			}},
			{Y: 311.1, Cells: []pdfextract.TextCell{
				makeCell("TXN000000102", 81.8, 123.1, 311.1, true),
				makeCell("110.00", 231.6, 266.4, 311.1, true),
				makeCell("4.13 EGP", 340.3, 385.7, 311.1, true),
				makeCell("454.30 EGP", 472.3, 530.2, 311.1, true),
			}},
			{Y: 330.0, Cells: []pdfextract.TextCell{
				makeCell("Total Quantity", 217.6, 280.4, 330.0, false),
				makeCell("Average Price", 332.1, 393.9, 330.0, false),
				makeCell("Total Cost", 486.4, 530.2, 330.0, false),
			}},
			{Y: 348.0, Cells: []pdfextract.TextCell{
				makeCell("300.00", 231.6, 266.4, 348.0, true),
				makeCell("4.13 EGP", 340.3, 385.7, 348.0, true),
				makeCell("1,239.00 EGP", 462.6, 530.3, 348.0, true),
			}},
			{Y: 388.5, Cells: []pdfextract.TextCell{makeCell("Fees", 351.3, 374.7, 388.5, true), makeCell("Amount", 490.6, 530.2, 388.5, true)}},
			{Y: 406.5, Cells: []pdfextract.TextCell{makeCell("EGX Services", 329.8, 396.2, 406.5, true), makeCell("0.13 EGP", 484.8, 530.2, 406.5, true)}},
			{Y: 424.5, Cells: []pdfextract.TextCell{makeCell("MCDR Services", 325.1, 400.9, 424.5, true), makeCell("0.13 EGP", 484.8, 530.2, 424.5, true)}},
			{Y: 442.5, Cells: []pdfextract.TextCell{makeCell("FRA Services", 330.0, 396.0, 442.5, true), makeCell("2.00 EGP", 484.8, 530.2, 442.5, true)}},
			{Y: 460.5, Cells: []pdfextract.TextCell{makeCell("Risk Insurance", 325.8, 400.2, 460.5, true), makeCell("0.06 EGP", 484.8, 530.2, 460.5, true)}},
			{Y: 478.5, Cells: []pdfextract.TextCell{makeCell("Trading Damgha", 321.5, 404.5, 478.5, true), makeCell("0.62 EGP", 484.8, 530.2, 478.5, true)}},
			{Y: 496.5, Cells: []pdfextract.TextCell{makeCell("Brokerage &", 331.3, 394.7, 496.5, true)}},
			{Y: 501.8, Cells: []pdfextract.TextCell{makeCell("1.23 EGP", 484.8, 530.2, 501.8, true)}},
			{Y: 507.0, Cells: []pdfextract.TextCell{makeCell("Custody Fees", 329.2, 396.8, 507.0, true)}},
			{Y: 525.0, Cells: []pdfextract.TextCell{makeCell("Brokerage Order", 320.7, 405.3, 525.0, true)}},
			{Y: 530.3, Cells: []pdfextract.TextCell{makeCell("2.00 EGP", 484.8, 530.2, 530.3, true)}},
			{Y: 535.5, Cells: []pdfextract.TextCell{makeCell("Fees", 351.3, 374.7, 535.5, true)}},
			{Y: 553.5, Cells: []pdfextract.TextCell{makeCell("Total Fees", 337.5, 388.5, 553.5, true), makeCell("6.17 EGP", 484.8, 530.2, 553.5, true)}},
			{Y: 573.8, Cells: []pdfextract.TextCell{makeCell("Grand Total", 328.9, 397.1, 573.8, true), makeCell("1,232.83 EGP", 451.3, 530.2, 573.8, true)}},
			{Y: 607.5, Cells: []pdfextract.TextCell{makeCell("Note: The text in this invoice is standardized.", 78.0, 523.2, 607.5, true)}},
		},
	}

	reg := NewRegistry()
	inv, err := reg.ParsePage(page, "thndr.pdf")
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	if inv.Broker != model.BrokerThndr {
		t.Errorf("broker = %q, want thndr", inv.Broker)
	}
	if inv.SecurityName != "Bonyan for Development and Trade" {
		t.Errorf("securityName = %q", inv.SecurityName)
	}
	if inv.RawSymbolCode != "EGS656M1C010" || inv.Side != model.SideSell {
		t.Errorf("meta mismatch: sym=%q side=%q", inv.RawSymbolCode, inv.Side)
	}
	if inv.TradeDate.Format("2006-01-02") != "2026-11-23" {
		t.Errorf("date = %s", inv.TradeDate.Format("2006-01-02"))
	}
	if len(inv.Fills) != 2 || !almostEqual(inv.TotalQuantity, 300, 1e-6) || !almostEqual(inv.TotalValue, 1239.00, 1e-6) {
		t.Errorf("fills/summary mismatch: fills=%+v qty=%v val=%v", inv.Fills, inv.TotalQuantity, inv.TotalValue)
	}
	if len(inv.Fees) != 7 {
		t.Errorf("fees len = %d, want 7", len(inv.Fees))
	}
	var sawCustody, sawOrder bool
	for _, f := range inv.Fees {
		if f.Name == "Brokerage & Custody Fees" && almostEqual(f.Amount, 1.23, 1e-6) {
			sawCustody = true
		}
		if f.Name == "Brokerage Order Fees" && almostEqual(f.Amount, 2.00, 1e-6) {
			sawOrder = true
		}
	}
	if !sawCustody || !sawOrder {
		t.Errorf("wrapped fee names not parsed cleanly: %+v", inv.Fees)
	}
	if !almostEqual(inv.TotalFees, 6.17, 1e-6) || !almostEqual(inv.GrandTotal, 1232.83, 1e-6) {
		t.Errorf("totals mismatch: totalFees=%v grandTotal=%v", inv.TotalFees, inv.GrandTotal)
	}
}

func TestGenericBrokerParserFallback(t *testing.T) {
	page := &pdfextract.Page{
		Number: 1,
		Width:  600,
		Height: 800,
		Rows: []pdfextract.VisualRow{
			{Y: 50, Cells: []pdfextract.TextCell{{Text: "Trade Confirmation", X: 40, EndX: 180}}},
			{Y: 70, Cells: []pdfextract.TextCell{{Text: "Date: 15/11/2026", X: 40, EndX: 160}, {Text: "Side: Buy", X: 300, EndX: 380}}},
			{Y: 90, Cells: []pdfextract.TextCell{{Text: "Security Name: Commercial International Bank", X: 40, EndX: 280}}},
			{Y: 110, Cells: []pdfextract.TextCell{{Text: "ISIN: EGS60121C018", X: 40, EndX: 200}}},
			{Y: 140, Cells: []pdfextract.TextCell{
				{Text: "Order No", X: 40, EndX: 100},
				{Text: "Qty", X: 180, EndX: 210},
				{Text: "Fill Price", X: 280, EndX: 340},
				{Text: "Amount", X: 420, EndX: 480},
			}},
			{Y: 160, Cells: []pdfextract.TextCell{
				{Text: "ORD123456", X: 40, EndX: 110},
				{Text: "10", X: 180, EndX: 200},
				{Text: "EGP 85.50", X: 280, EndX: 340},
				{Text: "EGP 855.00", X: 420, EndX: 490},
			}},
			{Y: 200, Cells: []pdfextract.TextCell{{Text: "Total Commission", X: 280, EndX: 380}, {Text: "EGP 4.50", X: 420, EndX: 480}}},
			{Y: 220, Cells: []pdfextract.TextCell{{Text: "Grand Total", X: 280, EndX: 360}, {Text: "EGP 859.50", X: 420, EndX: 495}}},
		},
	}

	reg := NewRegistry()
	inv, err := reg.ParsePage(page, "synthetic.pdf")
	if err != nil {
		t.Fatalf("ParsePage(generic): %v", err)
	}
	if inv.Broker != model.BrokerGeneric {
		t.Errorf("broker = %q, want generic", inv.Broker)
	}
	if inv.RawSymbolCode != "EGS60121C018" || inv.Side != model.SideBuy {
		t.Errorf("meta mismatch: sym=%q side=%q", inv.RawSymbolCode, inv.Side)
	}
	if len(inv.Fills) != 1 || inv.Fills[0].Quantity != 10 || !almostEqual(inv.Fills[0].Price, 85.50, 1e-6) {
		t.Errorf("fills mismatch: %+v", inv.Fills)
	}
	if !almostEqual(inv.TotalFees, 4.50, 1e-6) || !almostEqual(inv.GrandTotal, 859.50, 1e-6) {
		t.Errorf("totals mismatch: totalFees=%v grandTotal=%v", inv.TotalFees, inv.GrandTotal)
	}
	if len(inv.Warnings) == 0 {
		t.Error("expected generic fallback warning")
	}
}
