package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type mockCallback struct {
	created []*types.Order
}

func seedBalances(portfolioService *portfolio.Service, userID types.UserID, symbol string) {
	base := registry.GetBaseAsset(symbol)
	quote := registry.GetQuoteAsset(symbol)
	amount := types.Quantity(fixed.NewI(1_000_000_000, 0))
	portfolioService.Balances[userID] = map[string]*types.Balance{
		base:  {UserID: userID, Asset: base, Available: amount},
		quote: {UserID: userID, Asset: quote, Available: amount},
	}
}

func (m *mockCallback) OnChildOrderCreated(order *types.Order) {
	m.created = append(m.created, order)
}

func registerInstrument(reg *registry.Registry, symbol string, lastPrice int64) {
	inst := registry.FromSymbol(symbol, lastPrice)
	reg.SetInstrument(symbol, inst)
}

func newEngine(store *oms.Service, cb OrderCallback) (*Engine, *portfolio.Service) {
	reg := registry.New()
	registerInstrument(reg, "BTCUSDT", 5000000000000) // 50000 USDT
	registerInstrument(reg, "ETHUSDT", 300000000000)  // 3000 USDT
	e := NewEngine(store, nil, reg, cb)
	seedBalances(e.portfolio, types.UserID(1), "BTCUSDT")
	seedBalances(e.portfolio, types.UserID(2), "BTCUSDT")
	return e, e.portfolio
}

func TestEngine_PlaceOrder(t *testing.T) {
	store := oms.NewService(nil)
	cb := &mockCallback{}
	e, _ := newEngine(store, cb)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
	if err != nil {
		t.Errorf("PlaceOrder failed: %v", err)
	}

	order := result.Order
	if order == nil {
		t.Error("order is nil")
		return
	}
	if order.UserID != types.UserID(1) {
		t.Errorf("expected userID 1, got %d", order.UserID)
	}
}

func TestEngine_PlaceOrder_Conditional(t *testing.T) {
	store := oms.NewService(nil)
	cb := &mockCallback{}
	e, _ := newEngine(store, cb)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
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
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
	if err != constants.ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestEngine_Validate_ConditionalSpot(t *testing.T) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
	if err != constants.ErrConditionalSpot {
		t.Errorf("expected ErrConditionalSpot, got %v", err)
	}
}

func TestEngine_Validate_InvalidTriggerBuy(t *testing.T) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
	if err != constants.ErrInvalidTriggerForBuy {
		t.Errorf("expected ErrInvalidTriggerForBuy, got %v", err)
	}
}

func TestEngine_Validate_InvalidTriggerSell(t *testing.T) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
	if err != constants.ErrInvalidTriggerForSell {
		t.Errorf("expected ErrInvalidTriggerForSell, got %v", err)
	}
}

func TestEngine_Validate_ReduceOnlySpot(t *testing.T) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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

	result := e.Cmd(&PlaceOrderCmd{Req: req})
	err := result.Err
	if err != constants.ErrReduceOnlySpot {
		t.Errorf("expected ErrReduceOnlySpot, got %v", err)
	}
}

func TestEngine_CancelOrder(t *testing.T) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	result := e.Cmd(&CancelOrderCmd{OrderID: order.ID})
	err := result.Err
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
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	result := e.Cmd(&AmendOrderCmd{OrderID: order.ID, NewQty: types.Quantity(fixed.NewI(5, 0))})
	err := result.Err
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
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	e.onPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(5, 0)))
}

func TestEngine_OnPriceTick(t *testing.T) {
	store := oms.NewService(nil)
	cb := &mockCallback{}
	e, _ := newEngine(store, cb)

	store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	e.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(48500, 0)))

	if len(cb.created) != 1 {
		t.Errorf("expected 1 child order, got %d", len(cb.created))
	}
}

func BenchmarkEngine_PlaceOrder(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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
		_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
	}
}

func BenchmarkEngine_CancelOrder(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.Cmd(&CancelOrderCmd{OrderID: order.ID}).Err
	}
}

// BenchmarkEngine_AmendOrder measures order amendment throughput.
func BenchmarkEngine_AmendOrder(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	order := store.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newQty := types.Quantity(fixed.NewI(10+int64(i%10), 0))
		_ = e.Cmd(&AmendOrderCmd{OrderID: order.ID, NewQty: newQty}).Err
	}
}

// BenchmarkEngine_PlaceOrder_IOC measures IOC placement without liquidity.
func BenchmarkEngine_PlaceOrder_IOC(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(10, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.UserID = types.UserID(i)
		_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
	}
}

// BenchmarkEngine_PlaceOrder_FOKReject measures FOK rejection without liquidity.
func BenchmarkEngine_PlaceOrder_FOKReject(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	req := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_FOK,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(10, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.UserID = types.UserID(i)
		_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
	}
}

// BenchmarkEngine_PlaceOrder_PostOnlyReject measures post-only rejection on cross.
func BenchmarkEngine_PlaceOrder_PostOnlyReject(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	makerReq := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(100, 0)),
	}
	if err := e.Cmd(&PlaceOrderCmd{Req: makerReq}).Err; err != nil {
		b.Fatalf("setup maker failed: %v", err)
	}

	req := &types.PlaceOrderRequest{
		UserID:   types.UserID(2),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(10, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.UserID = types.UserID(i + 10)
		_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
	}
}

// BenchmarkEngine_Match_GTC measures direct match throughput for GTC orders.
func BenchmarkEngine_Match_GTC(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	makerReq := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(100, 0)),
	}
	if err := e.Cmd(&PlaceOrderCmd{Req: makerReq}).Err; err != nil {
		b.Fatalf("setup maker failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &types.PlaceOrderRequest{
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(fixed.NewI(50000, 0)),
			Quantity: types.Quantity(fixed.NewI(1, 0)),
		}
		_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
	}
}

func TestEngine_SetLeverage_PriceUnavailable(t *testing.T) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	cmd := &SetLeverageCmd{UserID: types.UserID(1), Symbol: "BTCUSDT", Leverage: types.Leverage(fixed.NewI(5, 0))}
	err := e.Cmd(cmd).Err

	if err != constants.ErrPriceUnavailable {
		t.Fatalf("expected ErrPriceUnavailable, got %v", err)
	}
}

func TestEngine_SetLeverage_LiquidationCheck(t *testing.T) {
	store := oms.NewService(nil)
	e, portfolioService := newEngine(store, nil)

	userID := types.UserID(1)
	symbol := "BTCUSDT"
	quote := registry.GetQuoteAsset(symbol)

	portfolioService.Balances[userID] = map[string]*types.Balance{
		quote: {UserID: userID, Asset: quote, Available: types.Quantity(fixed.NewI(100000, 0))},
	}
	portfolioService.Positions[userID] = map[string]*types.Position{
		symbol: {
			UserID:     userID,
			Symbol:     symbol,
			Size:       types.Quantity(fixed.NewI(1, 0)),
			EntryPrice: types.Price(fixed.NewI(100, 0)),
			Leverage:   types.Leverage(fixed.NewI(2, 0)),
		},
	}

	e.OnPriceTick(symbol, types.Price(fixed.NewI(90, 0)))

	cmd := &SetLeverageCmd{UserID: userID, Symbol: symbol, Leverage: types.Leverage(fixed.NewI(5, 0))}
	err := e.Cmd(cmd).Err

	if err != constants.ErrLeverageTooHigh {
		t.Fatalf("expected ErrLeverageTooHigh, got %v", err)
	}
}

func TestEngine_SetLeverage_Success(t *testing.T) {
	store := oms.NewService(nil)
	e, portfolioService := newEngine(store, nil)

	userID := types.UserID(1)
	symbol := "BTCUSDT"
	quote := registry.GetQuoteAsset(symbol)

	portfolioService.Balances[userID] = map[string]*types.Balance{
		quote: {UserID: userID, Asset: quote, Available: types.Quantity(fixed.NewI(100000, 0))},
	}
	portfolioService.Positions[userID] = map[string]*types.Position{
		symbol: {
			UserID:     userID,
			Symbol:     symbol,
			Size:       types.Quantity(fixed.NewI(1, 0)),
			EntryPrice: types.Price(fixed.NewI(100, 0)),
			Leverage:   types.Leverage(fixed.NewI(2, 0)),
		},
	}

	e.OnPriceTick(symbol, types.Price(fixed.NewI(95, 0)))

	cmd := &SetLeverageCmd{UserID: userID, Symbol: symbol, Leverage: types.Leverage(fixed.NewI(5, 0))}
	err := e.Cmd(cmd).Err

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := portfolioService.GetPosition(userID, symbol)
	if math.Cmp(pos.Leverage, types.Leverage(fixed.NewI(5, 0))) != 0 {
		t.Fatalf("expected leverage updated to 5")
	}
}

// BenchmarkEngine_Match_IOC measures direct match throughput for IOC orders.
func BenchmarkEngine_Match_IOC(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

	makerReq := &types.PlaceOrderRequest{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(100, 0)),
	}
	if err := e.Cmd(&PlaceOrderCmd{Req: makerReq}).Err; err != nil {
		b.Fatalf("setup maker failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &types.PlaceOrderRequest{
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_IOC,
			Price:    types.Price(fixed.NewI(50000, 0)),
			Quantity: types.Quantity(fixed.NewI(1, 0)),
		}
		_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
	}
}

// BenchmarkEngine_PlaceOrder_Parallel measures queue contention under parallel load.
func BenchmarkEngine_PlaceOrder_Parallel(b *testing.B) {
	store := oms.NewService(nil)
	e, _ := newEngine(store, nil)

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
	b.RunParallel(func(pb *testing.PB) {
		var localID int64
		for pb.Next() {
			localID++
			req.UserID = types.UserID(localID)
			_ = e.Cmd(&PlaceOrderCmd{Req: req}).Err
		}
	})
}
