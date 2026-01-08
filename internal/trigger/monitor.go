package trigger

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/types"
)

type Monitor struct {
	buyHeap  []types.OrderID
	sellHeap []types.OrderID
}

func NewMonitor() *Monitor {
	return &Monitor{
		buyHeap:  make([]types.OrderID, 0),
		sellHeap: make([]types.OrderID, 0),
	}
}

func (m *Monitor) AddOrder(order *types.Order) {
	if order.Side == constants.ORDER_SIDE_BUY {
		m.buyHeap = append(m.buyHeap, order.ID)
	} else {
		m.sellHeap = append(m.sellHeap, order.ID)
	}
}

func (m *Monitor) RemoveOrder(orderID types.OrderID) {
	for i, id := range m.buyHeap {
		if id == orderID {
			m.buyHeap = append(m.buyHeap[:i], m.buyHeap[i+1:]...)
			return
		}
	}
	for i, id := range m.sellHeap {
		if id == orderID {
			m.sellHeap = append(m.sellHeap[:i], m.sellHeap[i+1:]...)
			return
		}
	}
}

func (m *Monitor) Check(currentPrice types.Price) []types.OrderID {
	var triggered []types.OrderID

	for _, orderID := range m.buyHeap {
		triggered = append(triggered, orderID)
	}
	m.buyHeap = m.buyHeap[:0]

	for _, orderID := range m.sellHeap {
		triggered = append(triggered, orderID)
	}
	m.sellHeap = m.sellHeap[:0]

	return triggered
}

func (m *Monitor) OnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED
}
