package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/gateway"
	"github.com/anomalyco/meta-terminal-go/internal/history/duckdb"
	"github.com/anomalyco/meta-terminal-go/internal/marketdata"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/risk"
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

	reg := registry.New()

	e, err := engine.New(cfg)
	if err != nil {
		log.Fatalf("engine init: %v", err)
	}
	defer func() {
		_ = e.Close()
	}()

	var store *duckdb.Store
	if env.DuckDBPath != "" {
		store, err = duckdb.Open(env.DuckDBPath)
		if err != nil {
			log.Fatalf("duckdb open: %v", err)
		}
		defer func() {
			_ = store.Close()
		}()
	}

	md, err := marketdata.New(marketdata.Config{
		NATSURL:          env.NATSURL,
		StreamPrefix:     env.StreamPrefix,
		AssetsURL:        env.AssetsURL,
		MultiplexerURL:   env.MultiplexerURL,
		SyncIntervalSecs: env.SyncIntervalSecs,
	}, reg, e.OMS)
	if err != nil {
		log.Fatalf("marketdata init: %v", err)
	}

	var riskSvc *risk.Service
	if env.NATSURL != "" {
		riskSvc, err = risk.New(risk.Config{
			NATSURL:      env.NATSURL,
			StreamPrefix: env.StreamPrefix,
		}, e.Portfolio, e.OMS)
		if err != nil {
			log.Fatalf("risk init: %v", err)
		}
	}

	gw := gateway.New(gateway.Config{
		Port:         env.Port,
		NATSURL:      env.NATSURL,
		StreamPrefix: env.StreamPrefix,
		JWTSecret:    env.JWTSecret,
		JWTCookie:    env.JWTCookie,
	}, e.OMS, e.Portfolio, store, reg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := e.Start(ctx); err != nil {
		log.Fatalf("engine start: %v", err)
	}
	if err := md.Start(ctx); err != nil {
		log.Fatalf("marketdata start: %v", err)
	}
	if riskSvc != nil {
		if err := riskSvc.Start(ctx); err != nil {
			log.Fatalf("risk start: %v", err)
		}
	}
	if err := gw.Start(ctx); err != nil {
		log.Fatalf("gateway start: %v", err)
	}

	<-ctx.Done()
}
