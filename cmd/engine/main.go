package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
)

func main() {
	env := config.Load()
	if len(env.QuoteAssets) > 0 {
		balance.SetQuoteAssets(env.QuoteAssets)
	}

	cfg := engine.Config{
		NATSURL:      env.NATSURL,
		StreamPrefix: env.StreamPrefix,
		OutboxPath:   env.OutboxPath,
		OutboxBuf:    env.OutboxBuf,
	}

	e, err := engine.New(cfg)
	if err != nil {
		log.Fatalf("engine init: %v", err)
	}
	defer func() {
		_ = e.Close()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := e.Start(ctx); err != nil {
		log.Fatalf("engine start: %v", err)
	}

	<-ctx.Done()
}
