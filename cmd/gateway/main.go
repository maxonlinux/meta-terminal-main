package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/maxonlinux/meta-terminal-go/internal/mm"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/plan"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/internal/wallets"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/nats-io/nats.go"
	"github.com/robaho/fixed"
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

	ob, err := outbox.OpenWithOptions(cfg.DataDir, outbox.Options{EventSink: batchSink, SegmentSize: cfg.OutboxSegmentSize})
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
	planService, err := plan.NewService(planRepo, reg)
	if err != nil {
		log.Fatalf("plan service: %v", err)
	}

	walletRepo, err := wallets.NewRepository(persistenceStore.DB())
	if err != nil {
		log.Fatalf("wallet repo: %v", err)
	}
	walletService := wallets.NewService(walletRepo)

	eng, err := engine.NewEngine(ob, reg, nil)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}
	eng.SetPlanPolicy(planService)
	if err := persistenceStore.LoadCore(eng.Store(), eng.Portfolio()); err != nil {
		log.Fatalf("load core: %v", err)
	}
	eng.RebuildBooks()

	mmaker := mm.New(eng, reg, mm.Config{})
	mmaker.Start(ctx)
	startSystemOrderCleanup(ctx, persistenceStore, 2*time.Minute)
	natsConn, err := startPriceSubscriber(ctx, cfg, eng, mmaker)
	if err != nil {
		log.Fatalf("nats price subscriber: %v", err)
	}
	defer func() {
		if natsConn != nil {
			_ = natsConn.Drain()
			natsConn.Close()
		}
	}()

	ob.Start()

	kycRepo, err := kyc.NewRepository(persistenceStore.DB())
	if err != nil {
		log.Fatalf("kyc repo: %v", err)
	}

	go func() {
		if err := runServer(eng, cfg, persistenceStore, planService, planRepo, walletService, kycRepo); err != nil {
			log.Printf("gateway error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutdown signal received")
	eng.Shutdown()
}

// startSystemOrderCleanup periodically removes closed system orders from history.
func startSystemOrderCleanup(ctx context.Context, store *persistence.Store, interval time.Duration) {
	if store == nil {
		return
	}
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := utils.NowNano() - uint64(interval)
				if _, err := store.CleanupSystemOrders(cutoff); err != nil {
					log.Printf("system order cleanup failed: %v", err)
				}
			}
		}
	}()
}

type priceMessage struct {
	Symbol    string      `json:"symbol"`
	Price     json.Number `json:"price"`
	Timestamp int64       `json:"timestamp"`
}

func startPriceSubscriber(ctx context.Context, cfg config.Config, eng *engine.Engine, mmaker *mm.MarketMaker) (*nats.Conn, error) {
	if cfg.NatsURL == "" {
		return nil, nil
	}

	options := []nats.Option{
		nats.Name("meta-terminal-go-price-subscriber"),
	}
	if cfg.NatsToken != "" {
		options = append(options, nats.Token(cfg.NatsToken))
	}

	nc, err := nats.Connect(cfg.NatsURL, options...)
	if err != nil {
		return nil, err
	}

	_, err = nc.Subscribe(cfg.NatsPriceSubject, func(msg *nats.Msg) {
		var payload priceMessage
		decoder := json.NewDecoder(bytes.NewReader(msg.Data))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			log.Printf("nats price decode failed subject=%s err=%v", msg.Subject, err)
			return
		}
		if payload.Symbol == "" {
			log.Printf("nats price missing symbol subject=%s", msg.Subject)
			return
		}
		priceValue := payload.Price.String()
		if priceValue == "" {
			log.Printf("nats price missing price subject=%s symbol=%s", msg.Subject, payload.Symbol)
			return
		}
		parsed, err := fixed.Parse(priceValue)
		if err != nil {
			log.Printf("nats price parse failed subject=%s symbol=%s price=%s err=%v", msg.Subject, payload.Symbol, priceValue, err)
			return
		}
		price := types.Price(parsed)
		if math.Sign(price) <= 0 {
			log.Printf("nats price invalid subject=%s symbol=%s price=%s", msg.Subject, payload.Symbol, price.String())
			return
		}
		eng.OnPriceTick(payload.Symbol, price)
		if mmaker != nil {
			mmaker.OnPriceTick(payload.Symbol, price)
		}
	})
	if err != nil {
		nc.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		_ = nc.Drain()
		nc.Close()
	}()

	return nc, nil
}

func runServer(eng *engine.Engine, cfg config.Config, persistenceStore *persistence.Store, planService *plan.Service, planRepo *plan.Repository, walletService *wallets.Service, kycRepo *kyc.Repository) error {
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

	wsHandler := wsapi.NewWsHandler(eng.ReadBook, jwtService, cfg.JwtCookieName)
	eng.SetPublisher(wsHandler.Publisher())
	router, err := apihandlers.NewRouter(eng, persistenceStore, jwtService, authService, otpService, impService, planService, planRepo, walletService, kycRepo, wsHandler, cfg)
	if err != nil {
		return err
	}

	e := echo.New()

	router.Register(e)

	addr := ":" + cfg.Port
	log.Printf("http server listening on %s", addr)
	return e.Start(addr)
}
