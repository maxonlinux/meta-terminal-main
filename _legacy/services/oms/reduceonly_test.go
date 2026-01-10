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
		orders:               make(map[types.UserID]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[types.UserID]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
		mu: sync.RWMutex{},
	}

	userID := types.UserID(1)
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
	data = appendUint64(data, uint64(userID))
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(20))
	data = append(data, byte(constants.ORDER_SIDE_BUY))

	svc.handlePositionUpdate(data)

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order1 (SELL) to be CANCELED when position flipped to BUY, got %d", order1.Status)
	}
	if order2.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected order2 (SELL) to be CANCELED when position flipped to BUY, got %d", order2.Status)
	}
}

func TestReduceOnly_PositionDecreased_TrimOrders(t *testing.T) {
	svc := &Service{
		orders:               make(map[types.UserID]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[types.UserID]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
		mu: sync.RWMutex{},
	}

	userID := types.UserID(1)
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
	data = appendUint64(data, uint64(userID))
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(60))
	data = append(data, byte(constants.ORDER_SIDE_SELL))

	svc.handlePositionUpdate(data)

	if order1.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("Expected order1 to remain NEW, got %d", order1.Status)
	}
	if order2.Quantity != types.Quantity(10) {
		t.Errorf("Expected order2 to be trimmed to 10, got %d", order2.Quantity)
	}
}

func TestReduceOnly_PositionZero_CancelAll(t *testing.T) {
	svc := &Service{
		orders:               make(map[types.UserID]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[types.UserID]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
		mu: sync.RWMutex{},
	}

	userID := types.UserID(1)
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
	data = appendUint64(data, uint64(userID))
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

func TestReduceOnly_BatchTrimOrders(t *testing.T) {
	svc := &Service{
		orders:               make(map[types.UserID]map[types.OrderID]*types.Order),
		reduceOnlyCommitment: make(map[types.UserID]map[string]int64),
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
		mu: sync.RWMutex{},
	}

	userID := types.UserID(1)
	symbol := "BTCUSDT"

	svc.orders[userID] = make(map[types.OrderID]*types.Order)

	sizes := []int64{10, 25, 15, 50, 40, 30, 20, 5}

	for i, size := range sizes {
		order := pool.GetOrder()
		order.ID = types.OrderID(i + 1)
		order.UserID = userID
		order.Symbol = symbol
		order.Category = constants.CATEGORY_LINEAR
		order.Side = constants.ORDER_SIDE_SELL
		order.Status = constants.ORDER_STATUS_NEW
		order.Quantity = types.Quantity(size)
		order.Filled = 0
		order.ReduceOnly = true
		svc.orders[userID][order.ID] = order
	}

	totalBefore := int64(0)
	for _, order := range svc.orders[userID] {
		totalBefore += int64(order.Quantity)
	}
	if totalBefore != 195 {
		t.Errorf("Total before should be 195, got %d", totalBefore)
	}

	data := make([]byte, 0, 50)
	data = append(data, 0x02)
	data = appendUint64(data, uint64(userID))
	data = append(data, byte(len(symbol)))
	data = append(data, symbol...)
	data = appendUint64(data, uint64(100))
	data = append(data, byte(constants.ORDER_SIDE_SELL))

	svc.handlePositionUpdate(data)

	totalAfter := int64(0)
	for _, order := range svc.orders[userID] {
		if order.Status == constants.ORDER_STATUS_NEW {
			totalAfter += int64(order.Quantity)
		}
	}

	if totalAfter != 100 {
		t.Errorf("Total remaining should be 100, got %d", totalAfter)
	}
}

func appendUint64(buf []byte, v uint64) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24), byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}
