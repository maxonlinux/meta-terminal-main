package registry

import (
	"sync"
)

type Instrument struct {
	Symbol     string
	Category   int8
	BaseAsset  string
	QuoteAsset string
	TickSize   int64
	LotSize    int64
	MinQty     int64
}

type PriceTick struct {
	Price     int64
	Bid       int64
	Ask       int64
	Volume    int64
	Timestamp int64
}

type Registry struct {
	mu          sync.RWMutex
	instruments map[string]*Instrument
	prices      map[string]PriceTick
}

func New() *Registry {
	return &Registry{
		instruments: make(map[string]*Instrument),
		prices:      make(map[string]PriceTick),
	}
}

func (r *Registry) SetInstrument(symbol string, inst *Instrument) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instruments[symbol] = inst
}

func (r *Registry) GetInstrument(symbol string) (*Instrument, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.instruments[symbol]
	return inst, ok
}

func (r *Registry) SetPrice(symbol string, tick PriceTick) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices[symbol] = tick
}

func (r *Registry) GetPrice(symbol string) (PriceTick, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tick, ok := r.prices[symbol]
	return tick, ok
}

func (r *Registry) GetAllInstruments() map[string]*Instrument {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*Instrument, len(r.instruments))
	for k, v := range r.instruments {
		result[k] = v
	}
	return result
}

func (r *Registry) GetAllPrices() map[string]PriceTick {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]PriceTick, len(r.prices))
	for k, v := range r.prices {
		result[k] = v
	}
	return result
}
