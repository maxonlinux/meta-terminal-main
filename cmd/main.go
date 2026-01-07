package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/api"
	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/snapshot"
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

	w, err := wal.New(cfg.WALPath, cfg.WALBufferSize)
	if err != nil {
		log.Fatalf("Failed to initialize WAL: %v", err)
	}
	defer w.Close()

	state := state.New()
	var startOffset int64 = 0

	snap := snapshot.New(cfg.SnapshotPath, 100*1024*1024)
	loadedState, offset, err := snap.Load()
	if err == nil {
		state = loadedState
		startOffset = offset
		log.Printf("Loaded snapshot, WAL offset: %d", startOffset)
	}

	ob := orderbook.New()
	tradingEngine := engine.New(w, state)

	if startOffset > 0 {
		log.Printf("Replaying WAL from offset %d...", startOffset)
		err = w.Replay(state, startOffset, ob)
		if err != nil {
			log.Printf("WAL replay error: %v", err)
		} else {
			log.Printf("WAL replay completed successfully")
		}
	}

	snapManager := snapshot.NewManager(snap, w, state, time.Duration(cfg.SnapshotInterval)*time.Second, 100)
	snapManager.Start(ctx)
	defer snapManager.Stop()

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
