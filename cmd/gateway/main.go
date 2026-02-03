package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	apihandlers "github.com/maxonlinux/meta-terminal-go/internal/api/http"
	wsapi "github.com/maxonlinux/meta-terminal-go/internal/api/ws"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)

	cfg := config.Load()
	log.Printf("gateway config data_dir=%s", cfg.DataDir)

	reg := registry.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loader := registry.NewLoader(cfg, reg)
	go loader.Start(ctx)

	persistenceStore, err := persistence.Open(cfg.DataDir, reg)
	if err != nil {
		log.Fatalf("persistence open: %v", err)
	}
	defer func() {
		_ = persistenceStore.Close()
	}()

	batchSink := outbox.NewBatchSink(persistenceStore, outbox.BatchOptions{
		BatchSize:  1000,
		FlushEvery: 100 * time.Millisecond,
	})
	defer func() {
		_ = batchSink.Stop()
	}()

	ob, err := outbox.OpenWithOptions(cfg.DataDir, outbox.Options{EventSink: batchSink})
	if err != nil {
		log.Fatalf("outbox open: %v", err)
	}
	defer func() {
		_ = ob.Close()
	}()

	eng := engine.NewEngine(ob, reg, nil)
	if err := persistenceStore.LoadCore(eng.Store(), eng.Portfolio()); err != nil {
		log.Fatalf("load core: %v", err)
	}
	eng.RebuildBooks()

	ob.Start()

	go func() {
		if err := runServer(eng, cfg, persistenceStore); err != nil {
			log.Printf("gateway error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutdown signal received")
	eng.Shutdown()
}

func runServer(eng *engine.Engine, cfg config.Config, persistenceStore *persistence.Store) error {
	userStore, err := users.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		return err
	}
	defer func() {
		_ = userStore.Close()
	}()

	jwtService := users.NewJWTService()
	authService := users.NewService(userStore)
	otpService := otp.NewService()
	impService := impersonation.NewService(authService)

	queryService := query.New(eng.Registry(), eng.Portfolio(), eng.Store(), eng.TradeFeed(), eng.ReadBook)
	router := apihandlers.NewRouter(eng, queryService, persistenceStore, userStore, jwtService, authService, otpService, impService)
	wsHandler := wsapi.NewWsHandler(queryService, jwtService)
	eng.SetPublisher(wsHandler.Publisher())
	router.SetWsHandler(wsHandler)

	e := echo.New()
	e.HideBanner = true

	router.Register(e)

	log.Printf("http server listening on :8080")
	return e.Start(":8080")
}
