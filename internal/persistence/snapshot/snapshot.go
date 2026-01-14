package snapshot

import (
	"bufio"
	"io"
	"os"
)

type Writer interface {
	io.Writer
	Sync() error
	Close() error
}

type Reader interface {
	io.Reader
	Close() error
}

type FileWriter struct {
	f *os.File
	w *bufio.Writer
}

func OpenFileWriter(path string, bufSize int) (*FileWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	return &FileWriter{f: f, w: bufio.NewWriterSize(f, bufSize)}, nil
}

func (w *FileWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
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

func (r *FileReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *FileReader) Close() error {
	return r.f.Close()
}

func ReadAll(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}
