package wal

import "io"

type Writer interface {
	Append(record []byte) error
	Flush() error
	Sync() error
	Close() error
}

type Reader interface {
	Next() ([]byte, error)
	Close() error
}

type SnapshotWriter interface {
	io.Writer
	Sync() error
	Close() error
}

type SnapshotReader interface {
	io.Reader
	Close() error
}
