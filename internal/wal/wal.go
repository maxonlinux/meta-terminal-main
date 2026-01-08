package wal

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	magicHeader = "WAL1"
	headerSize  = 8
)

type WAL struct {
	mu      sync.Mutex
	path    string
	file    *os.File
	writer  *bufio.Writer
	size    int64
	events  int
	maxSize int64
}

func New(path string, maxSize int64, bufferSize int) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if bufferSize <= 0 {
		bufferSize = 64 * 1024
	}
	w := &WAL{
		path:    path,
		file:    file,
		writer:  bufio.NewWriterSize(file, bufferSize),
		maxSize: maxSize,
	}
	if err := w.loadHeader(); err != nil {
		return nil, err
	}
	if err := w.scan(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *WAL) Append(eventType uint8, payload []byte) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	length := uint32(1 + 8 + len(payload))
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], length)
	offset := w.size
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return 0, err
	}
	if _, err := w.writer.Write(header[:]); err != nil {
		return 0, err
	}
	if err := w.writer.WriteByte(eventType); err != nil {
		return 0, err
	}
	var tsBuf [8]byte
	binary.LittleEndian.PutUint64(tsBuf[:], uint64(time.Now().UnixNano()))
	if _, err := w.writer.Write(tsBuf[:]); err != nil {
		return 0, err
	}
	if len(payload) > 0 {
		if _, err := w.writer.Write(payload); err != nil {
			return 0, err
		}
	}
	if err := w.writer.Flush(); err != nil {
		return 0, err
	}
	w.size += int64(4 + length)
	w.events++
	return offset, nil
}

func (w *WAL) Replay(handler func(Event) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(w.file, 64*1024)
	if err := w.readHeader(reader); err != nil {
		return err
	}
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(reader, lenBuf[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}
		length := binary.LittleEndian.Uint32(lenBuf[:])
		if length == 0 {
			return nil
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}
		event := Event{
			Type:      payload[0],
			Timestamp: binary.LittleEndian.Uint64(payload[1:9]),
			Payload:   payload[9:],
		}
		if err := handler(event); err != nil {
			return err
		}
	}
}

func (w *WAL) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

func (w *WAL) Events() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.events
}

func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	w.writer.Reset(w.file)
	w.size = 0
	w.events = 0
	return w.writeHeader()
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		_ = w.writer.Flush()
	}
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *WAL) loadHeader() error {
	info, err := w.file.Stat()
	if err != nil {
		return err
	}
	w.size = info.Size()
	if w.size == 0 {
		return w.writeHeader()
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(w.file, headerSize)
	return w.readHeader(reader)
}

func (w *WAL) readHeader(r *bufio.Reader) error {
	buf := make([]byte, headerSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	if string(buf[:4]) != magicHeader {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func (w *WAL) writeHeader() error {
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, headerSize)
	copy(buf[:4], []byte(magicHeader))
	if _, err := w.file.Write(buf); err != nil {
		return err
	}
	w.size = int64(headerSize)
	return nil
}

func (w *WAL) scan() error {
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(w.file, 64*1024)
	if err := w.readHeader(reader); err != nil {
		return err
	}
	w.events = 0
	offset := int64(headerSize)
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(reader, lenBuf[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
		length := binary.LittleEndian.Uint32(lenBuf[:])
		if length == 0 {
			break
		}
		if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
		offset += int64(4 + length)
		w.events++
	}
	if offset > w.size {
		w.size = offset
	}
	return nil
}
