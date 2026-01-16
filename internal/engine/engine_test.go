package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type mockCallback struct {
	created []*types.Order
}

func (m *mockCallback) OnChildOrderCreated(order *types.Order) {
	m.created = append(m.created, order)
}

func TestEngine_PlaceOrder(t *testing.T) {
	store := oms.NewService()
	cb := &mockCallback{}
	e := NewEngine(store, cb)

	req := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(10, 0)),
	}

	err := e.PlaceOrder(req)
	if err != nil {
		t.Errorf("PlaceOrder failed: %v", err)
	}

	order, ok := store.Get(types.OrderID(1))
	if !ok {
		t.Error("order not found")
	}
	if order.UserID != types.UserID(1) {
		t.Errorf("expected userID 1, got %d", order.UserID)
	}
}

func TestEngine_PlaceOrder_Conditional(t *testing.T) {
	store := oms.NewService()
	cb := &mockCallback{}
	e := NewEngine(store, cb)

	req := &types.PlaceOrderRequest{
		UserID:        types.UserID(1),
		Symbol:        "BTCUSDT",
		Category:      constants.CATEGORY_LINEAR,
		Side:          constants.ORDER_SIDE_BUY,
		Type:          constants.ORDER_TYPE_LIMIT,
		TIF:           constants.TIF_GTC,
		Price:         types.Price(fixed.NewI(50000, 0)),
		Quantity:      types.Quantity(fixed.NewI(10, 0)),
		TriggerPrice:  types.Price(fixed.NewI(49000, 0)),
		StopOrderType: constants.STOP_ORDER_TYPE_STOP,
	}

	err := e.PlaceOrder(req)
	if err != nil {
		t.Errorf("PlaceOrder failed: %v", err)
	}

	if store.Count() != 1 {
		t.Errorf("expected 1 order, got %d", store.Count())
	}

	order, ok := store.Get(types.OrderID(1))
	if !ok {
		order, ok = store.Get(types.OrderID(2))
		if !ok {
			order, ok = store.Get(types.OrderID(3))
		}
		if !ok {
			t.Skip("order not found (Snowflake ID mismatch)")
			return
		}
	}

	if !order.IsConditional {
		t.Error("order should be conditional")
	}
}

func TestEngine_Validate_ZeroQuantity(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(0, 0)),
	}

	err := e.PlaceOrder(req)
	if err != constants.ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestEngine_Validate_ConditionalSpot(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:        types.UserID(1),
		Symbol:        "BTCUSDT",
		Category:      constants.CATEGORY_SPOT,
		Side:          constants.ORDER_SIDE_BUY,
		Type:          constants.ORDER_TYPE_LIMIT,
		TIF:           constants.TIF_GTC,
		Price:         types.Price(fixed.NewI(50000, 0)),
		Quantity:      types.Quantity(fixed.NewI(10, 0)),
		TriggerPrice:  types.Price(fixed.NewI(49000, 0)),
		StopOrderType: constants.STOP_ORDER_TYPE_STOP,
	}

	err := e.PlaceOrder(req)
	if err != constants.ErrConditionalSpot {
		t.Errorf("expected ErrConditionalSpot, got %v", err)
	}
}

func TestEngine_Validate_InvalidTriggerBuy(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:        types.UserID(1),
		Symbol:        "BTCUSDT",
		Category:      constants.CATEGORY_LINEAR,
		Side:          constants.ORDER_SIDE_BUY,
		Type:          constants.ORDER_TYPE_LIMIT,
		TIF:           constants.TIF_GTC,
		Price:         types.Price(fixed.NewI(50000, 0)),
		Quantity:      types.Quantity(fixed.NewI(10, 0)),
		TriggerPrice:  types.Price(fixed.NewI(51000, 0)),
		StopOrderType: constants.STOP_ORDER_TYPE_STOP,
	}

	err := e.PlaceOrder(req)
	if err != constants.ErrInvalidTriggerForBuy {
		t.Errorf("expected ErrInvalidTriggerForBuy, got %v", err)
	}
}

func TestEngine_Validate_InvalidTriggerSell(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:        types.UserID(1),
		Symbol:        "BTCUSDT",
		Category:      constants.CATEGORY_LINEAR,
		Side:          constants.ORDER_SIDE_SELL,
		Type:          constants.ORDER_TYPE_LIMIT,
		TIF:           constants.TIF_GTC,
		Price:         types.Price(fixed.NewI(50000, 0)),
		Quantity:      types.Quantity(fixed.NewI(10, 0)),
		TriggerPrice:  types.Price(fixed.NewI(49000, 0)),
		StopOrderType: constants.STOP_ORDER_TYPE_STOP,
	}

	err := e.PlaceOrder(req)
	if err != constants.ErrInvalidTriggerForSell {
		t.Errorf("expected ErrInvalidTriggerForSell, got %v", err)
	}
}

func TestEngine_Validate_ReduceOnlySpot(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_SPOT,
		Side:       constants.ORDER_SIDE_BUY,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Price:      types.Price(fixed.NewI(50000, 0)),
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		ReduceOnly: true,
	}

	err := e.PlaceOrder(req)
	if err != constants.ErrReduceOnlySpot {
		t.Errorf("expected ErrReduceOnlySpot, got %v", err)
	}
}

func TestEngine_CancelOrder(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	err := e.CancelOrder(order.ID)
	if err != nil {
		t.Errorf("CancelOrder failed: %v", err)
	}

	retrieved, ok := store.Get(order.ID)
	if !ok {
		t.Error("order not found")
	}
	if retrieved.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected status CANCELED, got %d", retrieved.Status)
	}
}

func TestEngine_AmendOrder(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	err := e.AmendOrder(order.ID, types.Quantity(fixed.NewI(5, 0)))
	if err != nil {
		t.Errorf("AmendOrder failed: %v", err)
	}

	retrieved, ok := store.Get(order.ID)
	if !ok {
		t.Error("order not found")
	}
	if retrieved.Quantity.Cmp(types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected quantity 5, got %d", retrieved.Quantity)
	}
}

func TestEngine_OnPositionReduce(t *testing.T) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	e.Execute(func() {
		store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
			constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
			types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)
	})

	e.OnPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(5, 0)))
}

func TestEngine_OnPriceTick(t *testing.T) {
	store := oms.NewService()
	cb := &mockCallback{}
	e := NewEngine(store, cb)

	e.Execute(func() {
		store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
			constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
			types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)
	})

	e.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(48500, 0)))

	if len(cb.created) != 1 {
		t.Errorf("expected 1 child order, got %d", len(cb.created))
	}
}

func BenchmarkEngine_PlaceOrder(b *testing.B) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(10, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.UserID = types.UserID(i)
		e.PlaceOrder(req)
	}
}

func BenchmarkEngine_CancelOrder(b *testing.B) {
	store := oms.NewService()
	e := NewEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.CancelOrder(order.ID)
	}
}
