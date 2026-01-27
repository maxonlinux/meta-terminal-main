package outbox

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
)

const (
	logRecordBegin  byte = 1
	logRecordData   byte = 2
	logRecordCommit byte = 3
	logRecordAbort  byte = 4
)

type Writer interface {
	Record(events.Event) error
}

type Outbox struct {
	db       *badger.DB
	log      *appendLog
	worker   *worker
	tailPath string
	seq      uint64
	eventSeq uint64
}

func Open(dir string, db *badger.DB) (*Outbox, error) {
	return OpenWithOptions(dir, db, Options{})
}

const defaultQueueSize = 1 << 18

type Options struct {
	QueueSize int
}

func OpenWithOptions(dir string, db *badger.DB, opts Options) (*Outbox, error) {
	if db == nil {
		return nil, errors.New("outbox requires badger db")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	logPath := filepath.Join(dir, "outbox.aol")
	tailPath := filepath.Join(dir, "outbox.bin")

	log, err := openAppendLog(logPath)
	if err != nil {
		return nil, err
	}

	tail, err := loadTail(tailPath)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	newTail, err := replayLog(db, log, tail)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	if err := storeTail(tailPath, newTail); err != nil {
		_ = log.Close()
		return nil, err
	}

	queue := newQueue(opts)
	worker := newWorker(db, log, tailPath, newTail, queue)

	return &Outbox{
		db:       db,
		log:      log,
		worker:   worker,
		tailPath: tailPath,
		seq:      uint64(time.Now().UnixNano()),
	}, nil
}

func (o *Outbox) Begin() *Tx {
	if o == nil {
		return nil
	}
	txID := atomic.AddUint64(&o.seq, 1)
	offset, err := o.log.Append(logRecordBegin, txID, nil, nil)
	if err != nil {
		return nil
	}
	o.worker.Enqueue(record{recordType: logRecordBegin, txID: txID, endOffset: offset})
	return &Tx{outbox: o, id: txID}
}

func (o *Outbox) Start() {
	if o == nil || o.worker == nil {
		return
	}
	o.worker.Start()
}

func (o *Outbox) Stop() {
	if o == nil || o.worker == nil {
		return
	}
	o.worker.Stop()
}

func (o *Outbox) Close() error {
	if o == nil {
		return nil
	}
	o.Stop()
	if o.log != nil {
		return o.log.Close()
	}
	return nil
}

func (o *Outbox) QueueGrowCount() uint64 {
	if o == nil || o.worker == nil {
		return 0
	}
	if rq, ok := o.worker.queue.(*ringQueue); ok {
		return rq.GrowCount()
	}
	return 0
}

type Tx struct {
	outbox *Outbox
	id     uint64
	closed bool
}

func (t *Tx) Record(event events.Event) error {
	if t == nil || t.outbox == nil || t.closed {
		return errors.New("transaction closed")
	}
	seq := atomic.AddUint64(&t.outbox.eventSeq, 1)
	key := persistence.EventKey(seq)
	data := append([]byte{byte(event.Type)}, event.Data...)
	offset, err := t.outbox.log.Append(logRecordData, t.id, key, data)
	if err != nil {
		return err
	}
	t.outbox.worker.Enqueue(record{recordType: logRecordData, txID: t.id, key: key, value: data, endOffset: offset})
	return nil
}

func (t *Tx) Commit() error {
	if t == nil || t.outbox == nil || t.closed {
		return errors.New("transaction closed")
	}
	offset, err := t.outbox.log.Append(logRecordCommit, t.id, nil, nil)
	if err == nil {
		t.outbox.worker.Enqueue(record{recordType: logRecordCommit, txID: t.id, endOffset: offset})
	}
	t.closed = true
	return err
}

func (t *Tx) Abort() error {
	if t == nil || t.outbox == nil || t.closed {
		return errors.New("transaction closed")
	}
	offset, err := t.outbox.log.Append(logRecordAbort, t.id, nil, nil)
	if err == nil {
		t.outbox.worker.Enqueue(record{recordType: logRecordAbort, txID: t.id, endOffset: offset})
	}
	t.closed = true
	return err
}

type appendLog struct {
	mu          sync.Mutex
	file        *os.File
	writer      *bufio.Writer
	pending     int
	size        int64
	flushEvery  int
	flushTicker *time.Ticker
	quit        chan struct{}
}

func openAppendLog(path string) (*appendLog, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	log := &appendLog{
		file:        file,
		writer:      bufio.NewWriterSize(file, 1<<20),
		pending:     0,
		size:        info.Size(),
		flushEvery:  1000,
		flushTicker: time.NewTicker(100 * time.Millisecond),
		quit:        make(chan struct{}),
	}
	go log.flushLoop()
	return log, nil
}

func (l *appendLog) Append(recordType byte, txID uint64, key, value []byte) (int64, error) {
	if l == nil {
		return 0, errors.New("log is closed")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var buf [10]byte
	if err := l.writer.WriteByte(recordType); err != nil {
		return l.size, err
	}
	l.size++

	n := binary.PutUvarint(buf[:], txID)
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.size, err
	}
	l.size += int64(n)

	n = binary.PutUvarint(buf[:], uint64(len(key)))
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.size, err
	}
	l.size += int64(n)

	n = binary.PutUvarint(buf[:], uint64(len(value)))
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.size, err
	}
	l.size += int64(n)

	if _, err := l.writer.Write(key); err != nil {
		return l.size, err
	}
	l.size += int64(len(key))

	if _, err := l.writer.Write(value); err != nil {
		return l.size, err
	}
	l.size += int64(len(value))

	l.pending++
	return l.size, nil
}

func (l *appendLog) flushLoop() {
	for {
		select {
		case <-l.flushTicker.C:
			_ = l.Flush()
		case <-l.quit:
			return
		}
	}
}

func (l *appendLog) Flush() error {
	if l == nil {
		return errors.New("log is closed")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flushLocked()
}

func (l *appendLog) flushLocked() error {
	if l.pending == 0 {
		return nil
	}
	l.pending = 0
	return l.writer.Flush()
}

func (l *appendLog) Close() error {
	if l == nil {
		return nil
	}
	close(l.quit)
	l.flushTicker.Stop()
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.flushLocked()
	return l.file.Close()
}

func (l *appendLog) ResetIfFullyApplied(offset int64) (int64, error) {
	if l == nil {
		return offset, errors.New("log is closed")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if offset == 0 || offset != l.size {
		return offset, nil
	}
	if err := l.writer.Flush(); err != nil {
		return offset, err
	}
	if err := l.file.Truncate(0); err != nil {
		return offset, err
	}
	if _, err := l.file.Seek(0, io.SeekEnd); err != nil {
		return offset, err
	}
	l.size = 0
	l.pending = 0
	return 0, nil
}

type record struct {
	recordType byte
	txID       uint64
	key        []byte
	value      []byte
	endOffset  int64
}

type worker struct {
	db         *badger.DB
	log        *appendLog
	tailPath   string
	queue      queue
	done       chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
	lastOffset int64
}

func newWorker(db *badger.DB, log *appendLog, tailPath string, offset int64, queue queue) *worker {
	return &worker{
		db:         db,
		log:        log,
		tailPath:   tailPath,
		queue:      queue,
		done:       make(chan struct{}),
		lastOffset: offset,
	}
}

func (w *worker) Start() {
	w.startOnce.Do(func() {
		go w.run()
	})
}

func (w *worker) Enqueue(rec record) {
	if w == nil || w.queue == nil {
		return
	}
	w.queue.Enqueue(rec)
}

func (w *worker) Stop() {
	w.stopOnce.Do(func() {
		if w.queue != nil {
			w.queue.Close()
		}
		<-w.done
	})
}

func (w *worker) run() {
	defer close(w.done)
	lastOffset := w.lastOffset
	pending := make(map[uint64][]record)

	applyTx := func(txID uint64, offset int64) {
		records := pending[txID]
		batch := w.db.NewWriteBatch()
		for i := range records {
			rec := records[i]
			if rec.key == nil {
				continue
			}
			_ = batch.Set(rec.key, rec.value)
		}
		_ = batch.Flush()
		batch.Cancel()
		delete(pending, txID)
		lastOffset = offset
		_ = storeTail(w.tailPath, lastOffset)
		if w.log != nil && len(pending) == 0 {
			if newOffset, err := w.log.ResetIfFullyApplied(lastOffset); err == nil {
				lastOffset = newOffset
			}
		}
	}

	for {
		rec, ok := w.queue.Dequeue()
		if !ok {
			return
		}
		if rec.endOffset > 0 {
			lastOffset = rec.endOffset
		}
		switch rec.recordType {
		case logRecordBegin:
			if _, ok := pending[rec.txID]; !ok {
				pending[rec.txID] = nil
			}
		case logRecordData:
			pending[rec.txID] = append(pending[rec.txID], rec)
		case logRecordCommit:
			applyTx(rec.txID, lastOffset)
		case logRecordAbort:
			delete(pending, rec.txID)
			_ = storeTail(w.tailPath, lastOffset)
		}
	}
}

func loadTail(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if len(data) < 8 {
		return 0, nil
	}
	return int64(binary.BigEndian.Uint64(data)), nil
}

func storeTail(path string, offset int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(offset))
	return os.WriteFile(path, buf[:], 0o600)
}

type logRecord struct {
	recordType byte
	txID       uint64
	key        []byte
	value      []byte
}

func replayLog(db *badger.DB, log *appendLog, offset int64) (int64, error) {
	if log == nil {
		return offset, nil
	}
	file, err := os.Open(log.file.Name())
	if err != nil {
		if os.IsNotExist(err) {
			return offset, nil
		}
		return 0, err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if offset >= info.Size() {
		return info.Size(), nil
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	reader := bufio.NewReaderSize(file, 1<<20)
	pending := make(map[uint64][]logRecord)
	lastOffset := offset
	appliedOffset := offset

	for {
		rec, size, err := readLogRecord(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		lastOffset += int64(size)
		switch rec.recordType {
		case logRecordBegin:
			if _, ok := pending[rec.txID]; !ok {
				pending[rec.txID] = nil
			}
		case logRecordData:
			pending[rec.txID] = append(pending[rec.txID], rec)
		case logRecordCommit:
			records := pending[rec.txID]
			batch := db.NewWriteBatch()
			for i := range records {
				r := records[i]
				if r.key == nil {
					continue
				}
				if err := batch.Set(r.key, r.value); err != nil {
					batch.Cancel()
					return lastOffset, err
				}
			}
			if err := batch.Flush(); err != nil {
				batch.Cancel()
				return lastOffset, err
			}
			batch.Cancel()
			delete(pending, rec.txID)
			appliedOffset = lastOffset
		case logRecordAbort:
			delete(pending, rec.txID)
			appliedOffset = lastOffset
		}
	}

	if log != nil {
		if len(pending) == 0 {
			if offset, err := log.ResetIfFullyApplied(appliedOffset); err == nil {
				appliedOffset = offset
			}
		}
	}

	return appliedOffset, nil
}

func readLogRecord(r *bufio.Reader) (logRecord, int, error) {
	recordType, err := r.ReadByte()
	if err != nil {
		return logRecord{}, 0, err
	}

	txID, txBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + txBytes, err
	}

	keyLen, keyBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + txBytes + keyBytes, err
	}
	valLen, valBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + txBytes + keyBytes + valBytes, err
	}

	key := make([]byte, keyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return logRecord{}, 1 + txBytes + keyBytes + valBytes, err
	}

	value := make([]byte, valLen)
	if _, err := io.ReadFull(r, value); err != nil {
		return logRecord{}, 1 + txBytes + keyBytes + valBytes + int(keyLen), err
	}

	size := 1 + txBytes + keyBytes + valBytes + int(keyLen) + int(valLen)
	return logRecord{recordType: recordType, txID: txID, key: key, value: value}, size, nil
}

func readUvarint(r *bufio.Reader) (uint64, int, error) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, i, err
		}
		if b < 0x80 {
			if i > 9 || (i == 9 && b > 1) {
				return 0, i + 1, errors.New("uvarint overflow")
			}
			return x | uint64(b)<<s, i + 1, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}
