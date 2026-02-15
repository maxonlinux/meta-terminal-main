package registry

import (
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type PriceTick struct {
	Price     types.Price
	Timestamp uint64
}

type Registry struct {
	mu          sync.RWMutex
	instruments map[string]*types.Instrument
	prices      map[string]PriceTick
}

func New() *Registry {
	return &Registry{
		instruments: make(map[string]*types.Instrument),
		prices:      make(map[string]PriceTick),
	}
}

func (r *Registry) SetInstrument(symbol string, inst *types.Instrument) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instruments[symbol] = inst
}

func (r *Registry) GetInstrument(symbol string) *types.Instrument {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instruments[symbol]
}

func (r *Registry) GetInstruments() []*types.Instrument {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*types.Instrument, 0, len(r.instruments))
	for _, inst := range r.instruments {
		out = append(out, inst)
	}
	return out
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

func (r *Registry) GetPrices() map[string]PriceTick {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]PriceTick, len(r.prices))
	for k, v := range r.prices {
		out[k] = v
	}
	return out
}
