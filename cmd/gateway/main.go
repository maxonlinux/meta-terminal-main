package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	apihandlers "github.com/maxonlinux/meta-terminal-go/internal/api/http"
	wsapi "github.com/maxonlinux/meta-terminal-go/internal/api/ws"
	"github.com/maxonlinux/meta-terminal-go/internal/auth"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/kyc"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/plan"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	if envMap, err := godotenv.Read(); err != nil {
		log.Printf("env file not loaded: %v", err)
	} else {
		for key, value := range envMap {
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
	}

	cfg := config.Load()
	log.Printf("gateway config data_dir=%s port=%s", cfg.DataDir, cfg.Port)

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

	planRepo, err := plan.NewRepository(persistenceStore.DB())
	if err != nil {
		log.Fatalf("plan repo: %v", err)
	}
	planService := plan.NewService(planRepo, reg)

	eng := engine.NewEngine(ob, reg, nil)
	eng.SetPlanPolicy(planService)
	if err := persistenceStore.LoadCore(eng.Store(), eng.Portfolio()); err != nil {
		log.Fatalf("load core: %v", err)
	}
	eng.RebuildBooks()

	ob.Start()

	kycRepo, err := kyc.NewRepository(persistenceStore.DB())
	if err != nil {
		log.Fatalf("kyc repo: %v", err)
	}

	go func() {
		if err := runServer(eng, cfg, persistenceStore, planService, planRepo, kycRepo); err != nil {
			log.Printf("gateway error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutdown signal received")
	eng.Shutdown()
}

func runServer(eng *engine.Engine, cfg config.Config, persistenceStore *persistence.Store, planService *plan.Service, planRepo *plan.Repository, kycRepo *kyc.Repository) error {
	userStore, err := users.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		return err
	}
	defer func() {
		_ = userStore.Close()
	}()

	jwtService, err := auth.NewJWTService(cfg)
	if err != nil {
		log.Fatalf("jwt service: %v", err)
	}
	authService := users.NewService(userStore)
	otpService := otp.NewService()
	impService := impersonation.NewService(authService)

	router := apihandlers.NewRouter(eng, persistenceStore, userStore, jwtService, authService, otpService, impService, planService, planRepo, kycRepo, cfg)
	wsHandler := wsapi.NewWsHandler(eng.ReadBook, jwtService, cfg.JwtCookieName)
	eng.SetPublisher(wsHandler.Publisher())
	router.SetWsHandler(wsHandler)

	e := echo.New()

	router.Register(e)

	addr := ":" + cfg.Port
	log.Printf("http server listening on %s", addr)
	return e.Start(addr)
}
