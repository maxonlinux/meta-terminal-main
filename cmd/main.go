package main

import (
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	"github.com/anomalyco/meta-terminal-go/config"
	httpapi "github.com/anomalyco/meta-terminal-go/internal/api/http"
	"github.com/anomalyco/meta-terminal-go/internal/api/ws"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/idgen"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/market"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/orderstore"
	"github.com/anomalyco/meta-terminal-go/internal/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/persistence"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/snapshot"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/storage"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	orders := orderstore.New()
	idGen := idgen.NewSnowflake(uint16(cfg.SnowflakeNodeID))
	books := orderbook.NewStateWithIDGenerator(idGen)
	users := state.NewUsers()
	reg := registry.New()
	triggers := state.NewTriggers()

	db, err := storage.OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("sqlite init failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()
	if err := storage.EnsureSchema(db); err != nil {
		log.Fatalf("schema init failed: %v", err)
	}

	hist := history.NewSQLite(db)
	out := outbox.NewSQLite(db, cfg.OutboxBatchSize, time.Duration(cfg.OutboxFlushDuration)*time.Second)
	if err != nil {
		log.Fatalf("outbox init failed: %v", err)
	}

	spotClearing := spot.NewClearing(users, reg)
	spotMarket := spot.NewMarket(books, spotClearing)

	linearValidator := linear.NewValidator(users)
	linearClearing := linear.NewClearing(users, reg)
	linearMarket := linear.NewMarket(books, linearValidator, linearClearing)

	markets := map[int8]market.Market{
		spotMarket.GetCategory():   spotMarket,
		linearMarket.GetCategory(): linearMarket,
	}

	eng := engine.New(orders, books, users, reg, triggers, hist, out, markets, idGen)

	walStore, err := wal.New(cfg.WALPath, cfg.WALMaxSize, cfg.WALBufferSize)
	if err != nil {
		log.Fatalf("wal init failed: %v", err)
	}
	snapStore, err := snapshot.New(cfg.SnapshotPath)
	if err != nil {
		log.Fatalf("snapshot init failed: %v", err)
	}
	persist := persistence.New(eng, walStore, snapStore, cfg.WALMaxEvents, cfg.WALMaxSize, time.Duration(cfg.SnapshotInterval)*time.Second)
	if err := persist.Replay(); err != nil {
		log.Fatalf("replay failed: %v", err)
	}
	persist.Start()

	hub := ws.NewHub(books)
	eng.SetEventSink(hub)

	apiServer := httpapi.NewServer(eng, persist, reg)
	mux := http.NewServeMux()
	apiServer.Register(mux)
	mux.Handle("/ws", hub)

	log.Printf("listening on %s", cfg.ServerAddr)
	if err := http.ListenAndServe(cfg.ServerAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
