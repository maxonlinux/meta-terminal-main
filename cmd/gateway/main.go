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
)

func main() {
	env := config.Load()
	balance.SetQuoteAssets(env.QuoteAssets)

	reg := registry.New()
	engineCfg := engine.Config{
		NATSURL:      env.NATSURL,
		StreamPrefix: env.StreamPrefix,
		OutboxPath:   env.OutboxPath,
		OutboxBuf:    env.OutboxBuf,
	}
	e, err := engine.New(engineCfg)
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
	var md *marketdata.Service
	if env.AssetsURL != "" && env.MultiplexerURL != "" {
		md, err = marketdata.New(marketdata.Config{
			NATSURL:          env.NATSURL,
			StreamPrefix:     env.StreamPrefix,
			AssetsURL:        env.AssetsURL,
			MultiplexerURL:   env.MultiplexerURL,
			SyncIntervalSecs: env.SyncIntervalSecs,
		}, reg, e.OMS)
		if err != nil {
			log.Fatalf("marketdata init: %v", err)
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
	if md != nil {
		if err := md.Start(ctx); err != nil {
			log.Fatalf("marketdata start: %v", err)
		}
	}
	if err := gw.Start(ctx); err != nil {
		log.Fatalf("gateway start: %v", err)
	}

	<-ctx.Done()
	if md != nil {
		md.Stop()
	}
	gw.Stop()
}
