package symbols

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolverAllTiers(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "symbols.json")
	if err := os.WriteFile(customPath, []byte(`{
		"isin": {
			"EGS99999C019": "CUST",
			"EGS656M1C010": "BONY_OVERRIDE"
		},
		"names": {
			"Custom Corp S.A.E.": "CCRP"
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := NewResolver(Config{
		ExchangePrefix: "EGX",
		SymbolsPath:    customPath,
	})
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}

	tests := []struct {
		name         string
		raw          string
		secName      string
		wantSym      string
		wantWarning  bool
	}{
		{"already prefixed", "egx:egal", "Egypt Aluminum", "EGX:EGAL", false},
		{"direct ticker", "MPCO", "Mansoura Poultry", "EGX:MPCO", false},
		{"custom ISIN override", "EGS656M1C010", "Bonyan", "EGX:BONY_OVERRIDE", false},
		{"custom ISIN new", "EGS99999C019", "Unknown Name", "EGX:CUST", false},
		{"builtin ISIN SKPC", "EGS380S1C017", "Sidi Kerir Petrochemicals", "EGX:SKPC", false},
		{"builtin ISIN SAUD", "EGS60101C010", "Al Baraka Bank", "EGX:SAUD", false},
		{"builtin ISIN EGAL", "EGS3E181C010", "Egypt Aluminum", "EGX:EGAL", false},
		{"custom name fallback", "EGS00000C001", "Custom Corp S.A.E.", "EGX:CCRP", false},
		{"builtin name fallback", "EGS00000C002", "Bonyan for Development and Trade", "EGX:BONY", false},
		{"unknown ISIN warns", "EGS12345C019", "Mystery Co", "EGX:EGS12345C019", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, warn := res.Resolve(tc.raw, tc.secName)
			if got != tc.wantSym {
				t.Errorf("Resolve(%q, %q) = %q, want %q", tc.raw, tc.secName, got, tc.wantSym)
			}
			if tc.wantWarning && warn == "" {
				t.Errorf("expected warning for %q, got empty", tc.raw)
			}
			if !tc.wantWarning && warn != "" {
				t.Errorf("unexpected warning for %q: %s", tc.raw, warn)
			}
		})
	}
}

func TestResolverOnlineOpenFIGIFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"data": []map[string]string{{"ticker": "TEST.CA", "exchCode": "EG"}}},
		})
	}))
	defer srv.Close()

	res, err := NewResolver(Config{
		ExchangePrefix: "EGX",
		OnlineLookup:   true,
		OpenFIGIURL:    srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	sym, warn := res.Resolve("EGS88888C018", "Unseen Security")
	if sym != "EGX:TEST" || warn != "" {
		t.Fatalf("expected EGX:TEST without warning, got (%q, %q)", sym, warn)
	}
	if !strings.HasPrefix(sym, "EGX:") {
		t.Fatalf("unexpected prefix: %q", sym)
	}
}

func TestConfigSymbolsJSON(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "config", "symbols.json")
	res, err := NewResolver(Config{
		ExchangePrefix: "EGX",
		SymbolsPath:    cfgPath,
	})
	if err != nil {
		t.Fatalf("NewResolver(config/symbols.json): %v", err)
	}
	if len(res.customISIN) < 290 {
		t.Fatalf("expected >= 290 EGX ISIN entries in config/symbols.json, got %d", len(res.customISIN))
	}
	for code, ticker := range res.customISIN {
		if !isinRegex.MatchString(code) {
			t.Errorf("invalid ISIN key in config/symbols.json: %q", code)
		}
		if ticker == "" {
			t.Errorf("empty ticker for ISIN %q", code)
		}
	}

	checks := []struct {
		isin string
		want string
	}{
		{"EGS656M1C010", "EGX:BONY"},
		{"EGS380S1C017", "EGX:SKPC"},
		{"EGS60101C010", "EGX:SAUD"},
		{"EGS3E181C010", "EGX:EGAL"},
		{"EGS60121C018", "EGX:COMI"},
		{"EGS65571C019", "EGX:MASR"},
		{"EGS60041C018", "EGX:CIEB"},
		{"EGS60321C014", "EGX:FAIT"},
		{"EGS60081C014", "EGX:QNBE"},
	}
	for _, tc := range checks {
		got, warn := res.Resolve(tc.isin, "")
		if got != tc.want || warn != "" {
			t.Errorf("Resolve(%q) = (%q, %q), want (%q, \"\")", tc.isin, got, warn, tc.want)
		}
	}
}
