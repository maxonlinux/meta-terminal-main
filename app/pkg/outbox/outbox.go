package outbox

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/logging"
)

const (
	logRecordBegin     byte = 1
	logRecordData      byte = 2
	logRecordCommit    byte = 3
	logRecordAbort     byte = 4
	defaultSegmentSize      = 16 << 20
)

type Writer interface {
	Record(events.Event) error
}

type EventSink interface {
	Apply(events []events.Event) error
}

type Outbox struct {
	log      *appendLog
	worker   *worker
	tailPath string
	seq      uint64
	sink     EventSink
}

func Open(dir string) (*Outbox, error) {
	return OpenWithOptions(dir, Options{})
}

const defaultQueueSize = 1 << 18

type Options struct {
	QueueSize   int
	EventSink   EventSink
	SegmentSize int64
}

type segmentInfo struct {
	path  string
	start int64
	size  int64
}

func OpenWithOptions(dir string, opts Options) (*Outbox, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	logPath := filepath.Join(dir, "outbox.aol")
	tailPath := filepath.Join(dir, "outbox.bin")

	log, err := openAppendLog(logPath, opts.SegmentSize)
	if err != nil {
		return nil, err
	}

	tail, err := loadTail(tailPath)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	newTail, err := replayLog(log, tail, opts.EventSink, tailPath)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	queue := newQueue(opts)
	worker := newWorker(log, tailPath, newTail, queue, opts.EventSink)

	return &Outbox{
		log:      log,
		worker:   worker,
		tailPath: tailPath,
		seq:      uint64(time.Now().UnixNano()),
		sink:     opts.EventSink,
	}, nil
}

func (o *Outbox) Begin() *Tx {
	if o == nil {
		return nil
	}
	txID := atomic.AddUint64(&o.seq, 1)
	offset, err := o.log.Append(logRecordBegin, txID, nil)
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
	data := append([]byte{byte(event.Type)}, event.Data...)
	offset, err := t.outbox.log.Append(logRecordData, t.id, data)
	if err != nil {
		return err
	}
	t.outbox.worker.Enqueue(record{recordType: logRecordData, txID: t.id, value: data, endOffset: offset})
	return nil
}

func (t *Tx) Commit() error {
	if t == nil || t.outbox == nil || t.closed {
		return errors.New("transaction closed")
	}
	offset, err := t.outbox.log.Append(logRecordCommit, t.id, nil)
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
	offset, err := t.outbox.log.Append(logRecordAbort, t.id, nil)
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
	segmentBase int64
	segmentSize int64
	basePath    string
	flushTicker *time.Ticker
	quit        chan struct{}
}

func openAppendLog(path string, segmentSize int64) (*appendLog, error) {
	if err := ensureSegments(path); err != nil {
		return nil, err
	}
	segments, err := listSegments(path)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		if err := createSegment(path, 0); err != nil {
			return nil, err
		}
		segments, err = listSegments(path)
		if err != nil {
			return nil, err
		}
	}
	last := segments[len(segments)-1]
	file, err := os.OpenFile(last.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if segmentSize <= 0 {
		segmentSize = defaultSegmentSize
	}
	log := &appendLog{
		file:        file,
		writer:      bufio.NewWriterSize(file, 1<<20),
		pending:     0,
		size:        info.Size(),
		segmentBase: last.start,
		segmentSize: segmentSize,
		basePath:    path,
		flushTicker: time.NewTicker(100 * time.Millisecond),
		quit:        make(chan struct{}),
	}
	go log.flushLoop()
	return log, nil
}

func ensureSegments(basePath string) error {
	if basePath == "" {
		return errors.New("log path is empty")
	}
	if _, err := os.Stat(basePath); err == nil {
		segments, segErr := listSegments(basePath)
		if segErr != nil {
			return segErr
		}
		if len(segments) == 0 {
			if err := os.Rename(basePath, segmentPath(basePath, 0)); err != nil {
				return err
			}
		}
	}
	return nil
}

func listSegments(basePath string) ([]segmentInfo, error) {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath) + "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	segments := make([]segmentInfo, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		suffix := strings.TrimPrefix(name, base)
		start, err := parseSegmentStart(suffix)
		if err != nil {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		segments = append(segments, segmentInfo{path: path, start: start, size: info.Size()})
	}
	slices.SortFunc(segments, func(a, b segmentInfo) int {
		if a.start < b.start {
			return -1
		}
		if a.start > b.start {
			return 1
		}
		return 0
	})
	return segments, nil
}

func parseSegmentStart(value string) (int64, error) {
	if value == "" {
		return 0, errors.New("empty segment start")
	}
	var start int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid segment start")
		}
		start = start*10 + int64(ch-'0')
	}
	return start, nil
}

func segmentPath(basePath string, start int64) string {
	return fmt.Sprintf("%s.%020d", basePath, start)
}

func createSegment(basePath string, start int64) error {
	path := segmentPath(basePath, start)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func (l *appendLog) Append(recordType byte, txID uint64, value []byte) (int64, error) {
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

	n = binary.PutUvarint(buf[:], uint64(len(value)))
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.size, err
	}
	l.size += int64(n)

	if _, err := l.writer.Write(value); err != nil {
		return l.size, err
	}
	l.size += int64(len(value))

	l.pending++
	endOffset := l.segmentBase + l.size
	if l.size >= l.segmentSize {
		if err := l.rotateLocked(); err != nil {
			return endOffset, err
		}
	}
	return endOffset, nil
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

func (l *appendLog) CompactApplied(offset int64) (int64, error) {
	if l == nil {
		return offset, errors.New("log is closed")
	}
	if offset <= 0 {
		return offset, nil
	}
	segments, err := listSegments(l.basePath)
	if err != nil {
		return offset, err
	}
	for _, seg := range segments {
		if seg.start+seg.size <= offset {
			_ = os.Remove(seg.path)
			continue
		}
	}
	if l.segmentBase+l.size <= offset {
		if err := l.rotateLocked(); err != nil {
			return offset, err
		}
	}
	return offset, nil
}

func (l *appendLog) rotateLocked() error {
	if l.writer != nil {
		_ = l.writer.Flush()
	}
	if l.file != nil {
		_ = l.file.Close()
	}
	l.segmentBase = l.segmentBase + l.size
	l.size = 0
	if err := createSegment(l.basePath, l.segmentBase); err != nil {
		return err
	}
	segments, err := listSegments(l.basePath)
	if err != nil {
		return err
	}
	last := segments[len(segments)-1]
	file, err := os.OpenFile(last.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	l.file = file
	l.writer = bufio.NewWriterSize(file, 1<<20)
	l.pending = 0
	return nil
}

type record struct {
	recordType byte
	txID       uint64
	value      []byte
	endOffset  int64
}

type worker struct {
	log        *appendLog
	tailPath   string
	queue      queue
	done       chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
	lastOffset int64
	sink       EventSink
}

func newWorker(log *appendLog, tailPath string, offset int64, queue queue, sink EventSink) *worker {
	return &worker{
		log:        log,
		tailPath:   tailPath,
		queue:      queue,
		done:       make(chan struct{}),
		lastOffset: offset,
		sink:       sink,
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
	lastCommitted := w.lastOffset
	pending := make(map[uint64][]record)
	pendingOffsets := make(map[uint64]int64)

	applyTx := func(txID uint64, offset int64) error {
		records := pending[txID]
		delete(pending, txID)
		delete(pendingOffsets, txID)
		lastCommitted = offset
		if w.sink != nil && len(records) > 0 {
			eventsBatch := make([]events.Event, 0, len(records))
			for i := range records {
				value := records[i].value
				if len(value) == 0 {
					continue
				}
				eventsBatch = append(eventsBatch, events.Event{Type: events.Type(value[0]), Data: value[1:]})
			}
			if len(eventsBatch) > 0 {
				if err := w.sink.Apply(eventsBatch); err != nil {
					logging.Log().Error().Err(err).Uint64("tx_id", txID).Int("events", len(eventsBatch)).Msg("outbox: sink apply failed")
					return err
				}
			}
		}
		if w.log != nil {
			cutoff := lastCommitted
			if len(pendingOffsets) > 0 {
				minPending := cutoff
				for _, off := range pendingOffsets {
					if off < minPending {
						minPending = off
					}
				}
				if minPending > 0 && minPending-1 < cutoff {
					cutoff = minPending - 1
				}
			}
			if cutoff > 0 {
				if _, err := w.log.CompactApplied(cutoff); err != nil {
					logging.Log().Error().Err(err).Int64("cutoff", cutoff).Msg("outbox: compact failed")
					return err
				}
			}
		}
		_ = storeTail(w.tailPath, lastCommitted)
		return nil
	}

	for {
		rec, ok := w.queue.Dequeue()
		if !ok {
			return
		}
		switch rec.recordType {
		case logRecordBegin:
			if _, ok := pending[rec.txID]; !ok {
				pending[rec.txID] = nil
			}
			pendingOffsets[rec.txID] = rec.endOffset
		case logRecordData:
			pending[rec.txID] = append(pending[rec.txID], rec)
			if off, ok := pendingOffsets[rec.txID]; !ok || rec.endOffset < off {
				pendingOffsets[rec.txID] = rec.endOffset
			}
		case logRecordCommit:
			if err := applyTx(rec.txID, rec.endOffset); err != nil {
				logging.Log().Error().Err(err).Uint64("tx_id", rec.txID).Int64("end_offset", rec.endOffset).Msg("outbox: apply tx failed, worker stopped")
				return
			}
		case logRecordAbort:
			delete(pending, rec.txID)
			delete(pendingOffsets, rec.txID)
			lastCommitted = rec.endOffset
			_ = storeTail(w.tailPath, lastCommitted)
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
	value      []byte
}

func replayLog(log *appendLog, offset int64, sink EventSink, tailPath string) (int64, error) {
	if log == nil {
		return offset, nil
	}
	segments, err := listSegments(log.basePath)
	if err != nil {
		return 0, err
	}
	if len(segments) == 0 {
		return offset, nil
	}

	pending := make(map[uint64][]logRecord)
	lastOffset := offset
	appliedOffset := offset

	applyTx := func(txID uint64) error {
		records := pending[txID]
		delete(pending, txID)
		appliedOffset = lastOffset
		if sink != nil && len(records) > 0 {
			eventsBatch := make([]events.Event, 0, len(records))
			for i := range records {
				value := records[i].value
				if len(value) == 0 {
					continue
				}
				eventsBatch = append(eventsBatch, events.Event{Type: events.Type(value[0]), Data: value[1:]})
			}
			if len(eventsBatch) > 0 {
				if err := sink.Apply(eventsBatch); err != nil {
					return err
				}
			}
		}
		return storeTail(tailPath, appliedOffset)
	}

	for _, seg := range segments {
		segStart := seg.start
		segEnd := seg.start + seg.size
		if offset >= segEnd {
			continue
		}
		file, err := os.Open(seg.path)
		if err != nil {
			return appliedOffset, err
		}
		seekOffset := int64(0)
		if offset > segStart {
			seekOffset = offset - segStart
		}
		if _, err := file.Seek(seekOffset, io.SeekStart); err != nil {
			_ = file.Close()
			return appliedOffset, err
		}
		reader := bufio.NewReaderSize(file, 1<<20)
		pos := segStart + seekOffset
		for {
			rec, size, err := readLogRecord(reader)
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = file.Close()
				return appliedOffset, err
			}
			pos += int64(size)
			lastOffset = pos
			switch rec.recordType {
			case logRecordBegin:
				if _, ok := pending[rec.txID]; !ok {
					pending[rec.txID] = nil
				}
			case logRecordData:
				pending[rec.txID] = append(pending[rec.txID], rec)
			case logRecordCommit:
				if err := applyTx(rec.txID); err != nil {
					_ = file.Close()
					return lastOffset, err
				}
			case logRecordAbort:
				delete(pending, rec.txID)
				appliedOffset = lastOffset
				if err := storeTail(tailPath, appliedOffset); err != nil {
					_ = file.Close()
					return lastOffset, err
				}
			}
		}
		_ = file.Close()
	}

	if len(pending) == 0 {
		if newOffset, err := log.CompactApplied(appliedOffset); err == nil {
			appliedOffset = newOffset
		}
	}
	if err := storeTail(tailPath, appliedOffset); err != nil {
		return appliedOffset, err
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

	valLen, valBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + txBytes + valBytes, err
	}

	value := make([]byte, valLen)
	if _, err := io.ReadFull(r, value); err != nil {
		return logRecord{}, 1 + txBytes + valBytes, err
	}

	size := 1 + txBytes + valBytes + int(valLen)
	return logRecord{recordType: recordType, txID: txID, value: value}, size, nil
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
