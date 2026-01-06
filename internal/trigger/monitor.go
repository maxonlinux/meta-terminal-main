package trigger

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/memory"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Monitor struct {
	state      *state.State
	orderStore *memory.OrderStore
	stopOrders map[types.SymbolID]*state.Heap
	tpOrders   map[types.SymbolID]*state.Heap
	slOrders   map[types.SymbolID]*state.Heap
}

func NewMonitor(s *state.State, orderStore *memory.OrderStore) *Monitor {
	return &Monitor{
		state:      s,
		orderStore: orderStore,
		stopOrders: make(map[types.SymbolID]*state.Heap),
		tpOrders:   make(map[types.SymbolID]*state.Heap),
		slOrders:   make(map[types.SymbolID]*state.Heap),
	}
}

func (m *Monitor) AddOrder(order *types.Order) {
	var heap *state.Heap
	var heapMap map[types.SymbolID]*state.Heap

	switch order.StopOrderType {
	case constants.STOP_ORDER_TYPE_STOP:
		heapMap = m.stopOrders
	case constants.STOP_ORDER_TYPE_TP:
		heapMap = m.tpOrders
	case constants.STOP_ORDER_TYPE_SL:
		heapMap = m.slOrders
	default:
		return
	}

	if _, ok := heapMap[order.Symbol]; !ok {
		heapMap[order.Symbol] = state.NewHeap(order.Side == constants.ORDER_SIDE_SELL, func(o *types.Order) types.Price {
			return o.TriggerPrice
		}, m.orderStore)
	}

	heap = heapMap[order.Symbol]
	heap.Push(order.ID)

	if order.Side == constants.ORDER_SIDE_BUY {
		m.state.GetSymbolState(order.Symbol).BuyTriggers = heap
	} else {
		m.state.GetSymbolState(order.Symbol).SellTriggers = heap
	}
}

func (m *Monitor) Check(symbol types.SymbolID, currentPrice types.Price) []*types.Order {
	var triggered []*types.Order

	ss := m.state.GetSymbolState(symbol)

	if ss.BuyTriggers != nil {
		for ss.BuyTriggers.Len() > 0 {
			oid := ss.BuyTriggers.Peek()
			if oid == 0 {
				break
			}
			order := m.orderStore.Get(oid)
			if order == nil {
				ss.BuyTriggers.Pop()
				continue
			}
			if order.TriggerPrice <= currentPrice {
				ss.BuyTriggers.Pop()
				triggered = append(triggered, order)
			} else {
				break
			}
		}
	}

	if ss.SellTriggers != nil {
		for ss.SellTriggers.Len() > 0 {
			oid := ss.SellTriggers.Peek()
			if oid == 0 {
				break
			}
			order := m.orderStore.Get(oid)
			if order == nil {
				ss.SellTriggers.Pop()
				continue
			}
			if order.TriggerPrice >= currentPrice {
				ss.SellTriggers.Pop()
				triggered = append(triggered, order)
			} else {
				break
			}
		}
	}

	return triggered
}

func (m *Monitor) OnTrigger(order *types.Order) ([]*types.Order, error) {
	order.Status = constants.ORDER_STATUS_TRIGGERED

	if order.CloseOnTrigger {
		return nil, nil
	}

	triggeredOrder := &types.Order{
		ID:             m.state.NextOrderID,
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Side:           order.Side,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Price:          order.Price,
		Quantity:       order.Quantity - order.Filled,
		Status:         constants.ORDER_STATUS_NEW,
		TriggerPrice:   0,
		StopOrderType:  constants.STOP_ORDER_TYPE_NORMAL,
		ReduceOnly:     order.ReduceOnly,
		CloseOnTrigger: false,
	}

	m.state.NextOrderID++

	return []*types.Order{triggeredOrder}, nil
}

func (m *Monitor) RemoveOrder(orderID types.OrderID, symbol types.SymbolID) {
	ss := m.state.GetSymbolState(symbol)

	if ss.BuyTriggers != nil {
		ss.BuyTriggers.Remove(orderID)
	}
	if ss.SellTriggers != nil {
		ss.SellTriggers.Remove(orderID)
	}
}
