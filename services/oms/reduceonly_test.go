package oms

import (
	"sync"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestReduceOnly_FlipPosition_CancelAll(t *testing.T) {
	svc := &Service{
		orders:               make(map[uint64]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[uint64]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
	}

	userID := uint64(1)
	symbol := "BTCUSDT"

	svc.orders[userID] = make(map[types.OrderID]*types.Order)

	order1 := pool.GetOrder()
	order1.ID = types.OrderID(1)
	order1.UserID = userID
	order1.Symbol = symbol
	order1.Category = constants.CATEGORY_LINEAR
	order1.Side = constants.ORDER_SIDE_SELL
	order1.Status = constants.ORDER_STATUS_NEW
	order1.Quantity = types.Quantity(50)
	order1.Filled = 0
	order1.ReduceOnly = true
	svc.orders[userID][order1.ID] = order1

	order2 := pool.GetOrder()
	order2.ID = types.OrderID(2)
	order2.UserID = userID
	order2.Symbol = symbol
	order2.Category = constants.CATEGORY_LINEAR
	order2.Side = constants.ORDER_SIDE_SELL
	order2.Status = constants.ORDER_STATUS_NEW
	order2.Quantity = types.Quantity(30)
	order2.Filled = 0
	order2.ReduceOnly = true
	svc.orders[userID][order2.ID] = order2

	order3 := pool.GetOrder()
	order3.ID = types.OrderID(3)
	order3.UserID = userID
	order3.Symbol = symbol
	order3.Category = constants.CATEGORY_LINEAR
	order3.Side = constants.ORDER_SIDE_BUY
	order3.Status = constants.ORDER_STATUS_NEW
	order3.Quantity = types.Quantity(10)
	order3.Filled = 0
	order3.ReduceOnly = true
	svc.orders[userID][order3.ID] = order3

	data := make([]byte, 0, 50)
	data = append(data, 0x02)
	data = appendUint64(data, userID)
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(20))
	data = append(data, byte(constants.ORDER_SIDE_BUY))

	svc.handlePositionUpdate(data)

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order1 to be CANCELED, got %d", order1.Status)
	}
	if order2.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order2 to be CANCELED, got %d", order2.Status)
	}
	if order3.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order3 to be CANCELED (opposite side), got %d", order3.Status)
	}
}

func TestReduceOnly_PositionDecreased_TrimOrders(t *testing.T) {
	svc := &Service{
		orders:               make(map[uint64]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[uint64]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
	}

	userID := uint64(1)
	symbol := "BTCUSDT"

	svc.orders[userID] = make(map[types.OrderID]*types.Order)

	order1 := pool.GetOrder()
	order1.ID = types.OrderID(1)
	order1.UserID = userID
	order1.Symbol = symbol
	order1.Category = constants.CATEGORY_LINEAR
	order1.Side = constants.ORDER_SIDE_SELL
	order1.Status = constants.ORDER_STATUS_NEW
	order1.Quantity = types.Quantity(50)
	order1.Filled = 0
	order1.ReduceOnly = true
	svc.orders[userID][order1.ID] = order1

	order2 := pool.GetOrder()
	order2.ID = types.OrderID(2)
	order2.UserID = userID
	order2.Symbol = symbol
	order2.Category = constants.CATEGORY_LINEAR
	order2.Side = constants.ORDER_SIDE_SELL
	order2.Status = constants.ORDER_STATUS_NEW
	order2.Quantity = types.Quantity(30)
	order2.Filled = 0
	order2.ReduceOnly = true
	svc.orders[userID][order2.ID] = order2

	data := make([]byte, 0, 50)
	data = append(data, 0x02)
	data = appendUint64(data, userID)
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(60))
	data = append(data, byte(constants.ORDER_SIDE_SELL))

	svc.handlePositionUpdate(data)

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order1 to be CANCELED (trimmed beyond position), got %d", order1.Status)
	}
	if order2.Quantity != types.Quantity(40) {
		t.Errorf("Expected order2 to be trimmed to 40, got %d", order2.Quantity)
	}
}

func TestReduceOnly_PositionZero_CancelAll(t *testing.T) {
	svc := &Service{
		orders:               make(map[uint64]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[uint64]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
	}

	userID := uint64(1)
	symbol := "BTCUSDT"

	svc.orders[userID] = make(map[types.OrderID]*types.Order)

	order1 := pool.GetOrder()
	order1.ID = types.OrderID(1)
	order1.UserID = userID
	order1.Symbol = symbol
	order1.Category = constants.CATEGORY_LINEAR
	order1.Side = constants.ORDER_SIDE_SELL
	order1.Status = constants.ORDER_STATUS_NEW
	order1.Quantity = types.Quantity(50)
	order1.Filled = 0
	order1.ReduceOnly = true
	svc.orders[userID][order1.ID] = order1

	order2 := pool.GetOrder()
	order2.ID = types.OrderID(2)
	order2.UserID = userID
	order2.Symbol = symbol
	order2.Category = constants.CATEGORY_LINEAR
	order2.Side = constants.ORDER_SIDE_SELL
	order2.Status = constants.ORDER_STATUS_NEW
	order2.Quantity = types.Quantity(30)
	order2.Filled = 0
	order2.ReduceOnly = true
	svc.orders[userID][order2.ID] = order2

	data := make([]byte, 0, 50)
	data = append(data, 0x02)
	data = appendUint64(data, userID)
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(0))
	data = append(data, byte(constants.ORDER_SIDE_SELL))

	svc.handlePositionUpdate(data)

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order1 to be CANCELED, got %d", order1.Status)
	}
	if order2.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order2 to be CANCELED, got %d", order2.Status)
	}
}

func TestReduceOnly_RaceCondition_ConcurrentUpdates(t *testing.T) {
	svc := &Service{
		orders:               make(map[uint64]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[uint64]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
		mu: sync.RWMutex{},
	}

	userID := uint64(1)
	symbol := "BTCUSDT"

	svc.orders[userID] = make(map[types.OrderID]*types.Order)

	for i := 0; i < 100; i++ {
		order := pool.GetOrder()
		order.ID = types.OrderID(i + 1)
		order.UserID = userID
		order.Symbol = symbol
		order.Category = constants.CATEGORY_LINEAR
		order.Side = constants.ORDER_SIDE_SELL
		order.Status = constants.ORDER_STATUS_NEW
		order.Quantity = types.Quantity(10)
		order.Filled = 0
		order.ReduceOnly = true
		svc.orders[userID][order.ID] = order
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			data := make([]byte, 0, 50)
			data = append(data, 0x02)
			data = appendUint64(data, userID)
			data = append(data, byte(len(symbol)))
			data = append(data, symbol...)
			newSize := uint64(50 + iteration*10)
			data = appendUint64(data, newSize)
			data = append(data, byte(constants.ORDER_SIDE_SELL))

			svc.mu.Lock()
			svc.handlePositionUpdate(data)
			svc.mu.Unlock()
		}(i)
	}

	wg.Wait()

	select {
	case err := <-errors:
		t.Errorf("Race condition detected: %v", err)
	default:
	}

	total := int64(0)
	svc.mu.RLock()
	for _, order := range svc.orders[userID] {
		total += int64(order.Quantity - order.Filled)
	}
	svc.mu.RUnlock()

	if total > 140 {
		t.Errorf("Race condition: total reduceOnly commitment %d exceeds expected max 140", total)
	}
}

func TestReduceOnly_OppositeSideOrders_NotAffected(t *testing.T) {
	svc := &Service{
		orders:               make(map[uint64]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[uint64]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
	}

	userID := uint64(1)
	symbol := "BTCUSDT"

	svc.orders[userID] = make(map[types.OrderID]*types.Order)

	sellOrder := pool.GetOrder()
	sellOrder.ID = types.OrderID(1)
	sellOrder.UserID = userID
	sellOrder.Symbol = symbol
	sellOrder.Category = constants.CATEGORY_LINEAR
	sellOrder.Side = constants.ORDER_SIDE_SELL
	sellOrder.Status = constants.ORDER_STATUS_NEW
	sellOrder.Quantity = types.Quantity(50)
	sellOrder.Filled = 0
	sellOrder.ReduceOnly = true
	svc.orders[userID][sellOrder.ID] = sellOrder

	buyOrder := pool.GetOrder()
	buyOrder.ID = types.OrderID(2)
	buyOrder.UserID = userID
	buyOrder.Symbol = symbol
	buyOrder.Category = constants.CATEGORY_LINEAR
	buyOrder.Side = constants.ORDER_SIDE_BUY
	buyOrder.Status = constants.ORDER_STATUS_NEW
	buyOrder.Quantity = types.Quantity(10)
	buyOrder.Filled = 0
	buyOrder.ReduceOnly = true
	svc.orders[userID][buyOrder.ID] = buyOrder

	data := make([]byte, 0, 50)
	data = append(data, 0x02)
	data = appendUint64(data, userID)
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(30))
	data = append(data, byte(constants.ORDER_SIDE_BUY))

	svc.handlePositionUpdate(data)

	if sellOrder.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected SELL reduceOnly to be CANCELED when position became BUY, got %d", sellOrder.Status)
	}
	if buyOrder.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected BUY reduceOnly to be CANCELED when position side matches (flip), got %d", buyOrder.Status)
	}
}

func appendUint64(buf []byte, v uint64) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24), byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}
