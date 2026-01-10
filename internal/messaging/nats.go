package messaging

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func init() {
	gob.Register(types.Order{})
	gob.Register(types.Trade{})
	gob.Register(types.Match{})
	gob.Register(types.OrderInput{})
	gob.Register(types.OrderEvent{})
	gob.Register(types.TradeEvent{})
	gob.Register(types.RPNLEvent{})
	gob.Register(types.Position{})
	gob.Register(types.UserBalance{})
}

type Config struct {
	URL          string
	StreamPrefix string
}

type NATS struct {
	conn *nats.Conn
	js   jetstream.JetStream
	cfg  Config
	mu   sync.RWMutex
}

func New(cfg Config) (*NATS, error) {
	nc, err := nats.Connect(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	return &NATS{conn: nc, js: js, cfg: cfg}, nil
}

func (n *NATS) Close()                  { n.conn.Close() }
func (n *NATS) JS() jetstream.JetStream { return n.js }

func (n *NATS) PublishGob(ctx context.Context, subject string, v any) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("gob encode: %w", err)
	}
	_, err := n.js.Publish(ctx, subject, buf.Bytes())
	return err
}

func DecodeGob(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(v)
}

func (n *NATS) PublishBytes(ctx context.Context, subject string, data []byte) error {
	_, err := n.js.Publish(ctx, subject, data)
	return err
}

type Subscription struct {
	cancel context.CancelFunc
}

func (s *Subscription) Close() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

func (n *NATS) Subscribe(ctx context.Context, subject, name string, handler func([]byte)) *Subscription {
	cancelCtx, cancel := context.WithCancel(ctx)

	streamName := n.cfg.StreamPrefix + "_" + name

	stream, _ := n.js.Stream(ctx, streamName)
	if stream == nil {
		stream, _ = n.js.CreateStream(ctx, jetstream.StreamConfig{
			Name:     streamName,
			Subjects: []string{subject},
		})
	}

	cons, _ := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          name,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		Durable:       name,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})

	go func() {
		for {
			select {
			case <-cancelCtx.Done():
				return
			default:
				msgs, err := cons.Fetch(100)
				if err != nil {
					if cancelCtx.Err() != nil {
						return
					}
					continue
				}
				for msg := range msgs.Messages() {
					handler(msg.Data())
					msg.Ack()
				}
			}
		}
	}()

	return &Subscription{cancel: cancel}
}
