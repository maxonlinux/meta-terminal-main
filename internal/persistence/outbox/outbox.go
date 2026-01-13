package outbox

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"
)

const HEADER_SIZE = 5

var errInvalidLength = errors.New("outbox: invalid record length")

// Writer provides thread-safe append operations to an outbox file.
// The mutex ensures that concurrent writes do not interleave, which would
// corrupt the binary file format.
type Writer struct {
	f  *os.File
	w  *bufio.Writer
	mu sync.Mutex
}

func OpenWriter(path string, bufSize int) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	return &Writer{
		f: f,
		w: bufio.NewWriterSize(f, bufSize),
	}, nil
}

// Append writes a new record to the outbox file.
// Thread-safe: uses mutex to prevent concurrent write interleaving.
func (w *Writer) Append(kind byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var header [HEADER_SIZE]byte
	header[0] = kind
	binary.LittleEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := w.w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.w.Write(payload)
	return err
}

func (w *Writer) Flush() error {
	return w.w.Flush()
}

func (w *Writer) Sync() error {
	if err := w.w.Flush(); err != nil {
		return err
	}
	return w.f.Sync()
}

func (w *Writer) Close() error {
	if err := w.w.Flush(); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}

type Reader struct {
	f   *os.File
	r   *bufio.Reader
	pos int64
}

func OpenReader(path string, bufSize int) (*Reader, error) {
	return OpenReaderAt(path, bufSize, 0)
}

func OpenReaderAt(path string, bufSize int, offset int64) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	return &Reader{
		f:   f,
		r:   bufio.NewReaderSize(f, bufSize),
		pos: offset,
	}, nil
}

func (r *Reader) Next() (byte, []byte, error) {
	var header [HEADER_SIZE]byte
	if _, err := io.ReadFull(r.r, header[:]); err != nil {
		return 0, nil, err
	}
	size := binary.LittleEndian.Uint32(header[1:])
	if size == 0 {
		return header[0], nil, errInvalidLength
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r.r, payload); err != nil {
		return 0, nil, err
	}
	r.pos += HEADER_SIZE + int64(size)
	return header[0], payload, nil
}

func (r *Reader) NextInto(buf []byte) (byte, []byte, error) {
	var header [HEADER_SIZE]byte
	if _, err := io.ReadFull(r.r, header[:]); err != nil {
		return 0, nil, err
	}
	size := int(binary.LittleEndian.Uint32(header[1:]))
	if size <= 0 {
		return header[0], nil, errInvalidLength
	}
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return 0, nil, err
	}
	r.pos += HEADER_SIZE + int64(size)
	return header[0], buf, nil
}

func (r *Reader) Offset() int64 {
	return r.pos
}

func (r *Reader) Close() error {
	return r.f.Close()
}
