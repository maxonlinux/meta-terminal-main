package messaging

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

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

func (n *NATS) Publish(ctx context.Context, subject string, data any) error {
	b, err := Encode(data)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	_, err = n.js.Publish(ctx, subject, b)
	return err
}

func (n *NATS) PublishBytes(ctx context.Context, subject string, data []byte) error {
	_, err := n.js.Publish(ctx, subject, data)
	return err
}

func (n *NATS) RequestReply(ctx context.Context, subject string, data []byte, timeout time.Duration) ([]byte, error) {
	msg, err := n.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		return nil, err
	}
	return msg.Data, nil
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

var endian = binary.LittleEndian

func Encode(v any) ([]byte, error) {
	switch x := v.(type) {
	case int64:
		b := make([]byte, 9)
		b[0] = 0x21
		endian.PutUint64(b[1:], uint64(x))
		return b, nil
	case int32:
		b := make([]byte, 5)
		b[0] = 0x20
		endian.PutUint32(b[1:], uint32(x))
		return b, nil
	case int:
		b := make([]byte, 9)
		b[0] = 0x21
		endian.PutUint64(b[1:], uint64(x))
		return b, nil
	case uint64:
		b := make([]byte, 9)
		b[0] = 0x22
		endian.PutUint64(b[1:], x)
		return b, nil
	case uint32:
		b := make([]byte, 5)
		b[0] = 0x22
		endian.PutUint32(b[1:], x)
		return b, nil
	case uint:
		b := make([]byte, 9)
		b[0] = 0x22
		endian.PutUint64(b[1:], uint64(x))
		return b, nil
	case string:
		b := make([]byte, 1+len(x))
		b[0] = 0xa0 | byte(len(x))
		copy(b[1:], x)
		return b, nil
	case []byte:
		b := make([]byte, 1+len(x))
		b[0] = 0xc4
		endian.PutUint32(b[1:], uint32(len(x)))
		copy(b[5:], x)
		return b, nil
	case bool:
		if x {
			return []byte{0xc3}, nil
		}
		return []byte{0xc2}, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}
