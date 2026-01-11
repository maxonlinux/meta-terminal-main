package history_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/nats-io/nats-server/v2/server"
)

func TestOutboxWriterConsumesOrderEvents(t *testing.T) {
	srv := startNATSServer(t)
	defer srv.Shutdown()

	nc, err := messaging.New(messaging.Config{
		URL:          srv.ClientURL(),
		StreamPrefix: "test",
	})
	if err != nil {
		t.Fatalf("nats client: %v", err)
	}
	defer nc.Close()

	dir := t.TempDir()
	outboxPath := filepath.Join(dir, "outbox.log")
	writer, err := history.NewOutboxWriter(history.OutboxConfig{
		Path:    outboxPath,
		BufSize: 32 * 1024,
		NATS:    nc,
	})
	if err != nil {
		t.Fatalf("outbox writer: %v", err)
	}
	defer func() {
		_ = writer.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer.Start(ctx)

	event := &types.OrderEvent{
		OrderID:   1,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_FILLED,
		Price:     1000,
		Quantity:  1,
		Filled:    1,
		UpdatedAt: types.NowNano(),
	}
	if err := nc.PublishGob(ctx, messaging.OrderEventTopic("BTCUSDT"), event); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if !waitForOutbox(outboxPath, 2*time.Second) {
		t.Fatalf("outbox not written")
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	reader, err := outbox.OpenReader(outboxPath, 32*1024)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	kind, payload, err := reader.Next()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if kind != history.KIND_ORDER_CLOSED {
		t.Fatalf("expected order closed record, got %d", kind)
	}
	record, err := history.DecodeRecord(kind, payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if record.OrderClosed == nil || record.OrderClosed.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("unexpected record: %+v", record.OrderClosed)
	}
}

func waitForOutbox(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func startNATSServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Port:      -1,
		JetStream: true,
	}
	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("nats server: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(2 * time.Second) {
		srv.Shutdown()
		t.Fatalf("nats server not ready")
	}
	return srv
}
