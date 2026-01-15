package oms

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestService_Sync_CreateRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected exposure 10, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	found := false
	h := s.reduceonly.buyHeaps["BTCUSDT"]
	for _, item := range h.items {
		if item.ID == order.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("order not found in buyHeaps")
	}
}

func TestService_Sync_CreateConditional(t *testing.T) {
	s := NewService()

	s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	if s.conditional.buyTriggers["BTCUSDT"] == nil {
		t.Error("buyTriggers[BTCUSDT] should be created")
	}
}

func TestService_Sync_CancelRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Cancel(order.ID)

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	if !s.reduceonly.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

func TestService_Sync_CancelConditional(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	s.Cancel(order.ID)

	if !s.conditional.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

func TestService_Sync_FillPartialRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Fill(order.ID, types.Quantity(fixed.NewI(5, 0)))

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected exposure 5, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}
}

func TestService_Sync_FillFullRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Fill(order.ID, types.Quantity(fixed.NewI(10, 0)))

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	if !s.reduceonly.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

func TestService_Sync_FillFullConditional(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	s.Fill(order.ID, types.Quantity(fixed.NewI(10, 0)))

	if !s.conditional.deleted[&order.ID] {
		t.Error("conditional order should be marked as deleted after full fill")
	}
}

func TestService_Sync_AmendRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Amend(order.ID, types.Quantity(fixed.NewI(7, 0)))

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(7, 0))) != 0 {
		t.Errorf("expected exposure 7, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	if order.Quantity.Cmp(types.Quantity(fixed.NewI(7, 0))) != 0 {
		t.Errorf("expected quantity 7, got %d", order.Quantity)
	}
}

func TestService_Sync_CancelNonRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	s.Cancel(order.ID)

	if s.reduceonly.buyHeaps["BTCUSDT"] != nil {
		t.Error("non-RO order should not be in reduceonly index")
	}
}

func TestService_Sync_CancelNonConditional(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	s.Cancel(order.ID)

	if s.conditional.buyTriggers["BTCUSDT"] != nil {
		t.Error("non-conditional order should not be in conditional index")
	}
}
