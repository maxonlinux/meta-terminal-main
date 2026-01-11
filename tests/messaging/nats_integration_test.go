package messaging_test

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/nats-io/nats-server/v2/server"
)

func TestNATSPublishSubscribeGob(t *testing.T) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string, 1)
	sub := nc.Subscribe(ctx, "test.subject", "test", func(data []byte) {
		var payload struct {
			Value string
		}
		if err := messaging.DecodeGob(data, &payload); err == nil {
			ch <- payload.Value
		}
	})
	defer sub.Close()

	if err := nc.PublishGob(ctx, "test.subject", struct{ Value string }{Value: "ok"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-ch:
		if got != "ok" {
			t.Fatalf("unexpected payload: %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for message")
	}
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
