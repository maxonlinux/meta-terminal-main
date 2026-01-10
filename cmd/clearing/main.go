package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/clearing"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/portfolio"
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

	nats, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		log.Fatalf("Failed to create NATS: %v", err)
	}
	defer nats.Close()

	portfolioSvc := portfolio.New(portfolio.Config{NATS: nats})
	_ = clearing.New(portfolioSvc)

	log.Println("Clearing service started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
