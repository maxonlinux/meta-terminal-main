package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/triggers"
	"github.com/anomalyco/meta-terminal-go/internal/types"
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

	omsSvc, _ := oms.New(oms.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, nil, nil)

	triggerMon := triggers.New()

	go func() {
		if err := runTriggerMonitor(ctx, nats, omsSvc, triggerMon); err != nil {
			log.Printf("Trigger monitor error: %v", err)
		}
	}()

	log.Println("Triggers service started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()

	log.Println("Shutdown complete")
}

func runTriggerMonitor(ctx context.Context, nats *messaging.NATS, omsSvc *oms.Service, triggerMon *triggers.Monitor) error {
	nats.Subscribe(ctx, messaging.SubjectPriceTick, "triggers", func(data []byte) {
		var tick struct {
			Symbol string
			Price  int64
		}
		if err := messaging.DecodeGob(data, &tick); err != nil {
			return
		}
		triggerMon.Check(types.Price(tick.Price))
	})
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
