package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
)

func main() {
	cfg := config.Load()

	if cfg.DataDir == "" {
		log.Fatal("DATA_DIR is required")
	}

	pkv, err := persistence.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open pebblekv: %v", err)
	}
	defer pkv.Close()

	store := oms.NewService(pkv)
	reg := registry.New()

	loader := registry.NewLoader(cfg, reg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go loader.Start(ctx)

	eng := engine.NewEngine(store, pkv, reg, nil)

	<-ctx.Done()
	eng.Shutdown()
}
