package marketdata

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	NATSURL          string
	StreamPrefix     string
	AssetsURL        string
	MultiplexerURL   string
	SyncIntervalSecs int
}

type Service struct {
	cfg    Config
	nats   *messaging.NATS
	oms    *oms.Service
	reg    *registry.Registry
	subs   []*messaging.Subscription
	client *http.Client
}

func New(cfg Config, reg *registry.Registry, omsService *oms.Service) (*Service, error) {
	var n *messaging.NATS
	if cfg.NATSURL != "" {
		var err error
		n, err = messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
		if err != nil {
			return nil, err
		}
	}
	if cfg.SyncIntervalSecs <= 0 {
		cfg.SyncIntervalSecs = 300
	}
	return &Service{
		cfg:    cfg,
		nats:   n,
		oms:    omsService,
		reg:    reg,
		subs:   make([]*messaging.Subscription, 0),
		client: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	if s.nats != nil {
		sub := s.nats.Subscribe(ctx, messaging.SUBJECT_PRICE_TICK+".>", "marketdata-prices", s.handlePriceTick)
		s.subs = append(s.subs, sub)
	}

	if s.cfg.AssetsURL != "" && s.cfg.MultiplexerURL != "" {
		go s.loadAssetsLoop(ctx)
	}

	log.Println("marketdata service started")
	return nil
}

type priceTick struct {
	Symbol    string `json:"symbol"`
	Price     int64  `json:"price"`
	Bid       int64  `json:"bid"`
	Ask       int64  `json:"ask"`
	Volume    int64  `json:"volume"`
	Timestamp int64  `json:"timestamp"`
}

func (s *Service) handlePriceTick(data []byte) {
	var tick priceTick
	if err := messaging.DecodeGob(data, &tick); err != nil {
		log.Printf("marketdata: failed to decode price tick: %v", err)
		return
	}
	if tick.Symbol == "" || tick.Price <= 0 {
		return
	}

	s.reg.SetPrice(tick.Symbol, registry.PriceTick{
		Price:     tick.Price,
		Bid:       tick.Bid,
		Ask:       tick.Ask,
		Volume:    tick.Volume,
		Timestamp: tick.Timestamp,
	})
	if s.oms != nil {
		s.oms.OnPriceTick(tick.Symbol, types.Price(tick.Price))
	}
}

type assetResponse struct {
	Symbol string `json:"symbol"`
}

func (s *Service) loadAssetsLoop(ctx context.Context) {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.AssetsURL, nil)
	if err != nil {
		log.Printf("marketdata: assets request error: %v", err)
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("marketdata: assets fetch error: %v", err)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var assets []assetResponse
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		log.Printf("marketdata: assets decode error: %v", err)
		return
	}

	for i := range assets {
		s.loadInstrument(ctx, assets[i].Symbol)
	}
}

func (s *Service) loadInstrument(ctx context.Context, symbol string) {
	if symbol == "" {
		return
	}
	if s.reg.GetInstrument(symbol) != nil {
		return
	}

	url := strings.TrimRight(s.cfg.MultiplexerURL, "/") + "/prices?symbol=" + symbol
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("marketdata: price request error: %v", err)
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("marketdata: price fetch error for %s: %v", symbol, err)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return
	}

	var priceData struct {
		Price int64 `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&priceData); err != nil {
		log.Printf("marketdata: price decode error for %s: %v", symbol, err)
		return
	}
	if priceData.Price <= 0 {
		return
	}

	inst := registry.FromSymbol(symbol, priceData.Price)
	s.reg.SetInstrument(symbol, inst)

	now := time.Now().UnixMilli()
	s.reg.SetPrice(symbol, registry.PriceTick{
		Price:     priceData.Price,
		Bid:       priceData.Price,
		Ask:       priceData.Price,
		Volume:    0,
		Timestamp: now,
	})
	if s.oms != nil {
		s.oms.OnPriceTick(symbol, types.Price(priceData.Price))
	}
	s.publishPriceTick(symbol, priceData.Price)
}

func (s *Service) Stop() {
	for _, sub := range s.subs {
		sub.Close()
	}
	if s.nats != nil {
		s.nats.Close()
	}
}

func (s *Service) publishPriceTick(symbol string, price int64) {
	if s.nats == nil {
		return
	}
	event := struct {
		Symbol string
		Price  types.Price
	}{
		Symbol: symbol,
		Price:  types.Price(price),
	}
	_ = s.nats.PublishGob(context.Background(), messaging.PriceTickTopic(symbol), event)
}
