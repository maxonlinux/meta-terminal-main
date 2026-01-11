package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/marketdata"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
)

func main() {
	env := config.Load()
	if len(env.QuoteAssets) > 0 {
		balance.SetQuoteAssets(env.QuoteAssets)
	}

	reg := registry.New()
	svc, err := marketdata.New(marketdata.Config{
		NATSURL:          env.NATSURL,
		StreamPrefix:     env.StreamPrefix,
		AssetsURL:        env.AssetsURL,
		MultiplexerURL:   env.MultiplexerURL,
		SyncIntervalSecs: env.SyncIntervalSecs,
	}, reg, nil)
	if err != nil {
		log.Fatalf("marketdata init: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := svc.Start(ctx); err != nil {
		log.Fatalf("marketdata start: %v", err)
	}

	<-ctx.Done()
	svc.Stop()
}
