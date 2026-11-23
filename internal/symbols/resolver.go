package symbols

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	isinRegex   = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{9}[0-9]$`)
	tickerRegex = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)
)

// Config configures symbol resolution behavior.
type Config struct {
	ExchangePrefix string
	SymbolsPath    string
	OnlineLookup   bool
	OpenFIGIURL    string // optional override for testing
}

// Resolver maps raw brokerage symbol codes (tickers or ISINs) and company names
// into standardized accounting symbols (e.g., EGX:BONY).
type Resolver struct {
	exchange     string
	customISIN   map[string]string
	customNames  map[string]string
	onlineLookup bool
	openFIGIURL  string
	httpClient   *http.Client
	mu           sync.RWMutex
	onlineCache  map[string]string
}

type customSymbolFile struct {
	ISIN  map[string]string `json:"isin"`
	Names map[string]string `json:"names"`
}

// NewResolver initializes a Resolver, loading custom JSON mappings when configured
// or when ./symbols.json, config/symbols.json, or <exe>/../config/symbols.json exists.
func NewResolver(cfg Config) (*Resolver, error) {
	exch := strings.ToUpper(strings.TrimSpace(cfg.ExchangePrefix))
	if exch == "" {
		exch = "EGX"
	}
	figiURL := strings.TrimSpace(cfg.OpenFIGIURL)
	if figiURL == "" {
		figiURL = "https://api.openfigi.com/v3/mapping"
	}

	r := &Resolver{
		exchange:     exch,
		customISIN:   make(map[string]string),
		customNames:  make(map[string]string),
		onlineLookup: cfg.OnlineLookup,
		openFIGIURL:  figiURL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		onlineCache:  make(map[string]string),
	}

	path := strings.TrimSpace(cfg.SymbolsPath)
	explicit := path != ""
	if path == "" {
		candidates := []string{
			"symbols.json",
			filepath.Join("config", "symbols.json"),
		}
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			candidates = append(candidates,
				filepath.Join(exeDir, "config", "symbols.json"),
				filepath.Join(exeDir, "..", "config", "symbols.json"),
			)
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}

	if path != "" {
		if err := r.loadCustomFile(path); err != nil {
			if explicit || !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	return r, nil
}

func (r *Resolver) loadCustomFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read symbols file %q: %w", path, err)
	}

	var structured customSymbolFile
	if err := json.Unmarshal(raw, &structured); err == nil && (len(structured.ISIN) > 0 || len(structured.Names) > 0) {
		for k, v := range structured.ISIN {
			r.customISIN[strings.ToUpper(strings.TrimSpace(k))] = cleanTicker(v, r.exchange)
		}
		for k, v := range structured.Names {
			r.customNames[NormalizeName(k)] = cleanTicker(v, r.exchange)
		}
		return nil
	}

	var flat map[string]string
	if err := json.Unmarshal(raw, &flat); err != nil {
		return fmt.Errorf("parse symbols file %q: %w", path, err)
	}
	for k, v := range flat {
		uk := strings.ToUpper(strings.TrimSpace(k))
		if isinRegex.MatchString(uk) {
			r.customISIN[uk] = cleanTicker(v, r.exchange)
		} else {
			r.customNames[NormalizeName(k)] = cleanTicker(v, r.exchange)
		}
	}
	return nil
}

// Resolve returns the formatted accounting symbol (e.g., EGX:EGAL) and an
// optional warning if an ISIN could not be mapped to a known ticker.
func (r *Resolver) Resolve(rawCode, securityName string) (string, string) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))

	// 1. Already exchange-prefixed (e.g., EGX:EGAL)
	if strings.Contains(code, ":") {
		return code, ""
	}

	// 2. Direct ticker code (non-ISIN)
	if code != "" && !isinRegex.MatchString(code) && tickerRegex.MatchString(code) {
		return r.formatTicker(code), ""
	}

	// 3. Custom ISIN map -> Built-in ISIN map
	if code != "" {
		if ticker, ok := r.customISIN[code]; ok && ticker != "" {
			return r.formatTicker(ticker), ""
		}
		if ticker, ok := builtinISINToTicker[code]; ok && ticker != "" {
			return r.formatTicker(ticker), ""
		}
	}

	// 4. Custom normalized name map -> Built-in normalized name map
	if norm := NormalizeName(securityName); norm != "" {
		if ticker, ok := r.customNames[norm]; ok && ticker != "" {
			return r.formatTicker(ticker), ""
		}
		if ticker, ok := builtinNameToTicker[norm]; ok && ticker != "" {
			return r.formatTicker(ticker), ""
		}
	}

	// 5. Optional OpenFIGI lookup for unknown ISINs
	if r.onlineLookup && isinRegex.MatchString(code) {
		if ticker := r.lookupOpenFIGI(code); ticker != "" {
			return r.formatTicker(ticker), ""
		}
	}

	// 6. Preserve transaction with explicit warning
	if code != "" {
		return r.formatTicker(code), fmt.Sprintf("unresolved ISIN/symbol %q (%s); add it to symbols.json", code, securityName)
	}
	return r.formatTicker("UNKNOWN"), fmt.Sprintf("missing symbol code for security %q; add it to symbols.json", securityName)
}

func (r *Resolver) formatTicker(ticker string) string {
	t := cleanTicker(ticker, r.exchange)
	if strings.Contains(t, ":") {
		return t
	}
	return r.exchange + ":" + t
}

func cleanTicker(s, exchange string) string {
	t := strings.ToUpper(strings.TrimSpace(s))
	t = strings.TrimPrefix(t, exchange+":")
	t = strings.TrimSuffix(t, ".CA")
	return strings.TrimSpace(t)
}

func (r *Resolver) lookupOpenFIGI(isin string) string {
	r.mu.RLock()
	if cached, ok := r.onlineCache[isin]; ok {
		r.mu.RUnlock()
		return cached
	}
	r.mu.RUnlock()

	type figiReq struct {
		IDType   string `json:"idType"`
		IDValue  string `json:"idValue"`
		ExchCode string `json:"exchCode,omitempty"`
	}
	type figiItem struct {
		Ticker   string `json:"ticker"`
		ExchCode string `json:"exchCode"`
	}
	type figiResp struct {
		Data []figiItem `json:"data"`
	}

	query := func(exchCode string) string {
		payload, _ := json.Marshal([]figiReq{{IDType: "ID_ISIN", IDValue: isin, ExchCode: exchCode}})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.openFIGIURL, bytes.NewReader(payload))
		if err != nil {
			return ""
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return ""
		}
		var out []figiResp
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out) == 0 || len(out[0].Data) == 0 {
			return ""
		}
		return cleanTicker(out[0].Data[0].Ticker, r.exchange)
	}

	ticker := query("EG")
	if ticker == "" {
		ticker = query("")
	}

	r.mu.Lock()
	r.onlineCache[isin] = ticker
	r.mu.Unlock()
	return ticker
}
