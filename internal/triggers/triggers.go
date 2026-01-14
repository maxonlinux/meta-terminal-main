package triggers

import (
	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// ActivatedCallback вызывается при активации триггера.
type ActivatedCallback func(o *types.Order)

// Monitor мониторит условные ордера.
type Monitor struct {
	bySymbol map[string]map[types.OrderID]*types.Trigger
	onActive ActivatedCallback
}

// New создаёт монитор.
func New(cb ActivatedCallback) *Monitor {
	return &Monitor{
		bySymbol: make(map[string]map[types.OrderID]*types.Trigger),
		onActive: cb,
	}
}

// Add добавляет условный ордер.
func (m *Monitor) Add(o *types.Order) {
	if o.TriggerPrice == 0 {
		return
	}

	if m.bySymbol[o.Symbol] == nil {
		m.bySymbol[o.Symbol] = make(map[types.OrderID]*types.Trigger)
	}

	m.bySymbol[o.Symbol][o.ID] = &types.Trigger{
		Order:        o,
		Symbol:       o.Symbol,
		TriggerPrice: o.TriggerPrice,
		Side:         o.Side,
		IsActive:     true,
	}

	o.Status = constants.ORDER_STATUS_UNTRIGGERED
	o.IsConditional = true
}

// Remove удаляет триггер.
func (m *Monitor) Remove(orderID types.OrderID) {
	for _, members := range m.bySymbol {
		delete(members, orderID)
	}
}

// OnPriceTick проверяет триггеры символа.
func (m *Monitor) OnPriceTick(symbol string, price int64) {
	members := m.bySymbol[symbol]
	if members == nil {
		return
	}

	for _, t := range members {
		if !t.IsActive {
			continue
		}

		var activate bool
		if t.Side == constants.ORDER_SIDE_BUY {
			activate = price <= int64(t.TriggerPrice)
		} else {
			activate = price >= int64(t.TriggerPrice)
		}

		if activate {
			t.IsActive = false
			delete(members, t.Order.ID)

			t.Order.Status = constants.ORDER_STATUS_TRIGGERED
			t.Order.IsConditional = false
			t.Order.TriggerPrice = 0
			t.Order.UpdatedAt = types.NowNano()

			if m.onActive != nil {
				m.onActive(t.Order)
			}
		}
	}
}

// Count возвращает количество активных триггеров.
func (m *Monitor) Count() int {
	var n int
	for _, members := range m.bySymbol {
		for _, t := range members {
			if t.IsActive {
				n++
			}
		}
	}
	return n
}
