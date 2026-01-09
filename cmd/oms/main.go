package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/id"
	"github.com/anomalyco/meta-terminal-go/internal/persistence"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/services/oms"
)

func main() {
	shard := flag.String("shard", "BTCUSDT", "symbol to handle")
	dbPath := flag.String("db", "data/snapshots.db", "SQLite path")
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS URL")
	flag.Parse()

	fmt.Printf("OMS starting: shard=%s node=%d\n", *shard, id.NodeID())

	reg := registry.New()

	var store *persistence.SnapshotStore
	if *dbPath != "" {
		var err error
		store, err = persistence.New(*dbPath)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
	}

	svc, err := oms.New(oms.Config{
		NATSURL:      *natsURL,
		StreamPrefix: "META",
		Shard:        *shard,
		Snapshots:    store,
	}, reg)
	if err != nil {
		log.Fatalf("create oms: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		log.Fatalf("start: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	cancel()
	svc.Close()
	log.Println("stopped")
}
