package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/meta-terminal-go/config"
	"github.com/anomalyco/meta-terminal-go/internal/api"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/snapshot"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/wal"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := state.NewEngineState()
	orderStore := state.NewOrderStore()

	spotMarket := spot.New(s, orderStore)
	linearMarket := linear.New(s, orderStore)

	w, err := wal.New(cfg.WALPath, cfg.WALBufferSize)
	if err != nil {
		log.Fatalf("Failed to init WAL: %v", err)
	}
	defer w.Close()

	snap := snapshot.New(cfg.SnapshotPath)
	_, offset, _ := snap.Load()
	if offset > 0 {
		log.Printf("Loaded snapshot, WAL offset: %d", offset)
	}

	out, err := outbox.New(cfg.OutboxPath, cfg.OutboxBatchSize, cfg.OutboxFlushDuration)
	if err != nil {
		log.Fatalf("Failed to init outbox: %v", err)
	}
	defer out.Close()

	server := api.NewServer(cfg, s, spotMarket, linearMarket)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	if err := server.Start(ctx); err != nil {
		log.Printf("Server error: %v", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				snap.Save(s, 0)
				return
			case <-time.After(time.Duration(cfg.SnapshotInterval) * time.Second):
				snap.Save(s, 0)
			}
		}
	}()

	log.Println("Trading engine shutdown complete")
}
