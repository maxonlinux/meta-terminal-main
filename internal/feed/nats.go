package feed

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/types"
	"github.com/nats-io/nats.go"
)

// NATSConfig holds configuration for NATS price feed connection
type NATSConfig struct {
	URL    string // NATS server URL, e.g., "nats://localhost:4222"
	Stream string // Subject prefix, e.g., "meta" → subscribes to "meta.price.tick.*"
}

// NATSFeed subscribes to price tick topics and dispatches to handlers
// Supports automatic reconnection and clean shutdown
type NATSFeed struct {
	conn       *nats.Conn
	registry   *registry.Registry
	dispatcher *PriceTickDispatcher
	subject    string // Full subject pattern, e.g., "price.tick.*"
}

// NewNATSFeed creates a new NATS price feed subscriber
// Returns error if connection fails; caller should handle retry
func NewNATSFeed(cfg NATSConfig, r *registry.Registry, d *PriceTickDispatcher) (*NATSFeed, error) {
	// Connect to NATS with automatic reconnection
	conn, err := nats.Connect(cfg.URL,
		nats.MaxReconnects(-1), // Unlimited reconnects
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, err
	}

	// Build subject pattern: if Stream is "meta", subject is "meta.price.tick.*"
	subject := cfg.Stream
	if subject != "" {
		subject += "."
	}
	subject += "price.tick.*"

	return &NATSFeed{
		conn:       conn,
		registry:   r,
		dispatcher: d,
		subject:    subject,
	}, nil
}

// Start begins subscribing to price tick topics
// Returns error if subscription fails; connection errors are handled internally
func (f *NATSFeed) Start() error {
	_, err := f.conn.Subscribe(f.subject, f.handleMessage)
	return err
}

// Stop closes the NATS connection and cleans up resources
// Call this during shutdown to ensure clean exit
func (f *NATSFeed) Stop() {
	if f.conn != nil {
		f.conn.Close()
	}
}

// handleMessage processes incoming NATS messages
// Parses symbol from subject, updates registry, dispatches to handlers
func (f *NATSFeed) handleMessage(msg *nats.Msg) {
	// Extract symbol from subject: "price.tick.BTCUSDT" → "BTCUSDT"
	symbol := extractSymbol(msg.Subject)
	if symbol == "" {
		return // Invalid subject format
	}

	// Parse price tick from message data (JSON)
	var tick types.PriceTick
	if err := json.Unmarshal(msg.Data, &tick); err != nil {
		// Log error in production; skip invalid messages
		return
	}
	tick.Symbol = symbol

	// Update registry with latest price data
	f.registry.PriceTick(symbol, tick)

	// Dispatch to all registered handlers (triggers, risk, portfolio)
	f.dispatcher.Dispatch(symbol, tick)
}

// extractSymbol parses the symbol from a NATS subject
// Expected format: "prefix.price.tick.SYMBOL" → returns SYMBOL
// Returns empty string for invalid subjects (not enough parts or wrong pattern)
// Uses Cut for zero-allocation parsing when possible
func extractSymbol(subject string) string {
	// Fast path: try to cut from the end using strings.Cut
	// This avoids creating a full slice allocation like strings.Split does
	_, symbol, found := strings.Cut(subject, "price.tick.")
	if !found {
		// Try alternative format without prefix
		_, symbol, found = strings.Cut(subject, "tick.")
		if !found {
			return ""
		}
		return symbol
	}

	// Verify it's not just "price.tick" without a symbol
	if symbol == "" {
		return ""
	}
	return symbol
}
