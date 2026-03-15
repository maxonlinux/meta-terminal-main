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
	logRecordFlush       byte = 0
	logRecordBegin       byte = 1
	logRecordData        byte = 2
	logRecordCommit      byte = 3
	logRecordAbort       byte = 4
	defaultSegmentSize        = 16 << 20
	retryAttempts             = 20
	applyBatchSize            = 2000
	applyBatchFlushEvery      = 200 * time.Millisecond
)

var retryBackoffStart = 100 * time.Millisecond
var retryBackoffMax = 5 * time.Second
var retrySleep = time.Sleep
var retryTimer = time.NewTimer

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
	start uint64
	end   uint64
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
		logging.Log().Error().Err(err).Msg("outbox: replay failed, continuing with last good tail")
		newTail = tail
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
	o.worker.Enqueue(record{recordType: logRecordBegin, txID: txID, endSeq: offset})
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
	t.outbox.worker.Enqueue(record{recordType: logRecordData, txID: t.id, value: data, endSeq: offset})
	return nil
}

func (t *Tx) Commit() error {
	if t == nil || t.outbox == nil || t.closed {
		return errors.New("transaction closed")
	}
	offset, err := t.outbox.log.Append(logRecordCommit, t.id, nil)
	if err == nil {
		t.outbox.worker.Enqueue(record{recordType: logRecordCommit, txID: t.id, endSeq: offset})
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
		t.outbox.worker.Enqueue(record{recordType: logRecordAbort, txID: t.id, endSeq: offset})
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
	segmentBase uint64
	segmentSize int64
	basePath    string
	flushTicker *time.Ticker
	quit        chan struct{}
	lastSeq     uint64
	nextSeq     uint64
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
		if err := createSegment(path, 1, 0); err != nil {
			return nil, err
		}
		segments, err = listSegments(path)
		if err != nil {
			return nil, err
		}
	}
	last := segments[len(segments)-1]
	current := last
	if last.end > 0 {
		start := last.end + 1
		if err := createSegment(path, start, 0); err != nil {
			return nil, err
		}
		segments, err = listSegments(path)
		if err != nil {
			return nil, err
		}
		current = segments[len(segments)-1]
	}
	file, err := os.OpenFile(current.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	lastSeq := uint64(0)
	if current.end > 0 {
		lastSeq = current.end
	} else if info.Size() > 0 {
		seq, err := scanLastSeq(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		lastSeq = seq
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		return nil, err
	}
	if segmentSize <= 0 {
		segmentSize = defaultSegmentSize
	}
	nextSeq := lastSeq + 1
	if nextSeq < current.start {
		nextSeq = current.start
	}
	log := &appendLog{
		file:        file,
		writer:      bufio.NewWriterSize(file, 1<<20),
		pending:     0,
		size:        info.Size(),
		segmentBase: current.start,
		segmentSize: segmentSize,
		basePath:    path,
		flushTicker: time.NewTicker(100 * time.Millisecond),
		quit:        make(chan struct{}),
		lastSeq:     lastSeq,
		nextSeq:     nextSeq,
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
			if err := os.Rename(basePath, segmentPath(basePath, 1, 0)); err != nil {
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
		start, end, err := parseSegmentRange(suffix)
		if err != nil {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		segments = append(segments, segmentInfo{path: path, start: start, end: end, size: info.Size()})
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

func parseSegmentRange(value string) (uint64, uint64, error) {
	if value == "" {
		return 0, 0, errors.New("empty segment range")
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return 0, 0, errors.New("invalid segment range")
	}
	start, err := parseSegmentNumber(parts[0])
	if err != nil {
		return 0, 0, err
	}
	if len(parts) == 1 {
		return start, 0, nil
	}
	if parts[1] == "" {
		return 0, 0, errors.New("empty segment end")
	}
	end, err := parseSegmentNumber(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		return 0, 0, errors.New("invalid segment range order")
	}
	return start, end, nil
}

func parseSegmentNumber(value string) (uint64, error) {
	if value == "" {
		return 0, errors.New("empty segment number")
	}
	var start uint64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid segment number")
		}
		start = start*10 + uint64(ch-'0')
	}
	return start, nil
}

func segmentPath(basePath string, start uint64, end uint64) string {
	if end == 0 {
		return fmt.Sprintf("%s.%020d", basePath, start)
	}
	return fmt.Sprintf("%s.%020d-%020d", basePath, start, end)
}

func createSegment(basePath string, start uint64, end uint64) error {
	path := segmentPath(basePath, start, end)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func scanLastSeq(file *os.File) (uint64, error) {
	if file == nil {
		return 0, errors.New("nil log file")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	reader := bufio.NewReaderSize(file, 1<<20)
	var last uint64
	for {
		rec, _, err := readLogRecord(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return last, err
		}
		last = rec.seq
	}
	return last, nil
}

func (l *appendLog) Append(recordType byte, txID uint64, value []byte) (uint64, error) {
	if l == nil {
		return 0, errors.New("log is closed")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var buf [10]byte
	if err := l.writer.WriteByte(recordType); err != nil {
		return l.lastSeq, err
	}
	l.size++
	seq := l.nextSeq
	l.nextSeq++
	l.lastSeq = seq

	n := binary.PutUvarint(buf[:], seq)
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.lastSeq, err
	}
	l.size += int64(n)

	n = binary.PutUvarint(buf[:], txID)
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.lastSeq, err
	}
	l.size += int64(n)

	n = binary.PutUvarint(buf[:], uint64(len(value)))
	if _, err := l.writer.Write(buf[:n]); err != nil {
		return l.lastSeq, err
	}
	l.size += int64(n)

	if _, err := l.writer.Write(value); err != nil {
		return l.lastSeq, err
	}
	l.size += int64(len(value))

	l.pending++
	endOffset := l.lastSeq
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

func (l *appendLog) CompactApplied(offset uint64) (uint64, error) {
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
	deleted := 0
	for _, seg := range segments {
		if seg.end > 0 && seg.end <= offset {
			if err := os.Remove(seg.path); err == nil {
				deleted++
			}
			continue
		}
	}
	if deleted > 0 {
		logging.Log().Info().Int("deleted", deleted).Uint64("offset", offset).Msg("outbox: deleted old segments")
	}
	if l.size > 0 && l.lastSeq > 0 && l.lastSeq <= offset {
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
	if l.size > 0 && l.lastSeq >= l.segmentBase {
		currentPath := segmentPath(l.basePath, l.segmentBase, 0)
		finalPath := segmentPath(l.basePath, l.segmentBase, l.lastSeq)
		if err := os.Rename(currentPath, finalPath); err != nil {
			return err
		}
	}
	start := l.lastSeq + 1
	if err := createSegment(l.basePath, start, 0); err != nil {
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
	l.segmentBase = start
	l.size = 0
	return nil
}

type record struct {
	recordType byte
	txID       uint64
	value      []byte
	endSeq     uint64
}

type worker struct {
	log        *appendLog
	tailPath   string
	queue      queue
	done       chan struct{}
	stop       chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
	lastOffset uint64
	sink       EventSink
}

func newWorker(log *appendLog, tailPath string, offset uint64, queue queue, sink EventSink) *worker {
	return &worker{
		log:        log,
		tailPath:   tailPath,
		queue:      queue,
		done:       make(chan struct{}),
		stop:       make(chan struct{}),
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
		close(w.stop)
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
	pendingOffsets := make(map[uint64]uint64)
	type committedTx struct {
		txID   uint64
		endSeq uint64
	}
	batchEvents := make([]events.Event, 0, applyBatchSize)
	batchTxs := make([]committedTx, 0, applyBatchSize)
	flushTicker := time.NewTicker(applyBatchFlushEvery)
	defer flushTicker.Stop()
	go func() {
		for {
			select {
			case <-flushTicker.C:
				w.queue.Enqueue(record{recordType: logRecordFlush})
			case <-w.stop:
				return
			}
		}
	}()

	finalizeTx := func(txID uint64, offset uint64) error {
		delete(pending, txID)
		delete(pendingOffsets, txID)
		lastCommitted = offset
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
				if newOffset, err := w.log.CompactApplied(cutoff); err != nil {
					logging.Log().Error().Err(err).Uint64("cutoff", cutoff).Msg("outbox: compact failed")
					return err
				} else if newOffset > 0 {
					logging.Log().Info().Uint64("cutoff", cutoff).Uint64("compacted_to", newOffset).Msg("outbox: compacted")
				}
			}
		}
		_ = storeTail(w.tailPath, lastCommitted)
		return nil
	}

	buildEvents := func(txID uint64) []events.Event {
		records := pending[txID]
		if len(records) == 0 {
			return nil
		}
		eventsBatch := make([]events.Event, 0, len(records))
		for i := range records {
			value := records[i].value
			if len(value) == 0 {
				continue
			}
			eventsBatch = append(eventsBatch, events.Event{Type: events.Type(value[0]), Data: value[1:]})
		}
		return eventsBatch
	}

	finalizeBatch := func(txs []committedTx) error {
		if len(txs) == 0 {
			return nil
		}
		for i := range txs {
			delete(pending, txs[i].txID)
			delete(pendingOffsets, txs[i].txID)
		}
		lastCommitted = txs[len(txs)-1].endSeq
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
				if newOffset, err := w.log.CompactApplied(cutoff); err != nil {
					logging.Log().Error().Err(err).Uint64("cutoff", cutoff).Msg("outbox: compact failed")
					return err
				} else if newOffset > 0 {
					logging.Log().Info().Uint64("cutoff", cutoff).Uint64("compacted_to", newOffset).Msg("outbox: compacted")
				}
			}
		}
		_ = storeTail(w.tailPath, lastCommitted)
		return nil
	}

	flushBatch := func() error {
		if len(batchTxs) == 0 {
			return nil
		}
		lastTx := batchTxs[len(batchTxs)-1]
		apply := func() error {
			if w.sink == nil || len(batchEvents) == 0 {
				return nil
			}
			if err := w.sink.Apply(batchEvents); err != nil {
				logging.Log().Error().Err(err).Int("events", len(batchEvents)).Int("tx_count", len(batchTxs)).Msg("outbox: sink apply failed")
				return err
			}
			return nil
		}
		finalize := func() error {
			err := finalizeBatch(batchTxs)
			batchTxs = batchTxs[:0]
			batchEvents = batchEvents[:0]
			return err
		}
		return applyWithRetry(lastTx.txID, lastTx.endSeq, w.stop, apply, finalize, "live")
	}

	for {
		rec, ok := w.queue.Dequeue()
		if !ok {
			_ = flushBatch()
			return
		}
		switch rec.recordType {
		case logRecordFlush:
			_ = flushBatch()
		case logRecordBegin:
			if _, ok := pending[rec.txID]; !ok {
				pending[rec.txID] = nil
			}
			pendingOffsets[rec.txID] = rec.endSeq
		case logRecordData:
			pending[rec.txID] = append(pending[rec.txID], rec)
			if off, ok := pendingOffsets[rec.txID]; !ok || rec.endSeq < off {
				pendingOffsets[rec.txID] = rec.endSeq
			}
		case logRecordCommit:
			eventsBatch := buildEvents(rec.txID)
			if len(eventsBatch) == 0 {
				_ = finalizeTx(rec.txID, rec.endSeq)
				continue
			}
			batchEvents = append(batchEvents, eventsBatch...)
			batchTxs = append(batchTxs, committedTx{txID: rec.txID, endSeq: rec.endSeq})
			if len(batchEvents) >= applyBatchSize {
				_ = flushBatch()
			}
		case logRecordAbort:
			delete(pending, rec.txID)
			delete(pendingOffsets, rec.txID)
			lastCommitted = rec.endSeq
			_ = storeTail(w.tailPath, lastCommitted)
		}
	}
}

func loadTail(path string) (uint64, error) {
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
	return binary.BigEndian.Uint64(data), nil
}

func storeTail(path string, offset uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(offset))
	return os.WriteFile(path, buf[:], 0o600)
}

type logRecord struct {
	recordType byte
	seq        uint64
	txID       uint64
	value      []byte
}

func replayLog(log *appendLog, offset uint64, sink EventSink, tailPath string) (uint64, error) {
	if log == nil {
		return offset, nil
	}
	segments, err := listSegments(log.basePath)
	if err != nil {
		logging.Log().Error().Err(err).Msg("outbox: replay list segments failed")
		return offset, nil
	}
	if len(segments) == 0 {
		return offset, nil
	}

	pending := make(map[uint64][]logRecord)
	lastOffset := offset
	appliedOffset := offset

	applyTx := func(txID uint64) error {
		records := pending[txID]
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
		return nil
	}

	finalizeTx := func(txID uint64) error {
		delete(pending, txID)
		appliedOffset = lastOffset
		return storeTail(tailPath, appliedOffset)
	}

	applyTxWithRetry := func(txID uint64, endSeq uint64) error {
		return applyWithRetry(txID, endSeq, nil, func() error {
			return applyTx(txID)
		}, func() error {
			return finalizeTx(txID)
		}, "replay")
	}

	for _, seg := range segments {
		if seg.end > 0 && seg.end <= offset {
			continue
		}
		file, err := os.Open(seg.path)
		if err != nil {
			logging.Log().Error().Err(err).Str("path", seg.path).Msg("outbox: replay open segment failed, skipping")
			continue
		}
		reader := bufio.NewReaderSize(file, 1<<20)
		for {
			rec, size, err := readLogRecord(reader)
			if err == io.EOF {
				break
			}
			if err != nil && err != io.EOF {
				_ = file.Close()
				logging.Log().Error().Err(err).Msg("outbox: replay read record failed, stopping segment")
				break
			}
			_ = size
			if rec.seq <= offset {
				continue
			}
			lastOffset = rec.seq
			switch rec.recordType {
			case logRecordBegin:
				if _, ok := pending[rec.txID]; !ok {
					pending[rec.txID] = nil
				}
			case logRecordData:
				pending[rec.txID] = append(pending[rec.txID], rec)
			case logRecordCommit:
				_ = applyTxWithRetry(rec.txID, rec.seq)
			case logRecordAbort:
				delete(pending, rec.txID)
				appliedOffset = lastOffset
				if err := storeTail(tailPath, appliedOffset); err != nil {
					logging.Log().Error().Err(err).Msg("outbox: replay store tail failed on abort")
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

	seq, seqBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + seqBytes, err
	}

	txID, txBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + seqBytes + txBytes, err
	}

	valLen, valBytes, err := readUvarint(r)
	if err != nil {
		return logRecord{}, 1 + seqBytes + txBytes + valBytes, err
	}

	value := make([]byte, valLen)
	if _, err := io.ReadFull(r, value); err != nil {
		return logRecord{}, 1 + seqBytes + txBytes + valBytes, err
	}

	size := 1 + seqBytes + txBytes + valBytes + int(valLen)
	return logRecord{recordType: recordType, seq: seq, txID: txID, value: value}, size, nil
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

func applyWithRetry(txID uint64, endSeq uint64, stop <-chan struct{}, apply func() error, finalize func() error, scope string) error {
	backoff := retryBackoffStart
	maxBackoff := retryBackoffMax
	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		if err := apply(); err == nil {
			return finalize()
		} else {
			lastErr = err
			logging.Log().Error().Err(err).Uint64("tx_id", txID).Uint64("end_seq", endSeq).Int("attempt", attempt).Str("scope", scope).Msg("outbox: apply failed, retrying")
		}
		if attempt == retryAttempts {
			break
		}
		if stop != nil {
			timer := retryTimer(backoff)
			select {
			case <-timer.C:
			case <-stop:
				timer.Stop()
				return errors.New("worker stopped")
			}
		} else {
			retrySleep(backoff)
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
	logging.Log().Error().Err(lastErr).Uint64("tx_id", txID).Uint64("end_seq", endSeq).Str("scope", scope).Msg("outbox: apply failed, dropping transaction")
	return finalize()
}
