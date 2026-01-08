package registry

import (
	"strings"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

type Instrument struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
}

type Registry struct {
	mu          sync.RWMutex
	instruments map[string]*Instrument
	lastPrices  map[string]domain.Price
}

func New() *Registry {
	return &Registry{
		instruments: make(map[string]*Instrument),
		lastPrices:  make(map[string]domain.Price),
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

func (r *Registry) SetLastPrice(symbol string, price domain.Price) {
	r.mu.Lock()
	r.lastPrices[symbol] = price
	r.mu.Unlock()
}

func (r *Registry) LastPrice(symbol string) (domain.Price, bool) {
	r.mu.RLock()
	p, ok := r.lastPrices[symbol]
	r.mu.RUnlock()
	return p, ok
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
