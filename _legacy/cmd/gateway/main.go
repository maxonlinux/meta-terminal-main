package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/services/gateway"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg := gateway.Config{
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:    getEnv("JWT_SECRET", "change-this-in-production"),
		Port:         getEnv("PORT", "8080"),
		StreamPrefix: getEnv("STREAM_PREFIX", "meta"),
	}

	svc, err := gateway.New(cfg)
	if err != nil {
		log.Fatalf("create gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	log.Printf("gateway starting on port %s", cfg.Port)
	if err := svc.Start(ctx); err != nil {
		log.Fatalf("start gateway: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
