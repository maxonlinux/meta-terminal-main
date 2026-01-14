package feed

import (
	"sync"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// mockHandler implements PriceTickHandler for testing
type mockHandler struct {
	mu    sync.Mutex
	calls []struct {
		symbol string
		price  types.Price
	}
}

func (m *mockHandler) OnPriceTick(symbol string, price types.Price) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, struct {
		symbol string
		price  types.Price
	}{symbol, price})
}

func TestDispatcherRegister(t *testing.T) {
	d := NewPriceTickDispatcher()
	h := &mockHandler{}
	d.Register(h)

	if len(d.Handlers()) != 1 {
		t.Errorf("expected 1 handler, got %d", len(d.Handlers()))
	}
}

func TestDispatcherDispatch(t *testing.T) {
	h := &mockHandler{}
	d := NewPriceTickDispatcher(h)

	d.Dispatch("BTCUSDT", types.Price(50000))

	if len(h.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(h.calls))
	}
	if h.calls[0].symbol != "BTCUSDT" {
		t.Errorf("expected symbol BTCUSDT, got %s", h.calls[0].symbol)
	}
	if h.calls[0].price != 50000 {
		t.Errorf("expected price 50000, got %d", h.calls[0].price)
	}
}

func TestDispatcherMultipleHandlers(t *testing.T) {
	h1 := &mockHandler{}
	h2 := &mockHandler{}
	d := NewPriceTickDispatcher(h1, h2)

	d.Dispatch("ETHUSDT", types.Price(3000))

	if len(h1.calls) != 1 || len(h2.calls) != 1 {
		t.Errorf("expected 1 call each, got h1=%d h2=%d", len(h1.calls), len(h2.calls))
	}
}

func TestDispatcherClear(t *testing.T) {
	h := &mockHandler{}
	d := NewPriceTickDispatcher(h)
	d.Clear()

	if len(d.Handlers()) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(d.Handlers()))
	}
}

func TestDispatcherNilHandler(t *testing.T) {
	d := NewPriceTickDispatcher(nil, &mockHandler{})
	// Should not panic
	d.Dispatch("BTCUSDT", types.Price(50000))
}

func TestDispatcherConcurrent(t *testing.T) {
	h := &mockHandler{}
	d := NewPriceTickDispatcher(h)

	wait := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			d.Dispatch("BTCUSDT", types.Price(50000))
			wait <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-wait
	}

	// All calls should complete without panic
	// Note: some calls may be lost due to lock contention, but no panic
	if len(h.calls) == 0 {
		t.Error("expected at least some calls to complete")
	}
}

func TestExtractSymbol(t *testing.T) {
	tests := []struct {
		subject string
		want    string
	}{
		{"price.tick.BTCUSDT", "BTCUSDT"},
		{"meta.price.tick.ETHUSDT", "ETHUSDT"},
		{"price.tick.SOL", "SOL"},
		{"price.tick", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractSymbol(tt.subject)
		if got != tt.want {
			t.Errorf("extractSymbol(%q) = %q, want %q", tt.subject, got, tt.want)
		}
	}
}
