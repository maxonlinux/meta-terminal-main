package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/services/portfolio"
)

func main() {
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS URL")
	flag.Parse()

	log.Println("Portfolio starting...")

	svc, err := portfolio.New(portfolio.Config{
		NATSURL:      *natsURL,
		StreamPrefix: "META",
	})
	if err != nil {
		log.Fatalf("create portfolio: %v", err)
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
	log.Println("portfolio stopped")
}
