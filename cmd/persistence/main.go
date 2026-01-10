package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/duckdb"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	cfg := struct {
		NATSURL      string
		StreamPrefix string
		DataDir      string
		OutboxDir    string
	}{
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		StreamPrefix: getEnv("STREAM_PREFIX", "meta"),
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

	nats, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		log.Fatalf("Failed to create NATS: %v", err)
	}
	defer nats.Close()

	outboxSvc, err := outbox.New(outbox.Config{
		Dir:           cfg.OutboxDir,
		FlushInterval: 1000,
		FlushSize:     1000,
		NATS:          nats,
	})
	if err != nil {
		log.Fatalf("Failed to create outbox: %v", err)
	}
	defer outboxSvc.Close()

	batchWriter := outbox.NewBatchWriter(outboxSvc, db)
	if err := batchWriter.Start(ctx); err != nil {
		log.Fatalf("Failed to start batch writer: %v", err)
	}

	log.Println("Persistence service started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
