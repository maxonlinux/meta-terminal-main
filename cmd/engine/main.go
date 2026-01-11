package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/engine"
)

func main() {
	cfg := engine.Config{
		NATSURL:      os.Getenv("NATS_URL"),
		StreamPrefix: envString("STREAM_PREFIX", "meta"),
		OutboxPath:   envString("OUTBOX_PATH", "data/outbox.log"),
		OutboxBuf:    64 * 1024,
	}

	e, err := engine.New(cfg)
	if err != nil {
		log.Fatalf("engine init: %v", err)
	}
	defer e.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := e.Start(ctx); err != nil {
		log.Fatalf("engine start: %v", err)
	}

	<-ctx.Done()
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
