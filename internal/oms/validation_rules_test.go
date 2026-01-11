package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type countingClearing struct {
	reserveCalls int
	lastQty      types.Quantity
	lastPrice    types.Price
	lastSide     int8
	lastCategory int8
	lastSymbol   string
}

func (c *countingClearing) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	c.reserveCalls++
	c.lastQty = qty
	c.lastPrice = price
	c.lastSide = side
	c.lastCategory = category
	c.lastSymbol = symbol
	return nil
}

func (c *countingClearing) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
}

func (c *countingClearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
}

func newTestServiceWithClearing(clearing Clearing) (*Service, *testPortfolio) {
	portfolio := &testPortfolio{
		positions: make(map[types.UserID]map[string]*types.Position),
	}
	s, _ := New(Config{}, portfolio, clearing)
	return s, portfolio
}

func TestValidateOrder_SpotRestrictions(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_SPOT,
		Side:       constants.ORDER_SIDE_BUY,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlySpot {
		t.Fatalf("expected ErrReduceOnlySpot, got %v", err)
	}

	input.ReduceOnly = false
	input.TriggerPrice = 90
	if err := s.validateOrder(input); err != ErrConditionalSpot {
		t.Fatalf("expected ErrConditionalSpot, got %v", err)
	}

	input.TriggerPrice = 0
	input.CloseOnTrigger = true
	if err := s.validateOrder(input); err != ErrCloseOnTriggerSpot {
		t.Fatalf("expected ErrCloseOnTriggerSpot, got %v", err)
	}
}

func TestValidateOrder_LinearMarketTIF(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
	}

	if err := s.validateOrder(input); err != ErrMarketTIF {
		t.Fatalf("expected ErrMarketTIF, got %v", err)
	}
}

func TestValidateOrder_InvalidFields(t *testing.T) {
	s, _ := newTestService()

	base := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	}

	input := *base
	input.Category = 9
	if err := s.validateOrder(&input); err != ErrInvalidCategory {
		t.Fatalf("expected ErrInvalidCategory, got %v", err)
	}

	input = *base
	input.Side = 9
	if err := s.validateOrder(&input); err != ErrInvalidSide {
		t.Fatalf("expected ErrInvalidSide, got %v", err)
	}

	input = *base
	input.Type = 9
	if err := s.validateOrder(&input); err != ErrInvalidOrderType {
		t.Fatalf("expected ErrInvalidOrderType, got %v", err)
	}

	input = *base
	input.TIF = 9
	if err := s.validateOrder(&input); err != ErrInvalidTIF {
		t.Fatalf("expected ErrInvalidTIF, got %v", err)
	}

	input = *base
	input.StopOrderType = 9
	if err := s.validateOrder(&input); err != ErrInvalidStopOrderType {
		t.Fatalf("expected ErrInvalidStopOrderType, got %v", err)
	}

	input = *base
	input.Symbol = "!"
	if err := s.validateOrder(&input); err != ErrInvalidSymbol {
		t.Fatalf("expected ErrInvalidSymbol, got %v", err)
	}
}

func TestValidateOrder_CloseOnTriggerRequiresTrigger(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       1,
		Price:          100,
		CloseOnTrigger: true,
		TriggerPrice:   0,
	}
	if err := s.validateOrder(input); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice, got %v", err)
	}
}

func TestValidateOrder_PostOnlyWouldCross(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}
	if err := s.validateOrder(input); err != ErrPostOnlyWouldCross {
		t.Fatalf("expected ErrPostOnlyWouldCross, got %v", err)
	}
}

func TestValidateOrder_PostOnlyDoesNotCross(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    105,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}
	if err := s.validateOrder(input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateOrder_PostOnlyEqualPriceRejected(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}
	if err := s.validateOrder(input); err != ErrPostOnlyWouldCross {
		t.Fatalf("expected ErrPostOnlyWouldCross, got %v", err)
	}
}

func TestPlaceOrder_PostOnlyRejectsBeforeReserve(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != ErrPostOnlyWouldCross {
		t.Fatalf("expected ErrPostOnlyWouldCross, got %v", err)
	}
	if clearing.reserveCalls != 0 {
		t.Fatalf("expected no reserve calls, got %d", clearing.reserveCalls)
	}
}

func TestValidateOCO_ChildTriggersRequired(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
		OCO: &types.OCOInput{
			TakeProfit: types.OCOChildOrder{TriggerPrice: 0, Price: 99, ReduceOnly: true},
			StopLoss:   types.OCOChildOrder{TriggerPrice: 90, Price: 91, ReduceOnly: true},
		},
	}
	if err := s.validateOCO(input); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice, got %v", err)
	}
}

func TestValidateOrder_TriggerPriceRules(t *testing.T) {
	s, _ := newTestService()
	s.lastPrices["BTCUSDT"] = 100

	buy := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        100,
		TriggerPrice: 100,
	}
	if err := s.validateOrder(buy); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice for BUY trigger >= price, got %v", err)
	}

	sell := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_SELL,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        100,
		TriggerPrice: 100,
	}
	if err := s.validateOrder(sell); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice for SELL trigger <= price, got %v", err)
	}
}

func TestValidateOrder_ReduceOnlyRequiresPosition(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlyNoPosition {
		t.Fatalf("expected ErrReduceOnlyNoPosition, got %v", err)
	}
}

func TestValidateOrder_ReduceOnlySideMismatch(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_BUY,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlySide {
		t.Fatalf("expected ErrReduceOnlySide, got %v", err)
	}
}

func TestValidateOrder_ReduceOnlyCommitmentExceeded(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)
	s.reduceOnlyCommitment[1] = map[string]int64{"BTCUSDT": 1}

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlyCommitmentExceeded {
		t.Fatalf("expected ErrReduceOnlyCommitmentExceeded, got %v", err)
	}
}

func TestReduceOnlyCommitmentUpdates(t *testing.T) {
	clearing := &countingClearing{}
	s, portfolio := newTestServiceWithClearing(clearing)
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   2,
		Price:      100,
		ReduceOnly: true,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.reduceOnlyCommitment[1]["BTCUSDT"] != 2 {
		t.Fatalf("expected commitment 2, got %d", s.reduceOnlyCommitment[1]["BTCUSDT"])
	}

	s.cancelOrder(result.Orders[0])
	if s.reduceOnlyCommitment[1]["BTCUSDT"] != 0 {
		t.Fatalf("expected commitment 0 after cancel, got %d", s.reduceOnlyCommitment[1]["BTCUSDT"])
	}
}

func TestValidateOrder_OCOOnSpotRejected(t *testing.T) {
	s, _ := newTestService()
	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
		OCO: &types.OCOInput{
			TakeProfit: types.OCOChildOrder{TriggerPrice: 120, Price: 119, ReduceOnly: true},
			StopLoss:   types.OCOChildOrder{TriggerPrice: 80, Price: 81, ReduceOnly: true},
		},
	}
	if err := s.validateOCO(input); err != ErrOCOSpot {
		t.Fatalf("expected ErrOCOSpot, got %v", err)
	}
}

func TestCheckSelfMatchMarket(t *testing.T) {
	s, _ := newTestService()

	order := &types.Order{
		ID:       10,
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	}
	s.storeOrder(order)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 1,
	}

	if err := s.checkSelfMatch(input); err != ErrSelfMatch {
		t.Fatalf("expected ErrSelfMatch, got %v", err)
	}
}

func TestCheckSelfMatchLimit(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	ask := &types.Order{
		ID:       10,
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	}
	ob.Add(ask)
	s.storeOrder(ask)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	}

	if err := s.checkSelfMatch(input); err != ErrSelfMatch {
		t.Fatalf("expected ErrSelfMatch, got %v", err)
	}
}

func TestValidateOrder_QuantityZeroRules(t *testing.T) {
	s, _ := newTestService()

	regular := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 0,
		Price:    100,
	}
	if err := s.validateOrder(regular); err != ErrInvalidQuantity {
		t.Fatalf("expected ErrInvalidQuantity, got %v", err)
	}

	conditional := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     0,
		Price:        100,
		TriggerPrice: 90,
	}
	if err := s.validateOrder(conditional); err != nil {
		t.Fatalf("expected no error for conditional qty=0, got %v", err)
	}
}

func TestValidateOCO_Rules(t *testing.T) {
	s, portfolio := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
		OCO: &types.OCOInput{
			TakeProfit: types.OCOChildOrder{TriggerPrice: 120, Price: 119, ReduceOnly: true},
			StopLoss:   types.OCOChildOrder{TriggerPrice: 110, Price: 111, ReduceOnly: true},
		},
	}

	if err := s.validateOCO(input); err != ErrOCONoPosition {
		t.Fatalf("expected ErrOCONoPosition, got %v", err)
	}

	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_LONG)
	input.OCO.TakeProfit.TriggerPrice = 110
	input.OCO.StopLoss.TriggerPrice = 120
	if err := s.validateOCO(input); err != ErrOCOTPTriggerInvalid {
		t.Fatalf("expected ErrOCOTPTriggerInvalid, got %v", err)
	}

	input.OCO.TakeProfit.TriggerPrice = 130
	input.OCO.StopLoss.TriggerPrice = 120
	if err := s.validateOCO(input); err != nil {
		t.Fatalf("expected no error for LONG with TP>SL, got %v", err)
	}

	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_SHORT)
	input.OCO.TakeProfit.TriggerPrice = 120
	input.OCO.StopLoss.TriggerPrice = 130
	if err := s.validateOCO(input); err != nil {
		t.Fatalf("expected no error for SHORT with TP<SL, got %v", err)
	}

	input.OCO.TakeProfit.TriggerPrice = 140
	if err := s.validateOCO(input); err != ErrOCOSLTriggerInvalid {
		t.Fatalf("expected ErrOCOSLTriggerInvalid, got %v", err)
	}
}

func TestPlaceOrder_FOKPrecheckRejects(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	maker := &types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_FOK,
		Quantity: 2,
		Price:    100,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != ErrFOKInsufficientLiquidity {
		t.Fatalf("expected ErrFOKInsufficientLiquidity, got %v", err)
	}
	if clearing.reserveCalls != 0 {
		t.Fatalf("expected no reserve calls, got %d", clearing.reserveCalls)
	}
	if len(s.orders) != 0 {
		t.Fatalf("expected no stored orders on FOK rejection")
	}
}

func TestPlaceOrder_OCOCreation(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    0,
		OCO: &types.OCOInput{
			Quantity: 0,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 60000,
				Price:        59900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 45000,
				Price:        45100,
				ReduceOnly:   true,
			},
		},
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(result.Orders))
	}

	tp := result.Orders[0]
	sl := result.Orders[1]
	if tp.OrderLinkId == 0 || sl.OrderLinkId == 0 || tp.OrderLinkId != sl.OrderLinkId {
		t.Fatalf("expected shared OrderLinkId >0, got %d and %d", tp.OrderLinkId, sl.OrderLinkId)
	}
	if !tp.IsConditional || !sl.IsConditional {
		t.Fatalf("expected OCO orders to be conditional")
	}
	if !tp.CloseOnTrigger || !sl.CloseOnTrigger {
		t.Fatalf("expected CloseOnTrigger=true for OCO orders")
	}
	if !tp.ReduceOnly || !sl.ReduceOnly {
		t.Fatalf("expected ReduceOnly=true for OCO orders")
	}
	if tp.Quantity != 0 || sl.Quantity != 0 {
		t.Fatalf("expected quantity=0 for OCO orders, got %d and %d", tp.Quantity, sl.Quantity)
	}
	if tp.StopOrderType != constants.STOP_ORDER_TYPE_TAKE_PROFIT {
		t.Fatalf("expected TP stopOrderType, got %d", tp.StopOrderType)
	}
	if sl.StopOrderType != constants.STOP_ORDER_TYPE_STOP_LOSS {
		t.Fatalf("expected SL stopOrderType, got %d", sl.StopOrderType)
	}
}

func TestPlaceOrder_ConditionalSkipsReserve(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        100,
		TriggerPrice: 90,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if clearing.reserveCalls != 0 {
		t.Fatalf("expected no reserve calls, got %d", clearing.reserveCalls)
	}
}

func TestCreateChildOrderInput_CloseOnTriggerUsesPositionSize(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 5, constants.SIDE_LONG)

	triggered := &types.Order{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          48000,
		TriggerPrice:   49000,
		CloseOnTrigger: true,
	}

	child := s.createChildOrderInput(triggered)
	if child == nil {
		t.Fatalf("expected child order input")
	}
	if child.Quantity != 5 {
		t.Fatalf("expected quantity from position size, got %d", child.Quantity)
	}
	if !child.ReduceOnly {
		t.Fatalf("expected reduceOnly=true for closeOnTrigger child")
	}
	if child.Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("expected opposite side, got %d", child.Side)
	}
	if child.Type != constants.ORDER_TYPE_LIMIT || child.Price != 48000 {
		t.Fatalf("expected limit child with same price, got type %d price %d", child.Type, child.Price)
	}
	if child.TIF != constants.TIF_GTC {
		t.Fatalf("expected GTC for limit child, got %d", child.TIF)
	}
}

func TestPlaceOrder_FOKFullyFills(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	for i := 0; i < 3; i++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   2,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Status:   constants.ORDER_STATUS_NEW,
			Price:    100,
			Quantity: 1,
		})
	}

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_FOK,
		Quantity: 3,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED status, got %d", result.Status)
	}
	if result.Remaining != 0 {
		t.Fatalf("expected remaining 0, got %d", result.Remaining)
	}
	if clearing.reserveCalls != 1 {
		t.Fatalf("expected 1 reserve call, got %d", clearing.reserveCalls)
	}
}

func TestPlaceOrder_IOCPartialFill(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("expected PARTIALLY_FILLED_CANCELED, got %d", result.Status)
	}
	if result.Filled != 1 || result.Remaining != 1 {
		t.Fatalf("expected filled 1 remaining 1, got filled %d remaining %d", result.Filled, result.Remaining)
	}
	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
}

func TestPlaceOrder_IOCFillsNothing(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_CANCELED {
		t.Fatalf("expected CANCELED, got %d", result.Status)
	}
	if result.Filled != 0 || result.Remaining != 2 {
		t.Fatalf("expected filled 0 remaining 2, got filled %d remaining %d", result.Filled, result.Remaining)
	}
	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
}
