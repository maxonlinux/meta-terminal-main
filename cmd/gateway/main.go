package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/gateway"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
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
		GatewayPort  int
	}{
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		StreamPrefix: getEnv("STREAM_PREFIX", "meta"),
		GatewayPort:  8080,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	omsSvc := &oms.Service{}
	portfolioSvc := portfolio.New(portfolio.Config{})

	gatewaySvc := gateway.New(gateway.Config{
		Port:         cfg.GatewayPort,
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, omsSvc, portfolioSvc, nil)

	go func() {
		if err := gatewaySvc.Start(ctx); err != nil {
			log.Printf("Gateway error: %v", err)
		}
	}()

	log.Println("Gateway service started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	gatewaySvc.Stop()

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
