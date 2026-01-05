package trigger

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Monitor struct {
	state      *state.State
	stopOrders map[types.SymbolID]*state.Heap
	tpOrders   map[types.SymbolID]*state.Heap
	slOrders   map[types.SymbolID]*state.Heap
}

func NewMonitor(s *state.State) *Monitor {
	return &Monitor{
		state:      s,
		stopOrders: make(map[types.SymbolID]*state.Heap),
		tpOrders:   make(map[types.SymbolID]*state.Heap),
		slOrders:   make(map[types.SymbolID]*state.Heap),
	}
}

func (m *Monitor) AddOrder(order *types.Order) {
	ss := m.state.GetSymbolState(order.Symbol)

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
		})
	}

	heap = heapMap[order.Symbol]
	ss.OrderMap[order.ID] = order
	heap.Push(ss.OrderMap, order.ID)

	if order.Side == constants.ORDER_SIDE_BUY {
		ss.BuyTriggers = heap
	} else {
		ss.SellTriggers = heap
	}
}

func (m *Monitor) Check(symbol types.SymbolID, currentPrice types.Price) []*types.Order {
	var triggered []*types.Order

	ss := m.state.GetSymbolState(symbol)

	if ss.BuyTriggers != nil {
		for ss.BuyTriggers.Len() > 0 {
			oid := ss.BuyTriggers.Peek(ss.OrderMap)
			if oid == 0 {
				break
			}
			order := ss.OrderMap[oid]
			if order.TriggerPrice <= currentPrice {
				ss.BuyTriggers.Pop(ss.OrderMap)
				triggered = append(triggered, order)
			} else {
				break
			}
		}
	}

	if ss.SellTriggers != nil {
		for ss.SellTriggers.Len() > 0 {
			oid := ss.SellTriggers.Peek(ss.OrderMap)
			if oid == 0 {
				break
			}
			order := ss.OrderMap[oid]
			if order.TriggerPrice >= currentPrice {
				ss.SellTriggers.Pop(ss.OrderMap)
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
		ss.BuyTriggers.Remove(orderID, ss.OrderMap)
	}
	if ss.SellTriggers != nil {
		ss.SellTriggers.Remove(orderID, ss.OrderMap)
	}
}
