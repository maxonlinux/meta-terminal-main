package registry

import (
	"encoding/json"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/types"
	sym "github.com/maxonlinux/meta-terminal-go/pkg/symbol"
)

// PriceBand defines trading constraints based on price range
type PriceBand struct {
	MinPrice          float64
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          int64
	StepSize          int64
	MinQty            int64
	MinNotional       int64
}

// Bands define price filter tiers for different price ranges
// Price ranges are inclusive of the minimum, exclusive of the next band
var Bands = []PriceBand{
	{1000.0, 2, 6, 1, 1, 1, 500000000},
	{100.0, 3, 6, 1, 1, 1, 500000000},
	{1.0, 4, 6, 1, 1, 1, 500000000},
	{0.01, 5, 0, 1, 1, 1, 500000000},
	{0.0, 8, 0, 1, 1, 1, 10000000},
}

// GetBand returns the price band for a given price level
func GetBand(price float64) *PriceBand {
	for _, b := range Bands {
		if price >= b.MinPrice {
			return &b
		}
	}
	return &Bands[len(Bands)-1]
}

// GetBandBounds returns the price band and valid price range for a price level
func GetBandBounds(price float64) (*PriceBand, int64, int64) {
	for i, b := range Bands {
		if price >= b.MinPrice {
			minPrice := int64(b.MinPrice)
			if minPrice < b.TickSize {
				minPrice = b.TickSize
			}
			return &b, minPrice, maxPriceForBand(i)
		}
	}
	last := Bands[len(Bands)-1]
	return &last, int64(last.MinPrice), math.MaxInt64
}

// maxPriceForBand returns the maximum valid price for a band
func maxPriceForBand(index int) int64 {
	if index == 0 {
		return math.MaxInt64
	}
	prev := Bands[index-1]
	max := int64(prev.MinPrice) - prev.TickSize
	if max <= 0 {
		return math.MaxInt64
	}
	return max
}

// Registry stores instruments and their price data
type Registry struct {
	mu     sync.RWMutex
	instr  map[string]*types.Instrument
	prices map[string]types.PriceTick
}

// New creates a new Registry instance
func New() *Registry {
	return &Registry{
		instr:  make(map[string]*types.Instrument),
		prices: make(map[string]types.PriceTick),
	}
}

// Add inserts an instrument into the registry
func (r *Registry) Add(inst *types.Instrument) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instr[inst.Symbol] = inst
}

// AddFromList creates instruments from a list of symbols
// Uses GetBaseAsset and GetQuoteAsset to parse symbol components
func (r *Registry) AddFromList(symbols []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range symbols {
		if r.instr[s] == nil {
			r.instr[s] = &types.Instrument{
				Symbol:     s,
				BaseAsset:  sym.GetBaseAsset(s),
				QuoteAsset: sym.GetQuoteAsset(s),
			}
		}
	}
}

// Get retrieves an instrument by symbol
func (r *Registry) Get(symbol string) *types.Instrument {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instr[symbol]
}

// SetPrice updates the price tick for a symbol
func (r *Registry) SetPrice(symbol string, tick types.PriceTick) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices[symbol] = tick
}

// PriceTick updates price data and syncs to instrument
// Called when new market data arrives from NATS or HTTP
// Updates LastPrice, Bid, Ask, Volume, UpdatedAt on the instrument
func (r *Registry) PriceTick(symbol string, tick types.PriceTick) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update price cache
	r.prices[symbol] = tick

	// Sync to instrument if it exists
	if inst := r.instr[symbol]; inst != nil {
		inst.LastPrice = tick.Price
		inst.UpdatedAt = uint64(time.Now().UnixNano())
	}
}

// GetLastPrice returns the last known price for a symbol
func (r *Registry) GetLastPrice(symbol string) (int64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tick, ok := r.prices[symbol]; ok {
		return tick.Price, true
	}
	return 0, false
}

// Price retrieves the current price tick for a symbol
func (r *Registry) Price(symbol string) (types.PriceTick, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tick, ok := r.prices[symbol]
	return tick, ok
}

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
func (l *Loader) fetchPrice(symbol string) (types.PriceTick, bool) {
	resp, err := http.Get(l.priceURL + "?symbol=" + symbol)
	if err != nil {
		return types.PriceTick{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return types.PriceTick{}, false
	}

	var p struct {
		Price  int64 `json:"p"`
		Bid    int64 `json:"b"`
		Ask    int64 `json:"a"`
		Volume int64 `json:"v"`
	}
	json.NewDecoder(resp.Body).Decode(&p)
	return types.PriceTick{
		Symbol: symbol,
		Price:  p.Price,
		Bid:    p.Bid,
		Ask:    p.Ask,
		Volume: p.Volume,
		Time:   time.Now().UnixNano(),
	}, true
}
