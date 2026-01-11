package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/history/duckdb"
)

func main() {
	env := config.Load()
	store, err := duckdb.Open(env.DuckDBPath)
	if err != nil {
		log.Fatalf("duckdb open: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	loader, err := duckdb.NewLoaderFromPath(store, env.OutboxPath, env.OutboxOffsetPath, env.OutboxBufSize, env.HistoryBatchSize)
	if err != nil {
		log.Fatalf("outbox loader: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(time.Duration(env.HistoryPollMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := loader.Drain(); err != nil {
				log.Printf("drain: %v", err)
			}
		}
	}
}
