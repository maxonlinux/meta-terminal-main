package duckdb

import (
	"io"

	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/outbox"
)

type Loader struct {
	store      *Store
	reader     *outbox.Reader
	offset     int64
	offsetPath string
	buf        []byte
	batch      []history.Record
	maxBatch   int
}

func NewLoaderFromPath(store *Store, outboxPath, offsetPath string, bufSize, maxBatch int) (*Loader, error) {
	offset := int64(0)
	if offsetPath != "" {
		v, err := outbox.ReadOffsetOrZero(offsetPath)
		if err != nil {
			return nil, err
		}
		offset = v
	}
	reader, err := outbox.OpenReaderAt(outboxPath, bufSize, offset)
	if err != nil {
		return nil, err
	}
	loader := NewLoader(store, reader, offsetPath, maxBatch)
	loader.offset = offset
	return loader, nil
}

func NewLoader(store *Store, reader *outbox.Reader, offsetPath string, maxBatch int) *Loader {
	if maxBatch <= 0 {
		maxBatch = 1024
	}
	return &Loader{
		store:      store,
		reader:     reader,
		offsetPath: offsetPath,
		maxBatch:   maxBatch,
	}
}

func (l *Loader) Drain() (int, error) {
	l.batch = l.batch[:0]
	for len(l.batch) < l.maxBatch {
		kind, payload, err := l.reader.NextInto(l.buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return len(l.batch), err
		}
		l.buf = payload
		rec, err := history.DecodeRecord(kind, payload)
		if err != nil {
			return len(l.batch), err
		}
		l.batch = append(l.batch, rec)
	}
	if len(l.batch) == 0 {
		return 0, nil
	}
	if err := l.store.InsertBatch(l.batch); err != nil {
		return len(l.batch), err
	}
	l.offset = l.reader.Offset()
	if l.offsetPath != "" {
		if err := outbox.WriteOffset(l.offsetPath, l.offset); err != nil {
			return len(l.batch), err
		}
	}
	return len(l.batch), nil
}

func (l *Loader) Offset() int64 {
	return l.offset
}
