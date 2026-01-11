package history

import (
	"context"
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var errOutboxClosed = errors.New("history: outbox writer closed")

type OutboxWriter struct {
	outbox *outbox.Writer
	nats   *messaging.NATS
	subs   []*messaging.Subscription
	closed bool
}

type OutboxConfig struct {
	Path    string
	BufSize int
	NATS    *messaging.NATS
}

func NewOutboxWriter(cfg OutboxConfig) (*OutboxWriter, error) {
	w, err := outbox.OpenWriter(cfg.Path, cfg.BufSize)
	if err != nil {
		return nil, err
	}
	return &OutboxWriter{
		outbox: w,
		nats:   cfg.NATS,
	}, nil
}

func (w *OutboxWriter) Start(ctx context.Context) {
	if w.nats == nil {
		return
	}
	w.subs = append(w.subs, w.nats.Subscribe(ctx, messaging.OrderEventTopic("*"), "history-orders", w.handleOrderEvent))
	w.subs = append(w.subs, w.nats.Subscribe(ctx, messaging.SUBJECT_CLEARING_TRADE, "history-trades", w.handleTradeEvent))
	w.subs = append(w.subs, w.nats.Subscribe(ctx, messaging.PositionReducedTopic("*"), "history-pnl", w.handlePositionReduced))
}

func (w *OutboxWriter) Close() error {
	if w.closed {
		return errOutboxClosed
	}
	for _, sub := range w.subs {
		sub.Close()
	}
	w.closed = true
	return w.outbox.Close()
}

func (w *OutboxWriter) Flush() error {
	if w.closed {
		return errOutboxClosed
	}
	return w.outbox.Flush()
}

func (w *OutboxWriter) handleOrderEvent(data []byte) {
	var event types.OrderEvent
	if err := messaging.DecodeGob(data, &event); err != nil {
		return
	}
	w.OnOrderEvent(&event)
}

func (w *OutboxWriter) OnOrderEvent(event *types.OrderEvent) {
	if !isClosedOrderStatus(event.Status) {
		return
	}
	record := &OrderClosed{
		OrderID:    int64(event.OrderID),
		UserID:     uint64(event.UserID),
		Symbol:     event.Symbol,
		Category:   event.Category,
		Side:       event.Side,
		Type:       event.Type,
		Status:     event.Status,
		Price:      int64(event.Price),
		Quantity:   int64(event.Quantity),
		Filled:     int64(event.Filled),
		ClosedAt:   event.UpdatedAt,
		OrderLink:  event.OrderLinkId,
		ReduceOnly: event.ReduceOnly,
	}
	payload, err := EncodeOrderClosed(record)
	if err != nil {
		return
	}
	_ = w.outbox.Append(KIND_ORDER_CLOSED, payload)
}

func (w *OutboxWriter) handleTradeEvent(data []byte) {
	var event types.TradeEvent
	if err := messaging.DecodeGob(data, &event); err != nil {
		return
	}
	w.OnTradeEvent(&event)
}

func (w *OutboxWriter) OnTradeEvent(event *types.TradeEvent) {
	record := &Trade{
		TradeID:    int64(event.TradeID),
		Symbol:     event.Symbol,
		Category:   event.Category,
		TakerID:    uint64(event.TakerID),
		MakerID:    uint64(event.MakerID),
		TakerOrder: int64(event.TakerOrderID),
		MakerOrder: int64(event.MakerOrderID),
		Price:      int64(event.Price),
		Quantity:   int64(event.Quantity),
		ExecutedAt: event.ExecutedAt,
	}
	payload, err := EncodeTrade(record)
	if err != nil {
		return
	}
	_ = w.outbox.Append(KIND_TRADE, payload)
}

func (w *OutboxWriter) handlePositionReduced(data []byte) {
	var event types.PositionReducedEvent
	if err := messaging.DecodeGob(data, &event); err != nil {
		return
	}
	w.OnPositionReduced(&event)
}

func (w *OutboxWriter) OnPositionReduced(event *types.PositionReducedEvent) {
	record := &PnL{
		UserID:    uint64(event.UserID),
		Symbol:    event.Symbol,
		Category:  event.Category,
		Realized:  event.RPNL,
		FeePaid:   0,
		CreatedAt: event.ExecutedAt,
	}
	payload, err := EncodePnL(record)
	if err != nil {
		return
	}
	_ = w.outbox.Append(KIND_PNL, payload)
}

func isClosedOrderStatus(status int8) bool {
	switch status {
	case constants.ORDER_STATUS_FILLED,
		constants.ORDER_STATUS_CANCELED,
		constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED,
		constants.ORDER_STATUS_DEACTIVATED:
		return true
	default:
		return false
	}
}
