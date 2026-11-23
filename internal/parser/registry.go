package parser

import (
	"fmt"

	"invoicer/internal/model"
	"invoicer/internal/pdfextract"
)

// PageParser detects and parses one brokerage invoice layout from a Page.
type PageParser interface {
	Name() model.Broker
	Detect(page *pdfextract.Page) float64
	Parse(page *pdfextract.Page, sourceFile string) (*model.InvoicePage, error)
}

// Registry maintains ordered PageParser implementations and selects the best
// match for each PDF invoice page.
type Registry struct {
	parsers []PageParser
}

// NewRegistry returns the default parser registry containing Telda, Thndr, and
// the heuristic generic fallback parser.
func NewRegistry() *Registry {
	return &Registry{
		parsers: []PageParser{
			&TeldaParser{},
			&ThndrParser{},
			&GenericBrokerParser{},
		},
	}
}

// ParsePage detects the highest-confidence parser for page and parses it.
func (r *Registry) ParsePage(page *pdfextract.Page, sourceFile string) (*model.InvoicePage, error) {
	var best PageParser
	bestScore := -1.0

	for _, p := range r.parsers {
		score := p.Detect(page)
		if score > bestScore {
			bestScore = score
			best = p
		}
	}

	if best == nil || bestScore < 0.2 {
		return nil, fmt.Errorf("%s page %d: unrecognized invoice format (no parser reached confidence threshold)", sourceFile, page.Number)
	}

	inv, err := best.Parse(page, sourceFile)
	if err != nil {
		// If a specific broker parser matched weakly or errored, try generic fallback if not already generic.
		if best.Name() != model.BrokerGeneric {
			for _, p := range r.parsers {
				if p.Name() == model.BrokerGeneric && p.Detect(page) >= 0.2 {
					if gInv, gErr := p.Parse(page, sourceFile); gErr == nil {
						gInv.Warnings = append(gInv.Warnings, fmt.Sprintf("%s parser failed (%v); fell back to generic parser", best.Name(), err))
						return gInv, nil
					}
				}
			}
		}
		return nil, err
	}
	return inv, nil
}
