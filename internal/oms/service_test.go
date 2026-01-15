package oms

import (
	"sync"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestService_Create(t *testing.T) {
	s := NewService()

	order := s.Create(
		types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
		false, false, 0,
	)

	if order.ID == 0 {
		t.Error("order ID should not be zero")
	}
	if order.UserID != 1 {
		t.Errorf("expected userID 1, got %d", order.UserID)
	}
	if order.Symbol != "BTCUSDT" {
		t.Errorf("expected symbol BTCUSDT, got %s", order.Symbol)
	}
	if order.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("expected status NEW, got %d", order.Status)
	}
	if s.Count() != 1 {
		t.Errorf("expected count 1, got %d", s.Count())
	}
}

func TestService_CreateConditional(t *testing.T) {
	s := NewService()

	order := s.Create(
		types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)),
		false, false, constants.STOP_ORDER_TYPE_STOP,
	)

	if !order.IsConditional {
		t.Error("order should be conditional")
	}
	if math.Cmp(order.TriggerPrice, types.Price(fixed.NewI(49000, 0))) != 0 {
		t.Errorf("expected triggerPrice 49000")
	}
}

func TestService_Get(t *testing.T) {
	s := NewService()

	created := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
		false, false, 0)

	order, ok := s.Get(created.ID)
	if !ok {
		t.Error("order not found")
	}
	if order.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, order.ID)
	}

	_, ok = s.Get(types.OrderID(99999))
	if ok {
		t.Error("should not find non-existent order")
	}
}

func TestService_Amend(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
		false, false, 0)

	err := s.Amend(order.ID, types.Quantity(fixed.NewI(5, 0)))
	if err != nil {
		t.Errorf("amend failed: %v", err)
	}
	if math.Cmp(order.Quantity, types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected quantity 5")
	}

	err = s.Amend(order.ID, types.Quantity(fixed.NewI(15, 0)))
	if err == nil {
		t.Error("amend to larger quantity should fail")
	}
}

func TestService_Cancel(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
		false, false, 0)

	err := s.Cancel(order.ID)
	if err != nil {
		t.Errorf("cancel failed: %v", err)
	}
	if order.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected status CANCELED, got %d", order.Status)
	}

	err = s.Cancel(order.ID)
	if err == nil {
		t.Error("cancel already canceled order should fail")
	}
}

func TestService_Fill(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
		false, false, 0)

	err := s.Fill(order.ID, types.Quantity(fixed.NewI(5, 0)))
	if err != nil {
		t.Errorf("fill failed: %v", err)
	}
	if math.Cmp(order.Filled, types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected filled 5")
	}
	if order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
		t.Errorf("expected status PARTIALLY_FILLED, got %d", order.Status)
	}

	err = s.Fill(order.ID, types.Quantity(fixed.NewI(5, 0)))
	if err != nil {
		t.Errorf("fill failed: %v", err)
	}
	if math.Cmp(order.Filled, types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected filled 10")
	}
	if order.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("expected status FILLED, got %d", order.Status)
	}
}

func TestService_Concurrent(t *testing.T) {
	s := NewService()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.Create(types.UserID(id), "BTCUSDT", constants.CATEGORY_LINEAR,
				constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
				math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
				false, false, 0)
		}(i)
	}

	wg.Wait()
	if s.Count() != 100 {
		t.Errorf("expected count 100, got %d", s.Count())
	}
}

func BenchmarkService_Create(b *testing.B) {
	s := NewService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Create(types.UserID(i), "BTCUSDT", constants.CATEGORY_LINEAR,
			constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
			math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
			false, false, 0)
	}
}

func BenchmarkService_Get(b *testing.B) {
	s := NewService()
	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		math.Zero, types.Quantity(fixed.NewI(10, 0)), math.Zero,
		false, false, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get(order.ID)
	}
}
