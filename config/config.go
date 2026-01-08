package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	DataDir             string
	DBPath              string
	WALPath             string
	WALBufferSize       int
	WALMaxSize          int64
	WALMaxEvents        int
	SnapshotPath        string
	SnapshotInterval    int
	OutboxBatchSize     int
	OutboxFlushDuration int
	SnowflakeNodeID     int
	ServerAddr          string
}

func Load() (*Config, error) {
	dataDir := getEnv("DATA_DIR", "data")
	dbPath := getEnv("DB_PATH", filepath.Join(dataDir, "engine.db"))
	walPath := getEnv("WAL_PATH", filepath.Join(dataDir, "wal.log"))
	snapshotPath := getEnv("SNAPSHOT_PATH", filepath.Join(dataDir, "snapshots"))

	return &Config{
		DataDir:             dataDir,
		DBPath:              dbPath,
		WALPath:             walPath,
		WALBufferSize:       getEnvInt("WAL_BUFFER_SIZE", 1024*1024),
		WALMaxSize:          getEnvInt64("WAL_MAX_SIZE", 1024*1024),
		WALMaxEvents:        getEnvInt("WAL_MAX_EVENTS", 1000),
		SnapshotPath:        snapshotPath,
		SnapshotInterval:    getEnvInt("SNAPSHOT_INTERVAL", 3),
		OutboxBatchSize:     getEnvInt("OUTBOX_BATCH_SIZE", 100),
		OutboxFlushDuration: getEnvInt("OUTBOX_FLUSH_DURATION", 5),
		SnowflakeNodeID:     getEnvInt("SNOWFLAKE_NODE_ID", 0),
		ServerAddr:          getEnv("SERVER_ADDR", ":8080"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return n
		}
	}
	return defaultValue
}
