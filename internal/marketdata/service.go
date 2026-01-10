package marketdata

import (
	"context"
	"log"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	NATSURL      string
	StreamPrefix string
}

type Service struct {
	nats *messaging.NATS
	oms  *oms.Service
}

func New(cfg Config, omsService *oms.Service) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{
		nats: n,
		oms:  omsService,
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.nats.Subscribe(ctx, "price.tick", "marketdata", s.handlePriceTick)
	log.Println("marketdata service started")
	return nil
}

func (s *Service) handlePriceTick(data []byte) {
	var tick struct {
		Symbol string
		Price  types.Price
	}
	if err := messaging.DecodeGob(data, &tick); err != nil {
		log.Printf("marketdata: failed to decode price tick: %v", err)
		return
	}

	s.oms.OnPriceTick(tick.Symbol, tick.Price)

	log.Printf("marketdata: price tick %s @ %d", tick.Symbol, tick.Price)
}

func (s *Service) Stop() {
	if s.nats != nil {
		s.nats.Close()
	}
}
