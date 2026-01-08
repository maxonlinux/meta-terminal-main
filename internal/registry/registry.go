package registry

import (
	"strings"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Instrument struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	Category   int8
}

type Registry struct {
	mu          sync.RWMutex
	instruments map[string]*Instrument
	lastPrices  map[string]types.Price
}

func New() *Registry {
	return &Registry{
		instruments: make(map[string]*Instrument),
		lastPrices:  make(map[string]types.Price),
	}
}

func (r *Registry) SetInstrument(inst *Instrument) {
	r.mu.Lock()
	r.instruments[inst.Symbol] = inst
	r.mu.Unlock()
}

func (r *Registry) GetInstrument(symbol string) *Instrument {
	r.mu.RLock()
	inst := r.instruments[symbol]
	r.mu.RUnlock()
	if inst != nil {
		return inst
	}
	return FromSymbol(symbol)
}

func (r *Registry) SetLastPrice(symbol string, price types.Price) {
	r.mu.Lock()
	r.lastPrices[symbol] = price
	r.mu.Unlock()
}

func (r *Registry) LastPrice(symbol string) (types.Price, bool) {
	r.mu.RLock()
	p, ok := r.lastPrices[symbol]
	r.mu.RUnlock()
	return p, ok
}

func (r *Registry) Instruments() map[string]*Instrument {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*Instrument, len(r.instruments))
	for k, v := range r.instruments {
		if v == nil {
			continue
		}
		cp := *v
		out[k] = &cp
	}
	return out
}

func (r *Registry) Prices() map[string]types.Price {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]types.Price, len(r.lastPrices))
	for k, v := range r.lastPrices {
		out[k] = v
	}
	return out
}

func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instruments = make(map[string]*Instrument)
	r.lastPrices = make(map[string]types.Price)
}

func FromSymbol(symbol string) *Instrument {
	quotes := []string{"USDT", "USD", "BTC", "ETH"}
	for _, q := range quotes {
		if strings.HasSuffix(symbol, q) && len(symbol) > len(q) {
			return &Instrument{
				Symbol:     symbol,
				BaseAsset:  strings.TrimSuffix(symbol, q),
				QuoteAsset: q,
			}
		}
	}
	return &Instrument{Symbol: symbol}
}
