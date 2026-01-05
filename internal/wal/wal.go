package wal

import (
	"os"
	"path/filepath"
)

type WAL struct {
	path   string
	file   *os.File
	offset int64
}

func New(path string) (*WAL, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(path, "wal.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &WAL{
		path:   path,
		file:   f,
		offset: 0,
	}, nil
}

func (w *WAL) Start(opID string) error {
	return nil
}

func (w *WAL) Commit(opID string, data []byte) error {
	return nil
}

func (w *WAL) Abort(opID string) error {
	return nil
}

func (w *WAL) Close() error {
	return w.file.Close()
}

func (w *WAL) ReadFrom(offset int64) ([]byte, error) {
	return nil, nil
}
