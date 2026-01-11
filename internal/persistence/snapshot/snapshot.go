package snapshot

import "io"

type Writer interface {
	io.Writer
	Sync() error
	Close() error
}

type Reader interface {
	io.Reader
	Close() error
}
