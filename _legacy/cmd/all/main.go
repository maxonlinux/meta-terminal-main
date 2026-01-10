package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/services/clearing"
	"github.com/anomalyco/meta-terminal-go/services/gateway"
	"github.com/anomalyco/meta-terminal-go/services/marketdata"
	"github.com/anomalyco/meta-terminal-go/services/oms"
	"github.com/anomalyco/meta-terminal-go/services/portfolio"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg := loadConfig()
	Mode := flag.String("mode", "all", "Mode: all, gateway, oms, portfolio, marketdata, clearing")
	flag.Parse()

	switch *Mode {
	case "all":
		runAll(cfg)
	case "gateway":
		runGateway(cfg)
	case "oms":
		runOMS(cfg)
	case "portfolio":
		runPortfolio(cfg)
	case "marketdata":
		runMarketData(cfg)
	case "clearing":
		runClearing(cfg)
	default:
		fmt.Printf("Unknown mode: %s\n", *Mode)
		os.Exit(1)
	}
}

type Config struct {
	NATSURL      string
	StreamPrefix string
	JWTSecret    string
	Port         string
	Shards       []string
}

func loadConfig() Config {
	return Config{
		NATSURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		StreamPrefix: getEnv("STREAM_PREFIX", "meta"),
		JWTSecret:    getEnv("JWT_SECRET", "default-secret-change-me"),
		Port:         getEnv("PORT", "8080"),
		Shards:       parseShards(getEnv("SHARDS", "BTCUSDT")),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseShards(s string) []string {
	if s == "" {
		return []string{}
	}
	var shards []string
	for _, shard := range split(s) {
		if shard != "" {
			shards = append(shards, shard)
		}
	}
	return shards
}

func split(s string) []string {
	var result []string
	var current []rune
	for _, r := range s {
		if r == ',' {
			result = append(result, string(current))
			current = nil
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

func runAll(cfg Config) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	reg := registry.New()

	wg.Add(1)
	go func() {
		defer wg.Done()
		svc, _ := marketdata.New(marketdata.Config{
			NATSURL:      cfg.NATSURL,
			StreamPrefix: cfg.StreamPrefix,
		}, reg)
		svc.Start(ctx)
	}()

	for _, shard := range cfg.Shards {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			svc, _ := oms.New(oms.Config{
				NATSURL:      cfg.NATSURL,
				StreamPrefix: cfg.StreamPrefix,
				Shard:        s,
				Snapshots:    nil,
			}, reg)
			svc.Start(ctx)
		}(shard)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		svc, _ := portfolio.New(portfolio.Config{
			NATSURL:      cfg.NATSURL,
			StreamPrefix: cfg.StreamPrefix,
		})
		svc.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		svc, _ := clearing.New(clearing.Config{
			NATSURL:      cfg.NATSURL,
			StreamPrefix: cfg.StreamPrefix,
		})
		svc.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		svc, _ := gateway.New(gateway.Config{
			NATSURL:      cfg.NATSURL,
			JWTSecret:    cfg.JWTSecret,
			Port:         cfg.Port,
			StreamPrefix: cfg.StreamPrefix,
		})
		svc.Start(ctx)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	cancel()
	wg.Wait()
	log.Println("all services stopped")
}

func runGateway(cfg Config) {
	svc, _ := gateway.New(gateway.Config{
		NATSURL:      cfg.NATSURL,
		JWTSecret:    cfg.JWTSecret,
		Port:         cfg.Port,
		StreamPrefix: cfg.StreamPrefix,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
}

func runOMS(cfg Config) {
	reg := registry.New()
	for _, shard := range cfg.Shards {
		svc, _ := oms.New(oms.Config{
			NATSURL:      cfg.NATSURL,
			StreamPrefix: cfg.StreamPrefix,
			Shard:        shard,
			Snapshots:    nil,
		}, reg)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc.Start(ctx)
	}
	select {}
}

func runPortfolio(cfg Config) {
	svc, _ := portfolio.New(portfolio.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
}

func runMarketData(cfg Config) {
	reg := registry.New()
	svc, _ := marketdata.New(marketdata.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	}, reg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
}

func runClearing(cfg Config) {
	svc, _ := clearing.New(clearing.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
}
