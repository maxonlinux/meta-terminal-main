package registry

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// Loader periodically syncs symbols and prices from external sources
type Loader struct {
	assetsURL string // URL to fetch symbol list from
	priceURL  string // URL template for price fetching
	registry  *Registry
	interval  time.Duration
}

// NewLoader creates a new Loader that syncs data at the specified interval
func NewLoader(assetsURL, priceURL string, r *Registry, interval time.Duration) *Loader {
	return &Loader{
		assetsURL: assetsURL,
		priceURL:  priceURL,
		registry:  r,
		interval:  interval,
	}
}

// Start begins periodic synchronization
func (l *Loader) Start() {
	l.sync()
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	for range ticker.C {
		l.sync()
	}
}

// sync fetches symbols, creates instruments, and updates prices
func (l *Loader) sync() {
	symbols, _ := l.fetchSymbols()
	l.registry.AddFromList(symbols)
	for _, s := range symbols {
		if tick, ok := l.fetchPrice(s); ok {
			l.registry.SetPrice(s, tick)
		}
	}
}

// fetchSymbols retrieves the list of available symbols from CoreURL
func (l *Loader) fetchSymbols() ([]string, error) {
	resp, err := http.Get(l.assetsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var assets []struct{ Symbol string }
	json.NewDecoder(resp.Body).Decode(&assets)
	syms := make([]string, len(assets))
	for i, a := range assets {
		syms[i] = a.Symbol
	}
	return syms, nil
}

// fetchPrice retrieves the current price for a symbol from multiplexer
func (l *Loader) fetchPrice(symbol string) (types.Price, bool) {
	resp, err := http.Get(l.priceURL + "?symbol=" + symbol)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, false
	}

	var price types.Price

	json.NewDecoder(resp.Body).Decode(&price)
	return price, true
}
