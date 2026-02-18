package outbox

import (
	"sync"
	"sync/atomic"
)

type queue interface {
	Enqueue(record)
	Dequeue() (record, bool)
	Close()
}

type ringQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []record
	head   int
	tail   int
	count  int
	closed bool
	grows  uint64
}

func newRingQueue(size int) *ringQueue {
	if size <= 0 {
		size = defaultQueueSize
	}
	q := &ringQueue{buf: make([]record, size)}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *ringQueue) Enqueue(rec record) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	if q.count == len(q.buf) {
		q.grow()
	}
	q.buf[q.tail] = rec
	q.tail = (q.tail + 1) % len(q.buf)
	q.count++
	q.cond.Signal()
	q.mu.Unlock()
}

func (q *ringQueue) Dequeue() (record, bool) {
	q.mu.Lock()
	for q.count == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.count == 0 && q.closed {
		q.mu.Unlock()
		return record{}, false
	}
	rec := q.buf[q.head]
	q.head = (q.head + 1) % len(q.buf)
	q.count--
	q.mu.Unlock()
	return rec, true
}

func (q *ringQueue) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *ringQueue) grow() {
	newBuf := make([]record, len(q.buf)*2)
	for i := 0; i < q.count; i++ {
		newBuf[i] = q.buf[(q.head+i)%len(q.buf)]
	}
	q.buf = newBuf
	q.head = 0
	q.tail = q.count
	atomic.AddUint64(&q.grows, 1)
}

func (q *ringQueue) GrowCount() uint64 {
	return atomic.LoadUint64(&q.grows)
}

func newQueue(opts Options) queue {
	return newRingQueue(opts.QueueSize)
}
