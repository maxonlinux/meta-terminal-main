package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/clearing"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/portfolio"
	"github.com/anomalyco/meta-terminal-go/internal/risk"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	cfg := struct {
		NATSURL      string
		StreamPrefix string
	}{
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		StreamPrefix: getEnv("STREAM_PREFIX", "meta"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nats, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		log.Fatalf("Failed to create NATS: %v", err)
	}
	defer nats.Close()

	portfolioSvc := portfolio.New(portfolio.Config{NATS: nats})
	clearingSvc := clearing.New(portfolioSvc)
	omsSvc, err := oms.New(oms.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, portfolioSvc, clearingSvc)
	if err != nil {
		log.Fatalf("Failed to create OMS: %v", err)
	}

	riskSvc, err := risk.New(risk.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, portfolioSvc, omsSvc)
	if err != nil {
		log.Fatalf("Failed to create Risk service: %v", err)
	}

	go func() {
		if err := riskSvc.Start(ctx); err != nil {
			log.Printf("Risk service error: %v", err)
		}
	}()

	log.Println("Risk service started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	riskSvc.Stop()

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
