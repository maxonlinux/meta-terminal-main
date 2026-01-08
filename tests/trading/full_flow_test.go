package trading

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

type TradingTest struct {
	t          *testing.T
	state      *state.EngineState
	orderStore *state.OrderStore
	spot       *spot.Spot
	linear     *linear.Linear
}

func NewTradingTest(t *testing.T) *TradingTest {
	s := state.NewEngineState()
	orderStore := state.NewOrderStore()

	return &TradingTest{
		t:          t,
		state:      s,
		orderStore: orderStore,
		spot:       spot.New(s, orderStore),
		linear:     linear.New(s, orderStore),
	}
}

// SetupUser создаёт пользователя с начальным балансом
func (tt *TradingTest) SetupUser(userID types.UserID, asset string, available int64) {
	us := tt.state.GetUserState(userID)
	us.Balances[asset] = types.NewUserBalance(userID, asset)
	us.Balances[asset].Buckets[types.BUCKET_AVAILABLE] = available
}

// TestSPOTBuyLimitOrder тестирует размещение лимитного ордера на покупку SPOT
func (tt *TradingTest) TestSPOTBuyLimitOrder() {
	tt.t.Run("SPOT_BuyLimitOrder", func(t *testing.T) {
		tt.spot.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 1000000)

		result, err := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000),
		})

		if err != nil {
			t.Fatalf("PlaceOrder failed: %v", err)
		}

		if result.Status != constants.ORDER_STATUS_NEW {
			t.Errorf("Expected NEW status, got %d", result.Status)
		}

		// Проверяем что баланс заблокирован
		us := tt.state.GetUserState(userID)
		usdtBal := us.Balances["USDT"]
		if usdtBal.Buckets[types.BUCKET_LOCKED] != 50000 {
			t.Errorf("Expected LOCKED=50000, got %d", usdtBal.Buckets[types.BUCKET_LOCKED])
		}
		if usdtBal.Buckets[types.BUCKET_AVAILABLE] != 950000 {
			t.Errorf("Expected AVAILABLE=950000, got %d", usdtBal.Buckets[types.BUCKET_AVAILABLE])
		}

		// Проверяем что ордер в книге
		bids, _ := tt.spot.GetOrderBook("BTCUSDT", 10)
		if len(bids) == 0 {
			t.Error("Expected bids in orderbook")
		}
	})
}

// TestSPOTSellLimitOrder тестирует размещение лимитного ордера на продажу SPOT
func (tt *TradingTest) TestSPOTSellLimitOrder() {
	tt.t.Run("SPOT_SellLimitOrder", func(t *testing.T) {
		tt.spot.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 1000000)
		tt.SetupUser(userID, "BTC", 100) // 1 BTC with precision 4

		result, err := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(55000),
		})

		if err != nil {
			t.Fatalf("PlaceOrder failed: %v", err)
		}

		if result.Status != constants.ORDER_STATUS_NEW {
			t.Errorf("Expected NEW status, got %d", result.Status)
		}
	})
}

// TestSPOTMatchOrders тестирует полное исполнение ордера
func (tt *TradingTest) TestSPOTMatchOrders() {
	tt.t.Run("SPOT_MatchOrders", func(t *testing.T) {
		tt.spot.Reset()
		makerID := types.UserID(1)
		takerID := types.UserID(2)

		tt.SetupUser(makerID, "USDT", 1000000)
		tt.SetupUser(makerID, "BTC", 100)
		tt.SetupUser(takerID, "USDT", 1000000)

		// Создаём ask (продажа) от maker
		_, err := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   makerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000),
		})
		if err != nil {
			t.Fatalf("Maker order failed: %v", err)
		}

		// Создаём bid (покупка) от taker (сразу исполняет, так как цена пересекается)
		result, err := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   takerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_FOK,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000), // Исполняет сразу по цене maker
		})
		if err != nil {
			t.Fatalf("Taker order failed: %v", err)
		}

		// Проверяем что ордер исполнился
		if result.Status != constants.ORDER_STATUS_FILLED {
			t.Errorf("Expected FILLED status, got %d", result.Status)
		}

		if result.Filled != types.Quantity(1) {
			t.Errorf("Expected Filled=1, got %d", result.Filled)
		}

		// Проверяем балансы
		makerUSDT := tt.state.GetUserState(makerID).Balances["USDT"]
		takerUSDT := tt.state.GetUserState(takerID).Balances["USDT"]

		if makerUSDT.Buckets[types.BUCKET_AVAILABLE] != 1050000 {
			t.Errorf("Maker USDT expected 1050000, got %d", makerUSDT.Buckets[types.BUCKET_AVAILABLE])
		}

		if takerUSDT.Buckets[types.BUCKET_AVAILABLE] != 950000 {
			t.Errorf("Taker USDT expected 950000, got %d", takerUSDT.Buckets[types.BUCKET_AVAILABLE])
		}
	})
}

// TestSPOTFOKPartialFill тестирует что FOK отменяет частичное исполнение
func (tt *TradingTest) TestSPOTFOKPartialFill() {
	tt.t.Run("SPOT_FOKPartialCancel", func(t *testing.T) {
		tt.spot.Reset()
		makerID := types.UserID(1)
		takerID := types.UserID(2)

		tt.SetupUser(makerID, "USDT", 1000000)
		tt.SetupUser(takerID, "USDT", 1000000)

		// Maker продаёт только 0.5 BTC
		_, _ = tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   makerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(5), // 0.5 BTC с точностью 4
			Price:    types.Price(50000),
		})

		// Taker хочет купить 1 BTC FOK - должен получить CANCEL
		result, err := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   takerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_MARKET,
			TIF:      constants.TIF_FOK,
			Quantity: types.Quantity(10), // 1 BTC
		})
		if err != nil {
			t.Fatalf("Taker order failed: %v", err)
		}

		if result.Status != constants.ORDER_STATUS_CANCELED {
			t.Errorf("Expected CANCELLED status for FOK partial, got %d", result.Status)
		}

		if result.Filled != 0 {
			t.Errorf("Expected Filled=0 for FOK partial, got %d", result.Filled)
		}
	})
}

// TestSPOTIOCPartialFill тестирует что IOC допускает частичное исполнение
func (tt *TradingTest) TestSPOTIOCPartialFill() {
	tt.t.Run("SPOT_IOCPartialFill", func(t *testing.T) {
		tt.spot.Reset()
		makerID := types.UserID(1)
		takerID := types.UserID(2)

		tt.SetupUser(makerID, "USDT", 1000000)
		tt.SetupUser(takerID, "USDT", 1000000)

		// Maker продаёт только 0.5 BTC
		_, _ = tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   makerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(5),
			Price:    types.Price(50000),
		})

		// Taker покупает 1 BTC IOC - получит частичное
		result, err := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   takerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_MARKET,
			TIF:      constants.TIF_IOC,
			Quantity: types.Quantity(10),
		})
		if err != nil {
			t.Fatalf("Taker order failed: %v", err)
		}

		if result.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
			t.Errorf("Expected PARTIALLY_FILLED_CANCELLED for IOC partial, got %d", result.Status)
		}

		if result.Filled != types.Quantity(5) {
			t.Errorf("Expected Filled=5 for IOC partial, got %d", result.Filled)
		}
	})
}

// TestLinearOpenPosition тестирует открытие позиции в LINEAR
func (tt *TradingTest) TestLinearOpenPosition() {
	tt.t.Run("Linear_OpenPosition", func(t *testing.T) {
		tt.linear.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 5000000) // 5M for default 2x leverage

		_, err := tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100), // 1 BTC
			Price:    types.Price(50000),
		})
		if err != nil {
			t.Fatalf("PlaceOrder failed: %v", err)
		}

		// Проверяем маржу (default leverage = 2)
		us := tt.state.GetUserState(userID)
		usdtBal := us.Balances["USDT"]
		expectedMargin := int64(100) * int64(50000) / 2 // 2500000

		if usdtBal.Buckets[types.BUCKET_MARGIN] != expectedMargin {
			t.Errorf("Expected MARGIN=%d, got %d", expectedMargin, usdtBal.Buckets[types.BUCKET_MARGIN])
		}

		// Проверяем позицию
		pos := us.Positions["BTCUSD"]
		if pos == nil {
			t.Fatal("Position not created")
		}

		if pos.Size != types.Quantity(100) {
			t.Errorf("Expected Position.Size=100, got %d", pos.Size)
		}

		if pos.Side != constants.ORDER_SIDE_BUY {
			t.Errorf("Expected Position.Side=BUY(0), got %d", pos.Side)
		}

		if pos.EntryPrice != types.Price(50000) {
			t.Errorf("Expected EntryPrice=50000, got %d", pos.EntryPrice)
		}
	})
}

// TestLinearReduceOnly тестирует reduceOnly ордера
func (tt *TradingTest) TestLinearReduceOnly() {
	tt.t.Run("Linear_ReduceOnly", func(t *testing.T) {
		tt.linear.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 5000000)

		// Открываем длинную позицию
		_, _ = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100),
			Price:    types.Price(50000),
		})

		// Проверяем что sell reduceOnly принимается
		result, err := tt.linear.PlaceOrder(&types.OrderInput{
			UserID:     userID,
			Symbol:     "BTCUSD",
			Category:   constants.CATEGORY_LINEAR,
			Side:       constants.ORDER_SIDE_SELL,
			Type:       constants.ORDER_TYPE_LIMIT,
			TIF:        constants.TIF_GTC,
			Quantity:   types.Quantity(50),
			Price:      types.Price(49000),
			ReduceOnly: true,
		})
		if err != nil {
			t.Errorf("ReduceOnly SELL should be accepted, got error: %v", err)
		}
		if result == nil {
			t.Error("ReduceOnly SELL should return result")
		}

		// Проверяем что buy reduceOnly отклоняется
		_, err = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:     userID,
			Symbol:     "BTCUSD",
			Category:   constants.CATEGORY_LINEAR,
			Side:       constants.ORDER_SIDE_BUY,
			Type:       constants.ORDER_TYPE_LIMIT,
			TIF:        constants.TIF_GTC,
			Quantity:   types.Quantity(50),
			Price:      types.Price(51000),
			ReduceOnly: true,
		})
		if err == nil {
			t.Error("ReduceOnly BUY should be rejected (increases position)")
		}
	})
}

// TestLinearPositionUpdate тестирует обновление позиции
func (tt *TradingTest) TestLinearPositionUpdate() {
	tt.t.Run("Linear_PositionUpdate", func(t *testing.T) {
		tt.linear.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 5000000)

		// Открываем длинную позицию 1 BTC @ 50000
		_, _ = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100),
			Price:    types.Price(50000),
		})

		us := tt.state.GetUserState(userID)
		pos := us.Positions["BTCUSD"]

		// Усредняем - докупаем ещё 0.5 BTC @ 45000
		_, _ = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(50),
			Price:    types.Price(45000),
		})

		// Проверяем что размер удвоился
		if pos.Size != types.Quantity(150) {
			t.Errorf("Expected Position.Size=150, got %d", pos.Size)
		}

		// Проверяем среднюю цену: (50000*1 + 45000*0.5) / 1.5 = 48333.33
		// В int64: (50000*100 + 45000*50) / 150 = 48333
		expectedAvg := int64(48333)
		if pos.EntryPrice != types.Price(expectedAvg) {
			t.Errorf("Expected EntryPrice=%d, got %d", expectedAvg, pos.EntryPrice)
		}
	})
}

// TestLinearClosePosition тестирует закрытие позиции
func (tt *TradingTest) TestLinearClosePosition() {
	tt.t.Run("Linear_ClosePosition", func(t *testing.T) {
		tt.linear.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 5000000)

		// Открываем длинную позицию
		_, _ = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100),
			Price:    types.Price(50000),
		})

		// Продаём всю позицию
		_, _ = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100),
			Price:    types.Price(55000),
		})

		us := tt.state.GetUserState(userID)
		pos := us.Positions["BTCUSD"]

		// Проверяем что позиция закрыта
		if pos.Size != 0 {
			t.Errorf("Expected Position.Size=0 (closed), got %d", pos.Size)
		}

		if pos.Side != types.SIDE_NONE {
			t.Errorf("Expected Position.Side=NONE(-1), got %d", pos.Side)
		}
	})
}

// TestLinearLiquidation тестирует ликвидацию
func (tt *TradingTest) TestLinearLiquidation() {
	tt.t.Run("Linear_Liquidation", func(t *testing.T) {
		tt.linear.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 5000000)

		// Открываем длинню позицию
		_, _ = tt.linear.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100),
			Price:    types.Price(50000),
		})

		// Проверяем цену ликвидации (default leverage = 2)
		us := tt.state.GetUserState(userID)
		pos := us.Positions["BTCUSD"]

		// При 2x плече ликвидация должна быть около 48750
		expectedLiquidation := int64(48750)
		actualLiquidation := int64(pos.LiquidationPrice)
		diff := actualLiquidation - expectedLiquidation
		if diff < -100 || diff > 100 {
			t.Errorf("LiquidationPrice expected around %d, got %d", expectedLiquidation, actualLiquidation)
		}
	})
}

// TestCancelOrder тестирует отмену ордера
func (tt *TradingTest) TestCancelOrder() {
	tt.t.Run("CancelOrder", func(t *testing.T) {
		tt.spot.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 1000000)

		// Создаём лимитный ордер
		result, _ := tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000),
		})

		orderID := result.Order.ID

		// Проверяем что ордер в статусе NEW
		if result.Order.Status != constants.ORDER_STATUS_NEW {
			t.Errorf("Expected NEW status before cancel, got %d", result.Order.Status)
		}

		// Отменяем ордер
		err := tt.spot.CancelOrder(orderID, userID)
		if err != nil {
			t.Fatalf("CancelOrder failed: %v", err)
		}

		// Проверяем что баланс разблокирован
		us := tt.state.GetUserState(userID)
		usdtBal := us.Balances["USDT"]
		if usdtBal.Buckets[types.BUCKET_LOCKED] != 0 {
			t.Errorf("Expected LOCKED=0 after cancel, got %d", usdtBal.Buckets[types.BUCKET_LOCKED])
		}
		if usdtBal.Buckets[types.BUCKET_AVAILABLE] != 1000000 {
			t.Errorf("Expected AVAILABLE=1000000 after cancel, got %d", usdtBal.Buckets[types.BUCKET_AVAILABLE])
		}
	})
}

// TestOrderBookDepth тестирует получение глубины книги
func (tt *TradingTest) TestOrderBookDepth() {
	tt.t.Run("OrderBookDepth", func(t *testing.T) {
		tt.spot.Reset()
		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 10000000)

		// Создаём несколько уровней
		for i := 0; i < 5; i++ {
			// Buyers need USDT
			tt.SetupUser(types.UserID(i+1), "USDT", 10000000)

			_, _ = tt.spot.PlaceOrder(&types.OrderInput{
				UserID:   types.UserID(i + 1),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity((i + 1) * 10),
				Price:    types.Price(50000 - int64(i)*100),
			})

			// Sellers need BTC
			tt.SetupUser(types.UserID(i+10), "BTC", 1000)

			_, _ = tt.spot.PlaceOrder(&types.OrderInput{
				UserID:   types.UserID(i + 10),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_SELL,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity((i + 1) * 10),
				Price:    types.Price(50100 + int64(i)*100),
			})
		}

		// Проверяем глубину
		bids, asks := tt.spot.GetOrderBook("BTCUSDT", 10)

		if len(bids) != 10 {
			t.Errorf("Expected 10 bid values (5 levels * 2), got %d", len(bids))
		}

		if len(asks) != 10 {
			t.Errorf("Expected 10 ask values (5 levels * 2), got %d", len(asks))
		}

		// Проверяем что bid цены убывают
		for i := 0; i < len(bids)-2; i += 2 {
			if bids[i] < bids[i+2] {
				t.Errorf("Bid prices should be descending: %d < %d", bids[i], bids[i+2])
			}
		}

		// Проверяем что ask цены возрастают
		for i := 0; i < len(asks)-2; i += 2 {
			if asks[i] > asks[i+2] {
				t.Errorf("Ask prices should be ascending: %d > %d", asks[i], asks[i+2])
			}
		}
	})
}

// TestAllTradingScenarios запускает все тесты торговли
func TestAllTradingScenarios(t *testing.T) {
	tt := NewTradingTest(t)

	tt.TestSPOTBuyLimitOrder()
	tt.TestSPOTSellLimitOrder()
	tt.TestSPOTMatchOrders()
	tt.TestSPOTFOKPartialFill()
	tt.TestSPOTIOCPartialFill()
	tt.TestCancelOrder()
	tt.TestOrderBookDepth()
}

// Тест производительности полного торгового потока
func BenchmarkFullTradingFlow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		tt := NewTradingTest(nil)
		tt.t = nil

		userID := types.UserID(1)
		tt.SetupUser(userID, "USDT", 1000000)

		b.StartTimer()

		// Размещаем ордер
		_, _ = tt.spot.PlaceOrder(&types.OrderInput{
			UserID:   userID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000),
		})

		// Получаем книгу
		_, _ = tt.spot.GetOrderBook("BTCUSDT", 10)

		// Отменяем
		orders := tt.orderStore.GetUserOrders(userID)
		if len(orders) > 0 {
			tt.spot.CancelOrder(orders[0], userID)
		}
	}
}
