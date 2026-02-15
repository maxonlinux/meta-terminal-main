package trades

import (
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// TradeBuffer stores a rolling window of recent trades.
type TradeBuffer struct {
	buffer [constants.TRADE_BUFFER_SIZE]types.Trade
	head   int
	size   int
}

// TradeFeed stores rolling public trades per market.
type TradeFeed struct {
	// mu guards trade buffers by market.
	mu      sync.RWMutex
	buffers map[int8]map[string]*TradeBuffer
}

// NewTradeFeed creates a trade feed with per-market buffers.
func NewTradeFeed() *TradeFeed {
	return &TradeFeed{
		buffers: make(map[int8]map[string]*TradeBuffer),
	}
}

// Add stores a trade in the rolling buffer for category+symbol.
func (t *TradeFeed) Add(category int8, symbol string, trade types.Trade) {
	t.mu.Lock()
	defer t.mu.Unlock()
	categoryBuffers := t.buffers[category]
	if categoryBuffers == nil {
		categoryBuffers = make(map[string]*TradeBuffer)
		t.buffers[category] = categoryBuffers
	}
	buffer := categoryBuffers[symbol]
	if buffer == nil {
		buffer = &TradeBuffer{}
		categoryBuffers[symbol] = buffer
	}
	buffer.Add(trade)
}

// Recent returns trades for category+symbol in chronological order.
func (t *TradeFeed) Recent(category int8, symbol string) []types.Trade {
	t.mu.RLock()
	defer t.mu.RUnlock()
	categoryBuffers := t.buffers[category]
	if categoryBuffers == nil {
		return nil
	}
	buffer := categoryBuffers[symbol]
	if buffer == nil {
		return nil
	}
	return buffer.Recent()
}

// Add stores a trade in the rolling buffer.
func (t *TradeBuffer) Add(trade types.Trade) {
	t.buffer[t.head] = trade
	t.head = (t.head + 1) % constants.TRADE_BUFFER_SIZE
	if t.size < constants.TRADE_BUFFER_SIZE {
		t.size++
	}
}

// Recent returns trades in chronological order.
func (t *TradeBuffer) Recent() []types.Trade {
	if t.size == 0 {
		return nil
	}

	trades := make([]types.Trade, t.size)
	start := t.head - t.size
	if start < 0 {
		start += constants.TRADE_BUFFER_SIZE
	}
	for i := 0; i < t.size; i++ {
		idx := (start + i) % constants.TRADE_BUFFER_SIZE
		trades[i] = t.buffer[idx]
	}
	return trades
}
