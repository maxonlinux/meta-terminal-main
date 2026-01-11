package engine_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestTerminalOrderWritesOutboxAndRemovesFromMemory(t *testing.T) {
	dir := t.TempDir()
	outboxPath := filepath.Join(dir, "outbox.log")

	e, err := engine.New(engine.Config{
		OutboxPath: outboxPath,
		OutboxBuf:  32 * 1024,
	})
	if err != nil {
		t.Fatalf("engine init: %v", err)
	}
	defer func() {
		_ = e.Close()
	}()

	userID := types.UserID(1)
	e.Portfolio.Balances[userID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 100000, Locked: 0},
	}

	result, err := e.OMS.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    1000,
	})
	if err != nil {
		t.Fatalf("place order: %v", err)
	}

	if err := e.OMS.CancelOrder(context.Background(), userID, result.Orders[0].ID); err != nil {
		t.Fatalf("cancel order: %v", err)
	}

	if len(e.OMS.GetOrders(userID)) != 0 {
		t.Fatalf("expected order removed from memory after cancel")
	}

	if err := e.Outbox.Flush(); err != nil {
		t.Fatalf("outbox flush: %v", err)
	}

	reader, err := outbox.OpenReader(outboxPath, 32*1024)
	if err != nil {
		t.Fatalf("outbox reader: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	kind, payload, err := reader.Next()
	if err != nil {
		t.Fatalf("outbox read: %v", err)
	}
	if kind != history.KIND_ORDER_CLOSED {
		t.Fatalf("expected order closed record, got kind %d", kind)
	}
	record, err := history.DecodeRecord(kind, payload)
	if err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if record.OrderClosed == nil || record.OrderClosed.Status != constants.ORDER_STATUS_CANCELED {
		t.Fatalf("expected canceled order record, got %+v", record.OrderClosed)
	}
}

func TestFilledOrdersWriteOutbox(t *testing.T) {
	dir := t.TempDir()
	outboxPath := filepath.Join(dir, "outbox.log")

	e, err := engine.New(engine.Config{
		OutboxPath: outboxPath,
		OutboxBuf:  32 * 1024,
	})
	if err != nil {
		t.Fatalf("engine init: %v", err)
	}
	defer func() {
		_ = e.Close()
	}()

	makerID := types.UserID(1)
	takerID := types.UserID(2)
	e.Portfolio.Balances[makerID] = map[string]*types.UserBalance{
		"BTC": {Asset: "BTC", Available: 1, Locked: 0},
	}
	e.Portfolio.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 100000, Locked: 0},
	}

	_, err = e.OMS.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    1000,
	})
	if err != nil {
		t.Fatalf("maker place: %v", err)
	}

	_, err = e.OMS.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 1,
		Price:    1000,
	})
	if err != nil {
		t.Fatalf("taker place: %v", err)
	}

	if len(e.OMS.GetOrders(makerID)) != 0 || len(e.OMS.GetOrders(takerID)) != 0 {
		t.Fatalf("expected filled orders removed from memory")
	}

	if err := e.Outbox.Flush(); err != nil {
		t.Fatalf("outbox flush: %v", err)
	}

	reader, err := outbox.OpenReader(outboxPath, 32*1024)
	if err != nil {
		t.Fatalf("outbox reader: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	var foundFilled bool
	for {
		kind, payload, err := reader.Next()
		if err != nil {
			break
		}
		if kind != history.KIND_ORDER_CLOSED {
			continue
		}
		record, err := history.DecodeRecord(kind, payload)
		if err != nil {
			t.Fatalf("decode record: %v", err)
		}
		if record.OrderClosed != nil && record.OrderClosed.Status == constants.ORDER_STATUS_FILLED {
			foundFilled = true
			break
		}
	}
	if !foundFilled {
		t.Fatalf("expected filled order record in outbox")
	}
}
