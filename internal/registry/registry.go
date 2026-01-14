package registry

import (
	"sync"

	sym "github.com/maxonlinux/meta-terminal-go/pkg/symbol"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// Instrument represents a trading instrument with precision settings
type Instrument struct {
	Symbol            string
	BaseAsset         string
	QuoteAsset        string
	MinPrice          float64
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          int64
	StepSize          int64
	MinQty            int64
	MinNotional       int64

	UpdatedAt uint64 // Timestamp of last update
}

// Registry stores instruments and their price data
type Registry struct {
	mu sync.RWMutex
	// symbol -> Instrument
	instr map[string]*Instrument
	// symbol -> Price (int64)
	prices map[string]types.Price
}

// New creates a new Registry instance
func New() *Registry {
	return &Registry{
		instr:  make(map[string]*Instrument),
		prices: make(map[string]types.Price),
	}
}

// Add inserts an instrument into the registry
func (r *Registry) Add(inst *Instrument) {
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
			r.instr[s] = &Instrument{
				Symbol:     s,
				BaseAsset:  sym.GetBaseAsset(s),
				QuoteAsset: sym.GetQuoteAsset(s),
			}
		}
	}
}

// Get retrieves an instrument by symbol
func (r *Registry) GetInstrument(symbol string) *Instrument {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instr[symbol]
}

// SetPrice updates the price tick for a symbol
func (r *Registry) SetPrice(symbol string, tick types.Price) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices[symbol] = tick
}

// Price retrieves the current price tick for a symbol
func (r *Registry) GetPrice(symbol string) (types.Price, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tick, ok := r.prices[symbol]
	return tick, ok
}
