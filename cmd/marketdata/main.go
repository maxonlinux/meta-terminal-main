package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/services/marketdata"
)

func main() {
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS URL")
	prefix := flag.String("prefix", "marketdata", "Stream prefix")
	flag.Parse()

	log.Println("MarketData starting...")

	reg := registry.New()

	cfg := marketdata.Config{
		NATSURL:      *natsURL,
		StreamPrefix: *prefix,
	}

	svc, err := marketdata.New(cfg, reg)
	if err != nil {
		log.Fatalf("create marketdata: %v", err)
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
	log.Println("marketdata stopped")
}
