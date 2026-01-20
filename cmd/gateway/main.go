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
)

func main() {
	cfg := config.Load()

	if cfg.DataDir == "" {
		log.Fatal("DATA_DIR is required")
	}

	reg := registry.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loader := registry.NewLoader(cfg, reg)
	go loader.Start(ctx)

	store := oms.NewService(nil)
	eng := engine.NewEngine(store, reg, nil)

	go func() {
		if err := gateway.Run(eng, cfg.DataDir, ":8080"); err != nil {
			log.Printf("gateway error: %v", err)
		}
	}()

	<-ctx.Done()

	eng.Shutdown()
}
