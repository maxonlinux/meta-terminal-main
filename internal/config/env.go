package config

import (
	"os"
	"strconv"
	"strings"
)

type Env struct {
	NATSURL          string
	StreamPrefix     string
	OutboxPath       string
	OutboxOffsetPath string
	OutboxBuf        int
	OutboxBufSize    int
	HistoryBatchSize int
	HistoryPollMs    int
	Port             int
	JWTSecret        string
	JWTCookie        string
	DuckDBPath       string
	AssetsURL        string
	MultiplexerURL   string
	SyncIntervalSecs int
	QuoteAssets      []string
}

func Load() Env {
	quoteAssets := envString("QUOTE_ASSETS", "USDT,USD,USDC,BUSD")
	outboxBuf := envInt("OUTBOX_BUF", 64*1024)
	outboxBufSize := envInt("OUTBOX_BUF_SIZE", outboxBuf)
	return Env{
		NATSURL:          envString("NATS_URL", "nats://localhost:4222"),
		StreamPrefix:     envString("STREAM_PREFIX", "meta"),
		OutboxPath:       envString("OUTBOX_PATH", "data/outbox.log"),
		OutboxOffsetPath: envString("OUTBOX_OFFSET_PATH", "data/outbox.offset"),
		OutboxBuf:        outboxBuf,
		OutboxBufSize:    outboxBufSize,
		HistoryBatchSize: envInt("HISTORY_BATCH_SIZE", 1024),
		HistoryPollMs:    envInt("HISTORY_POLL_MS", 250),
		Port:             envInt("PORT", 8080),
		JWTSecret:        envString("JWT_SECRET", ""),
		JWTCookie:        envString("JWT_COOKIE", "token"),
		DuckDBPath:       envString("DUCKDB_PATH", "data/history.duckdb"),
		AssetsURL:        envString("ASSETS_URL", ""),
		MultiplexerURL:   envString("MULTIPLEXER_URL", ""),
		SyncIntervalSecs: envInt("SYNC_INTERVAL_SECS", 300),
		QuoteAssets:      parseCSV(quoteAssets),
	}
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func parseCSV(input string) []string {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
