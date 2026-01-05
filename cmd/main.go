package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/api"
	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wal, err := wal.New(cfg.WALPath, cfg.WALBufferSize)
	if err != nil {
		log.Fatalf("Failed to initialize WAL: %v", err)
	}
	defer wal.Close()

	state := state.New()

	tradingEngine := engine.New(wal, state)

	outboxWorker, err := outbox.New(cfg.OutboxPath, cfg.OutboxBatchSize, cfg.OutboxFlushDuration)
	if err != nil {
		log.Fatalf("Failed to initialize outbox: %v", err)
	}
	defer outboxWorker.Close()

	server := api.NewServer(cfg, tradingEngine)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("Trading engine shutdown complete")
}
