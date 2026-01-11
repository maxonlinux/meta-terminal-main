package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/history/duckdb"
)

const (
	ENV_OUTBOX_PATH   = "OUTBOX_PATH"
	ENV_OFFSET_PATH   = "OUTBOX_OFFSET_PATH"
	ENV_DUCKDB_PATH   = "DUCKDB_PATH"
	ENV_BATCH_SIZE    = "HISTORY_BATCH_SIZE"
	ENV_BUF_SIZE      = "OUTBOX_BUF_SIZE"
	ENV_POLL_INTERVAL = "HISTORY_POLL_MS"
)

func main() {
	cfg := loadConfig()
	store, err := duckdb.Open(cfg.duckDBPath)
	if err != nil {
		log.Fatalf("duckdb open: %v", err)
	}
	defer store.Close()

	loader, err := duckdb.NewLoaderFromPath(store, cfg.outboxPath, cfg.offsetPath, cfg.bufSize, cfg.batchSize)
	if err != nil {
		log.Fatalf("outbox loader: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(cfg.pollInterval)
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

type config struct {
	outboxPath   string
	offsetPath   string
	duckDBPath   string
	batchSize    int
	bufSize      int
	pollInterval time.Duration
}

func loadConfig() config {
	return config{
		outboxPath:   envString(ENV_OUTBOX_PATH, "data/outbox.log"),
		offsetPath:   envString(ENV_OFFSET_PATH, "data/outbox.offset"),
		duckDBPath:   envString(ENV_DUCKDB_PATH, "data/history.duckdb"),
		batchSize:    envInt(ENV_BATCH_SIZE, 1024),
		bufSize:      envInt(ENV_BUF_SIZE, 64*1024),
		pollInterval: time.Duration(envInt(ENV_POLL_INTERVAL, 250)) * time.Millisecond,
	}
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
