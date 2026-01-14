package feed

import (
	"sync"

	"github.com/maxonlinux/meta-terminal-go/internal/types"
)

// PriceTickHandler processes price updates for trading decisions
// Implementations handle conditional orders, liquidation checks, PnL updates
type PriceTickHandler interface {
	// OnPriceTick is invoked when a new price tick arrives for a symbol
	// Called with the symbol and its current market data (from NATS stream)
	OnPriceTick(symbol string, tick types.PriceTick)
}

// PriceTickDispatcher distributes price ticks to registered handlers
// Uses RWMutex for concurrent read access during price updates
type PriceTickDispatcher struct {
	mu       sync.RWMutex
	handlers []PriceTickHandler
}

// NewPriceTickDispatcher creates a dispatcher with optional initial handlers
func NewPriceTickDispatcher(handlers ...PriceTickHandler) *PriceTickDispatcher {
	return &PriceTickDispatcher{
		handlers: handlers,
	}
}

// Register adds a handler to receive price tick notifications
// Handlers are called in the order they were registered
func (d *PriceTickDispatcher) Register(h PriceTickHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, h)
}

// Dispatch sends a price tick to all registered handlers
// This is the hot path - uses read lock for concurrent access
// Handlers are called synchronously; slow handlers may delay others
func (d *PriceTickDispatcher) Dispatch(symbol string, tick types.PriceTick) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, h := range d.handlers {
		if h != nil {
			h.OnPriceTick(symbol, tick)
		}
	}
}

// Handlers returns a copy of the currently registered handlers
func (d *PriceTickDispatcher) Handlers() []PriceTickHandler {
	d.mu.RLock()
	defer d.mu.RUnlock()

	clone := make([]PriceTickHandler, len(d.handlers))
	copy(clone, d.handlers)
	return clone
}

// Clear removes all registered handlers
func (d *PriceTickDispatcher) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = d.handlers[:0]
}
