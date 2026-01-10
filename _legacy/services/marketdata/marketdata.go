package marketdata

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
)

type Config struct {
	NATSURL          string `env:"NATS_URL" default:"nats://localhost:4222"`
	StreamPrefix     string `env:"STREAM_PREFIX" default:"marketdata"`
	AssetsURL        string `env:"ASSETS_URL" default:"http://146.103.123.216:3000/assets"`
	MultiplexerURL   string `env:"MULTIPLEXER_URL" default:"http://localhost:3333/proxy/multiplexer"`
	SyncIntervalSecs int    `env:"SYNC_INTERVAL_SECS" default:"300"`
}

type PriceTick struct {
	Symbol    string `json:"symbol"`
	Price     int64  `json:"price"`
	Bid       int64  `json:"bid"`
	Ask       int64  `json:"ask"`
	Volume    int64  `json:"volume"`
	Timestamp int64  `json:"timestamp"`
}

type Ticker struct {
	Symbol    string `json:"symbol"`
	Price     int64  `json:"price"`
	Change24h int64  `json:"change_24h"`
	Volume24h int64  `json:"volume_24h"`
	High24h   int64  `json:"high_24h"`
	Low24h    int64  `json:"low_24h"`
	UpdatedAt int64  `json:"updated_at"`
}

type Service struct {
	cfg     Config
	nats    *messaging.NATS
	reg     *registry.Registry
	prices  map[string]*PriceTick
	tickers map[string]*Ticker
	mu      sync.RWMutex

	subs       []*messaging.Subscription
	publishers map[string]*time.Ticker
}

func New(cfg Config, reg *registry.Registry) (*Service, error) {
	n, err := messaging.New(messaging.Config{
		URL:          cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	})
	if err != nil {
		return nil, err
	}

	return &Service{
		cfg:        cfg,
		nats:       n,
		reg:        reg,
		prices:     make(map[string]*PriceTick),
		tickers:    make(map[string]*Ticker),
		subs:       make([]*messaging.Subscription, 0),
		publishers: make(map[string]*time.Ticker),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	sub := s.nats.Subscribe(ctx, messaging.SubjectPriceTick+".>", "marketdata-prices", s.handlePriceTick)
	s.subs = append(s.subs, sub)

	go s.loadAssets(ctx)
	go s.publishPriceSnapshots(ctx)

	log.Println("marketdata service started")
	return nil
}

type AssetResponse struct {
	Symbol string `json:"symbol"`
}

func (s *Service) loadAssets(ctx context.Context) {
	s.syncAssets(ctx)

	ticker := time.NewTicker(time.Duration(s.cfg.SyncIntervalSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAssets(ctx)
		}
	}
}

func (s *Service) syncAssets(ctx context.Context) {
	resp, err := http.Get(s.cfg.AssetsURL)
	if err != nil {
		log.Printf("failed to fetch assets: %v", err)
		return
	}
	defer resp.Body.Close()

	var assets []AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		log.Printf("failed to decode assets: %v", err)
		return
	}

	for _, asset := range assets {
		s.loadInstrument(ctx, asset.Symbol)
	}
}

func (s *Service) loadInstrument(ctx context.Context, symbol string) {
	url := s.cfg.MultiplexerURL + "/prices?symbol=" + symbol
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to fetch price for %s: %v", symbol, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return
	}

	var priceData struct {
		Price int64 `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&priceData); err != nil {
		log.Printf("failed to decode price for %s: %v", symbol, err)
		return
	}

	if priceData.Price <= 0 {
		return
	}

	base, quote := s.parseBaseQuote(symbol)

	inst := &registry.Instrument{
		Symbol:     symbol,
		Category:   1,
		BaseAsset:  base,
		QuoteAsset: quote,
		TickSize:   1,
		LotSize:    1,
		MinQty:     1,
	}
	s.reg.SetInstrument(symbol, inst)

	s.reg.SetPrice(symbol, registry.PriceTick{
		Price:     priceData.Price,
		Bid:       priceData.Price,
		Ask:       priceData.Price,
		Volume:    0,
		Timestamp: time.Now().UnixMilli(),
	})

	log.Printf("loaded instrument: %s price=%d", symbol, priceData.Price)
}

func (s *Service) parseBaseQuote(symbol string) (string, string) {
	quotes := []string{"USDT", "USD", "USDC", "BUSD", "BTC", "ETH"}
	for _, q := range quotes {
		if strings.HasSuffix(symbol, q) && len(symbol) > len(q) {
			return strings.TrimSuffix(symbol, q), q
		}
	}
	return symbol, "USD"
}

func (s *Service) handlePriceTick(data []byte) {
	var tick PriceTick
	if err := json.Unmarshal(data, &tick); err != nil {
		log.Printf("price tick parse error: %v", err)
		return
	}

	s.mu.Lock()
	s.prices[tick.Symbol] = &tick

	t := s.tickers[tick.Symbol]
	if t == nil {
		t = &Ticker{
			Symbol:    tick.Symbol,
			Price:     tick.Price,
			Change24h: 0,
			Volume24h: tick.Volume,
			High24h:   tick.Price,
			Low24h:    tick.Price,
		}
	} else {
		oldPrice := t.Price
		t.Price = tick.Price
		t.Change24h = tick.Price - oldPrice
		t.Volume24h += tick.Volume
		if tick.Price > t.High24h {
			t.High24h = tick.Price
		}
		if tick.Price < t.Low24h {
			t.Low24h = tick.Price
		}
	}
	t.UpdatedAt = tick.Timestamp
	s.tickers[tick.Symbol] = t
	s.mu.Unlock()
}

func (s *Service) publishPriceSnapshots(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			snapshots := make(map[string]PriceTick)
			for sym, tick := range s.prices {
				snapshots[sym] = *tick
			}
			s.mu.RUnlock()

			for sym, tick := range snapshots {
				s.nats.Publish(ctx, messaging.MarketDataSnapshotTopic(sym), map[string]interface{}{
					"tick": tick,
				})
			}
		}
	}
}

func (s *Service) GetPrice(symbol string) (PriceTick, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tick, ok := s.prices[symbol]
	if !ok {
		return PriceTick{}, false
	}
	return *tick, true
}

func (s *Service) GetTicker(symbol string) (Ticker, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tickers[symbol]
	if !ok {
		return Ticker{}, false
	}
	return *t, true
}

func (s *Service) GetAllTickers() map[string]Ticker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]Ticker)
	for k, v := range s.tickers {
		result[k] = *v
	}
	return result
}

func (s *Service) GetAllPrices() map[string]PriceTick {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]PriceTick)
	for k, v := range s.prices {
		result[k] = *v
	}
	return result
}

func (s *Service) Close() {
	for _, sub := range s.subs {
		sub.Close()
	}
	for _, t := range s.publishers {
		t.Stop()
	}
	s.nats.Close()
}
