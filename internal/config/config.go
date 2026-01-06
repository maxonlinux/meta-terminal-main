package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerHost string
	ServerPort int
	ServerMode string

	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBName        string
	DBTablePrefix string

	RedisHost      string
	RedisPort      int
	RedisPassword  string
	RedisDB        int
	RedisKeyPrefix string

	NATSURL               string
	NATSMarketDataSubject string
	NATSMarketDataQueue   string

	CoreBaseURL string
	CoreAPIKey  string

	LogLevel  string
	LogFormat string

	OrderbookShards int

	SnapshotInterval int
	SnapshotPath     string

	OutboxPath          string
	OutboxBatchSize     int
	OutboxFlushInterval int
	OutboxFlushDuration time.Duration

	MaxMarkets           int
	MaxUsersPerMarket    int
	MaxOpenOrdersPerUser int

	WALPath       string
	WALBufferSize int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort: getEnvInt("SERVER_PORT", 3102),
		ServerMode: getEnv("SERVER_MODE", "debug"),

		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnvInt("DB_PORT", 5432),
		DBUser:        getEnv("DB_USER", "postgres"),
		DBPassword:    getEnv("DB_PASSWORD", "postgres"),
		DBName:        getEnv("DB_NAME", "trading_engine"),
		DBTablePrefix: getEnv("DB_TABLE_PREFIX", "sess_1e7e3c6a_"),

		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      getEnvInt("REDIS_PORT", 6379),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		RedisKeyPrefix: getEnv("REDIS_KEY_PREFIX", "sess:1e7e3c6a:"),

		NATSURL:               getEnv("NATS_URL", "nats://localhost:4222"),
		NATSMarketDataSubject: getEnv("NATS_MARKET_DATA_SUBJECT", "market.price"),
		NATSMarketDataQueue:   getEnv("NATS_MARKET_DATA_QUEUE", "trading-engine"),

		CoreBaseURL: getEnv("CORE_BASE_URL", "http://localhost:8081"),
		CoreAPIKey:  getEnv("CORE_API_KEY", ""),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),

		OrderbookShards: getEnvInt("ORDERBOOK_SHARDS", 4),

		SnapshotInterval: getEnvInt("SNAPSHOT_INTERVAL", 300),
		SnapshotPath:     getEnv("SNAPSHOT_PATH", "./data/snapshots"),

		OutboxPath:          getEnv("OUTBOX_PATH", "./data/outbox"),
		OutboxBatchSize:     getEnvInt("OUTBOX_BATCH_SIZE", 100),
		OutboxFlushInterval: getEnvInt("OUTBOX_FLUSH_INTERVAL", 5),
		OutboxFlushDuration: time.Duration(getEnvInt("OUTBOX_FLUSH_INTERVAL", 5)) * time.Second,

		MaxMarkets:           getEnvInt("MAX_MARKETS", 2000),
		MaxUsersPerMarket:    getEnvInt("MAX_USERS_PER_MARKET", 500),
		MaxOpenOrdersPerUser: getEnvInt("MAX_OPEN_ORDERS_PER_USER", 100),

		WALPath:       getEnv("WAL_PATH", "./data/wal"),
		WALBufferSize: getEnvInt("WAL_BUFFER_SIZE", 64),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
