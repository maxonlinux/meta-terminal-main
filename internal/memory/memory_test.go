package memory

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOrderPool(t *testing.T) {
	pool := NewOrderPool()

	order1 := pool.Get()
	if order1 == nil {
		t.Fatal("expected order from pool")
	}

	order1.ID = 1
	order1.UserID = 2
	order1.Quantity = 100

	pool.Put(order1)

	order2 := pool.Get()
	if order2.ID != 0 {
		t.Errorf("expected reset ID, got %d", order2.ID)
	}
	if order2.UserID != 0 {
		t.Errorf("expected reset UserID, got %d", order2.UserID)
	}
	if order2.Quantity != 0 {
		t.Errorf("expected reset Quantity, got %d", order2.Quantity)
	}
}

func TestTradePool(t *testing.T) {
	pool := NewTradePool()

	trade1 := pool.Get()
	if trade1 == nil {
		t.Fatal("expected trade from pool")
	}

	trade1.BuyerID = 1
	trade1.SellerID = 2
	trade1.Quantity = 100

	pool.Put(trade1)

	trade2 := pool.Get()
	if trade2.BuyerID != 0 {
		t.Errorf("expected reset BuyerID, got %d", trade2.BuyerID)
	}
}

func TestTradeBuffer(t *testing.T) {
	pool := NewTradePool()
	buffer := NewTradeBuffer(pool, 64)

	trade1 := pool.Get()
	trade1.Quantity = 10

	trade2 := pool.Get()
	trade2.Quantity = 20

	buffer.Add(trade1)
	buffer.Add(trade2)

	trades := buffer.Slice()
	if len(trades) != 2 {
		t.Errorf("expected 2 trades, got %d", len(trades))
	}

	total := types.Quantity(0)
	for _, trade := range trades {
		total += trade.Quantity
	}
	if total != 30 {
		t.Errorf("expected total 30, got %d", total)
	}

	buffer.Reset()

	if len(buffer.Slice()) != 0 {
		t.Error("expected empty buffer after reset")
	}
}

func TestUserQueue(t *testing.T) {
	q := NewUserQueue()

	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d", q.Len())
	}

	orderID := types.OrderID(1)
	q.Push(orderID)

	if q.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", q.Len())
	}

	popped := q.Pop()
	if popped != 1 {
		t.Errorf("expected order ID 1, got %d", popped)
	}

	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d", q.Len())
	}
}

func TestUserQueueClose(t *testing.T) {
	q := NewUserQueue()

	if q.IsClosed() {
		t.Error("expected queue not closed")
	}

	q.Close()

	if !q.IsClosed() {
		t.Error("expected queue closed")
	}
}

func TestDispatcher(t *testing.T) {
	s := state.New()
	d := NewDispatcher(s)

	userID := types.UserID(1)

	q1 := d.GetQueue(userID)
	q2 := d.GetQueue(userID)

	if q1 != q2 {
		t.Error("expected same queue for same user")
	}

	d.RemoveQueue(userID)

	q3 := d.GetQueue(userID)
	if q3 == q1 {
		t.Error("expected new queue after removal")
	}
}
