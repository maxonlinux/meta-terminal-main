package config

import "os"

type Config struct {
	WALPath             string
	WALBufferSize       int
	SnapshotPath        string
	SnapshotInterval    int
	OutboxPath          string
	OutboxBatchSize     int
	OutboxFlushDuration int
}

func Load() (*Config, error) {
	return &Config{
		WALPath:             getEnv("WAL_PATH", "wal"),
		WALBufferSize:       1024 * 1024,
		SnapshotPath:        getEnv("SNAPSHOT_PATH", "snapshots"),
		SnapshotInterval:    60,
		OutboxPath:          getEnv("OUTBOX_PATH", "outbox"),
		OutboxBatchSize:     100,
		OutboxFlushDuration: 5,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
