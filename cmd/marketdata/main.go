package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/marketdata"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
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

	omsSvc := &oms.Service{}

	marketdataSvc, err := marketdata.New(marketdata.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, omsSvc)
	if err != nil {
		log.Fatalf("Failed to create marketdata service: %v", err)
	}

	go func() {
		if err := marketdataSvc.Start(ctx); err != nil {
			log.Printf("Marketdata error: %v", err)
		}
	}()

	log.Println("Marketdata service started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	marketdataSvc.Stop()

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
