package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/grafana/pyroscope-go"
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
	"github.com/maxonlinux/meta-terminal-go/pkg/logging"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/nats-io/nats.go"
	"github.com/robaho/fixed"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	if envMap, err := godotenv.Read(); err != nil {
		logging.Log().Warn().Err(err).Msg("env file not loaded")
	} else {
		for key, value := range envMap {
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
	}

	cfg := config.Load()
	if _, err := logging.Init(cfg.LogLevel, cfg.LogFormat); err != nil {
		log.Fatalf("logger init: %v", err)
	}
	if err := snowflake.Init(cfg.SnowflakeNode); err != nil {
		log.Fatalf("snowflake init: %v", err)
	}

	if os.Getenv("PYROSCOPE_ADDRESS") != "" {
		if _, err := pyroscope.Start(pyroscope.Config{
			ApplicationName: os.Getenv("PYROSCOPE_APP_NAME"),
			ServerAddress:   os.Getenv("PYROSCOPE_ADDRESS"),
			Logger:          pyroscope.StandardLogger,
		}); err != nil {
			logging.Log().Warn().Err(err).Msg("pyroscope init failed")
		} else {
			logging.Log().Info().Str("address", os.Getenv("PYROSCOPE_ADDRESS")).Msg("pyroscope started")
		}
	}

	logging.Log().Info().Str("data_dir", cfg.DataDir).Str("port", cfg.Port).Msg("gateway started")

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

	eng, err := engine.NewEngine(ob, reg, nil)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}
	eng.SetPlanPolicy(planService)
	if err := persistenceStore.LoadCore(eng.Store(), eng.Portfolio()); err != nil {
		log.Fatalf("load core: %v", err)
	}
	eng.RebuildBooks()

	mmaker := mm.New(eng, reg, mm.Config{
		BotUserID:  types.UserID(cfg.BotUserID),
		MinBalance: cfg.BotMinBalance,
		MaxBalance: cfg.BotMaxBalance,
	})
	mmaker.Start(ctx)
	startSystemOrderCleanup(ctx, persistenceStore, 1*time.Minute)
	startBotDataCleanup(ctx, persistenceStore, cfg.BotUserID, 1*time.Minute)
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
		if err := runServer(eng, cfg, persistenceStore, planService, planRepo, kycRepo); err != nil {
			logging.Log().Error().Err(err).Msg("gateway error")
		}
	}()

	<-ctx.Done()
	logging.Log().Info().Msg("shutdown signal received")
	eng.Shutdown()
}

// startSystemOrderCleanup periodically removes closed system orders from history.
func startSystemOrderCleanup(ctx context.Context, store *persistence.Store, interval time.Duration) {
	if store == nil {
		return
	}
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupMu.Lock()
				cutoff := utils.NowNano() - uint64(interval)
				if _, err := store.CleanupSystemOrders(cutoff); err != nil {
					logging.Log().Error().Err(err).Msg("system order cleanup failed")
				}
				cleanupMu.Unlock()
			}
		}
	}()
}

var cleanupMu sync.Mutex

// startBotDataCleanup periodically removes old bot data from all tables.
func startBotDataCleanup(ctx context.Context, store *persistence.Store, botUserID int64, interval time.Duration) {
	if store == nil || botUserID == 0 {
		return
	}
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	botID := types.UserID(botUserID)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupMu.Lock()
				cutoff := utils.NowNano() - uint64(interval)
				if count, err := store.CleanupBotData(botID, cutoff); err != nil {
					logging.Log().Error().Err(err).Int64("bot_id", botUserID).Msg("bot data cleanup failed")
				} else if count > 0 {
					logging.Log().Info().Int64("bot_id", botUserID).Int64("deleted", count).Msg("bot data cleaned up")
				}
				cleanupMu.Unlock()
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
			logging.Log().Error().Str("subject", msg.Subject).Err(err).Msg("nats price decode failed")
			return
		}
		if payload.Symbol == "" {
			logging.Log().Warn().Str("subject", msg.Subject).Msg("nats price missing symbol")
			return
		}
		priceValue := payload.Price.String()
		if priceValue == "" {
			logging.Log().Warn().Str("subject", msg.Subject).Str("symbol", payload.Symbol).Msg("nats price missing price")
			return
		}
		parsed, err := fixed.Parse(priceValue)
		if err != nil {
			logging.Log().Error().Str("subject", msg.Subject).Str("symbol", payload.Symbol).Str("price", priceValue).Err(err).Msg("nats price parse failed")
			return
		}
		price := types.Price(parsed)
		if math.Sign(price) <= 0 {
			logging.Log().Warn().Str("subject", msg.Subject).Str("symbol", payload.Symbol).Str("price", price.String()).Msg("nats price invalid")
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

func runServer(eng *engine.Engine, cfg config.Config, persistenceStore *persistence.Store, planService *plan.Service, planRepo *plan.Repository, kycRepo *kyc.Repository) error {
	userStore, err := users.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		return err
	}
	defer func() {
		_ = userStore.Close()
	}()

	walletRepo, err := wallets.NewRepository(userStore.DB())
	if err != nil {
		return err
	}
	walletService := wallets.NewService(walletRepo)

	jwtService, err := auth.NewJWTService(cfg)
	if err != nil {
		log.Fatalf("jwt service: %v", err)
	}
	authService := users.NewService(userStore)
	otpService := otp.NewService(otp.Config{
		SiteName:       cfg.SiteName,
		SmsAuthToken:   cfg.SmsAuthToken,
		SmtpHost:       cfg.SmtpHost,
		SmtpPort:       cfg.SmtpPort,
		SmtpUser:       cfg.SmtpUser,
		SmtpPassword:   cfg.SmtpPassword,
		SmtpFrom:       cfg.SmtpFrom,
		SmtpSkipVerify: cfg.SmtpSkipVerify,
	})
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
	logging.Log().Info().Str("addr", addr).Msg("http server listening")
	return e.Start(addr)
}
