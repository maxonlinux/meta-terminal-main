package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/clearing"
	"github.com/anomalyco/meta-terminal-go/internal/gateway"
	"github.com/anomalyco/meta-terminal-go/internal/marketdata"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/duckdb"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
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
		GatewayPort  int
		DataDir      string
		OutboxDir    string
	}{
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		StreamPrefix: getEnv("STREAM_PREFIX", "meta"),
		GatewayPort:  8080,
		DataDir:      getEnv("DATA_DIR", "data"),
		OutboxDir:    getEnv("OUTBOX_DIR", "data/outbox"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := duckdb.New(duckdb.Config{Path: cfg.DataDir + "/trading.db"})
	if err != nil {
		log.Fatalf("Failed to open DuckDB: %v", err)
	}
	defer db.Close()

	outboxSvc, err := outbox.New(outbox.Config{
		Dir:           cfg.OutboxDir,
		FlushInterval: 1000,
		FlushSize:     1000,
	})
	if err != nil {
		log.Fatalf("Failed to create outbox: %v", err)
	}
	defer outboxSvc.Close()

	batchWriter := outbox.NewBatchWriter(outboxSvc, db)
	if err := batchWriter.Start(ctx); err != nil {
		log.Fatalf("Failed to start batch writer: %v", err)
	}

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

	gatewaySvc := gateway.New(gateway.Config{
		Port:         cfg.GatewayPort,
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, omsSvc, portfolioSvc, nil)

	marketdataSvc, err := marketdata.New(marketdata.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, omsSvc)
	if err != nil {
		log.Fatalf("Failed to create marketdata service: %v", err)
	}

	riskSvc, err := risk.New(risk.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, portfolioSvc, omsSvc)
	if err != nil {
		log.Fatalf("Failed to create risk service: %v", err)
	}

	go func() {
		if err := omsSvc.Start(ctx); err != nil {
			log.Printf("OMS error: %v", err)
		}
	}()

	go func() {
		if err := gatewaySvc.Start(ctx); err != nil {
			log.Printf("Gateway error: %v", err)
		}
	}()

	go func() {
		if err := marketdataSvc.Start(ctx); err != nil {
			log.Printf("Marketdata error: %v", err)
		}
	}()

	go func() {
		if err := riskSvc.Start(ctx); err != nil {
			log.Printf("Risk error: %v", err)
		}
	}()

	log.Println("Trading engine started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()

	gatewaySvc.Stop()
	marketdataSvc.Stop()
	riskSvc.Stop()

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
