package wal

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
)

const recordHeaderSize = 4

type FileWriter struct {
	f *os.File
	w *bufio.Writer
}

func OpenFileWriter(path string, bufSize int) (*FileWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	return &FileWriter{f: f, w: bufio.NewWriterSize(f, bufSize)}, nil
}

func (w *FileWriter) Append(record []byte) error {
	var header [recordHeaderSize]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(record)))
	if _, err := w.w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.w.Write(record)
	return err
}

func (w *FileWriter) Flush() error {
	return w.w.Flush()
}

func (w *FileWriter) Sync() error {
	if err := w.w.Flush(); err != nil {
		return err
	}
	return w.f.Sync()
}

func (w *FileWriter) Close() error {
	if err := w.w.Flush(); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}

type FileReader struct {
	f *os.File
	r *bufio.Reader
}

func OpenFileReader(path string, bufSize int) (*FileReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	return &FileReader{f: f, r: bufio.NewReaderSize(f, bufSize)}, nil
}

func (r *FileReader) Next() ([]byte, error) {
	var header [recordHeaderSize]byte
	if _, err := io.ReadFull(r.r, header[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (r *FileReader) NextInto(buf []byte) ([]byte, error) {
	var header [recordHeaderSize]byte
	if _, err := io.ReadFull(r.r, header[:]); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint32(header[:]))
	if size <= 0 {
		return nil, io.ErrUnexpectedEOF
	}
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (r *FileReader) Close() error {
	return r.f.Close()
}
