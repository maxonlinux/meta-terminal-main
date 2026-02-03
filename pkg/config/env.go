package config

import (
	"os"
	"sync"
	"time"
)

type Config struct {
	DataDir        string
	AssetsURL      string
	MultiplexerURL string
	SyncInterval   time.Duration
}

var (
	cfg  Config
	once sync.Once
)

func Load() Config {
	once.Do(func() {
		cfg = Config{
			DataDir:        envString("DATA_DIR", "data"),
			AssetsURL:      envString("ASSETS_URL", "http://localhost:3333/proxy/core/assets"),
			MultiplexerURL: envString("MULTIPLEXER_URL", "http://localhost:3333/proxy/multiplexer/prices"),
			SyncInterval:   envDuration("SYNC_INTERVAL", time.Minute),
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
