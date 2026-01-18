package config

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// NATS defines JetStream runtime configuration.
type NATS struct {
	URL               string
	Stream            string
	EventSubject      string
	OrderSubject      string
	TradeSubject      string
	AsyncMaxPending   int
	PublishAckTimeout time.Duration
	BatchInterval     time.Duration
	BatchMaxItems     int
	QueueSize         int
}

// WAL defines persistence WAL tuning values.
type WAL struct {
	BatchInterval time.Duration
	BatchMaxItems int
	BatchMaxBytes int
	QueueSize     int
}

// Config holds environment configuration values.
type Config struct {
	NATS NATS
	WAL  WAL
}

var (
	cfg  Config
	once sync.Once
)

// Load returns the singleton configuration parsed from env.
func Load() Config {
	once.Do(func() {
		cfg = Config{
			NATS: NATS{
				URL:               envString("NATS_URL", "nats://127.0.0.1:4222"),
				Stream:            envString("NATS_STREAM", "orders"),
				EventSubject:      envString("NATS_EVENT_SUBJECT", "events"),
				OrderSubject:      envString("NATS_ORDER_SUBJECT", "orders.events"),
				TradeSubject:      envString("NATS_TRADE_SUBJECT", "trades.events"),
				AsyncMaxPending:   envInt("NATS_ASYNC_MAX_PENDING", 4096),
				PublishAckTimeout: envDuration("NATS_PUBLISH_ACK_TIMEOUT", 2*time.Second),
				BatchInterval:     envDuration("NATS_BATCH_INTERVAL", 500*time.Microsecond),
				BatchMaxItems:     envInt("NATS_BATCH_MAX_ITEMS", 4096),
				QueueSize:         envInt("NATS_QUEUE_SIZE", 8192),
			},
			WAL: WAL{
				BatchInterval: envDuration("WAL_BATCH_INTERVAL", 500*time.Microsecond),
				BatchMaxItems: envInt("WAL_BATCH_MAX_ITEMS", 4096),
				BatchMaxBytes: envInt("WAL_BATCH_MAX_BYTES", 4<<20),
				QueueSize:     envInt("WAL_QUEUE_SIZE", 8192),
			},
		}
	})
	return cfg
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
