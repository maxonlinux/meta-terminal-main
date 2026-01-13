package engine

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/clearing"
	"github.com/anomalyco/meta-terminal-go/internal/events"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/portfolio"
)

type Config struct {
	NATSURL      string
	StreamPrefix string
	OutboxPath   string
	OutboxBuf    int
}

type Engine struct {
	OMS       *oms.ActorOMS
	Portfolio *portfolio.Service
	Clearing  *clearing.Service
	Outbox    *history.OutboxWriter
	nats      *messaging.NATS
}

func New(cfg Config) (*Engine, error) {
	var n *messaging.NATS
	if cfg.NATSURL != "" {
		var err error
		n, err = messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
		if err != nil {
			return nil, err
		}
	}

	outbox, err := history.NewOutboxWriter(history.OutboxConfig{
		Path:    cfg.OutboxPath,
		BufSize: cfg.OutboxBuf,
		NATS:    n,
	})
	if err != nil {
		if n != nil {
			n.Close()
		}
		return nil, err
	}

	sink := events.Sink(outbox)
	port := portfolio.New(portfolio.Config{NATS: n, Sink: sink})
	clear := clearing.New(port)
	omsActor, err := oms.NewActorOMS(oms.Config{
		NATSURL:      cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
		Sink:         sink,
	}, port, clear)
	if err != nil {
		_ = outbox.Close()
		if n != nil {
			n.Close()
		}
		return nil, err
	}

	return &Engine{
		OMS:       omsActor,
		Portfolio: port,
		Clearing:  clear,
		Outbox:    outbox,
		nats:      n,
	}, nil
}

func (e *Engine) Start(ctx context.Context) error {
	e.Outbox.Start(ctx)
	return e.OMS.Start(ctx)
}

func (e *Engine) Close() error {
	if e.Outbox != nil {
		_ = e.Outbox.Close()
	}
	if e.nats != nil {
		e.nats.Close()
	}
	return nil
}
