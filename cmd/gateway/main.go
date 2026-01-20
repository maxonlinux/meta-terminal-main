package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/gateway"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loader := registry.NewLoader(cfg, reg)
	go loader.Start(ctx)

	eng := engine.NewEngine(store, pkv, reg, nil)

	go func() {
		if err := gateway.Run(eng, pkv, ":8080"); err != nil {
			log.Printf("gateway error: %v", err)
		}
	}()

	<-ctx.Done()

	eng.Shutdown()
}
