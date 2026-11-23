// Package model defines the domain types shared across PDF extraction,
// brokerage parsing, symbol resolution, reconciliation, and CSV formatting.
package model

import "time"

// Broker identifies the brokerage layout detected for an invoice page.
type Broker string

const (
	BrokerTelda   Broker = "telda"
	BrokerThndr   Broker = "thndr"
	BrokerGeneric Broker = "generic"
)

// Side identifies the transaction side or cash/dividend event type in CSV records.
type Side string

const (
	SideBuy          Side = "Buy"
	SideSell         Side = "Sell"
	SideDeposit      Side = "Deposit"
	SideWithdrawal   Side = "Withdrawal"
	SideTaxesAndFees Side = "Taxes and fees"
	SideDividend     Side = "Dividend"
)

// Fill represents an individual execution line inside an invoice page.
type Fill struct {
	TransactionNo string  `json:"transaction_no"`
	Quantity      float64 `json:"quantity"`
	Price         float64 `json:"price"`
	Value         float64 `json:"value"`
}

// FeeItem represents a single fee or incentive line item on an invoice page.
type FeeItem struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// ReconciliationCheck represents a single mathematical verification check.
type ReconciliationCheck struct {
	Name     string  `json:"name"`
	Expected float64 `json:"expected"`
	Actual   float64 `json:"actual"`
	Diff     float64 `json:"diff"`
	Passed   bool    `json:"passed"`
	Message  string  `json:"message,omitempty"`
}

// ReconciliationResult aggregates all mathematical checks performed on an invoice page.
type ReconciliationResult struct {
	Passed bool                  `json:"passed"`
	Checks []ReconciliationCheck `json:"checks"`
}

// InvoicePage holds all structured data extracted from a single PDF invoice page.
type InvoicePage struct {
	SourceFile     string               `json:"source_file"`
	PageNumber     int                  `json:"page_number"`
	Broker         Broker               `json:"broker"`
	CustomerName   string               `json:"customer_name,omitempty"`
	Custodian      string               `json:"custodian,omitempty"`
	TradeDate      time.Time            `json:"trade_date"`
	Currency       string               `json:"currency,omitempty"`
	SecurityName   string               `json:"security_name"`
	RawSymbolCode  string               `json:"raw_symbol_code"`
	ResolvedSymbol string               `json:"resolved_symbol"`
	Side           Side                 `json:"side"`
	AverageCost    float64              `json:"average_cost,omitempty"`
	Fills          []Fill               `json:"fills"`
	TotalQuantity  float64              `json:"total_quantity"`
	AveragePrice   float64              `json:"average_price"`
	TotalValue     float64              `json:"total_value"`
	Fees           []FeeItem            `json:"fees"`
	TotalFees      float64              `json:"total_fees"`
	GrandTotal     float64              `json:"grand_total"`
	Reconciliation ReconciliationResult `json:"reconciliation"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// CSVRecord represents one row in the target accounting CSV format.
// FillPrice and Commission use pointers so cash/dividend rows can emit empty fields.
type CSVRecord struct {
	Symbol        string    `json:"symbol"`
	Side          Side      `json:"side"`
	Qty           float64   `json:"qty"`
	FillPrice     *float64  `json:"fill_price,omitempty"`
	Commission    *float64  `json:"commission,omitempty"`
	ClosingTime   time.Time `json:"closing_time"`
	SourceFile    string    `json:"source_file,omitempty"`
	PageNumber    int       `json:"page_number,omitempty"`
	Broker        Broker    `json:"broker,omitempty"`
	TransactionNo string    `json:"transaction_no,omitempty"`
}
