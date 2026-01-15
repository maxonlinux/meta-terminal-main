package oms

import (
	"container/heap"
	"errors"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type Service struct {
	mu          sync.RWMutex
	all         map[types.OrderID]*types.Order
	reduceonly  *ReduceOnlyIndex
	conditional *ConditionalIndex
}

func NewService() *Service {
	return &Service{
		all:         make(map[types.OrderID]*types.Order),
		reduceonly:  NewReduceOnlyManager(),
		conditional: NewConditionalIndex(),
	}
}

func (s *Service) Create(
	userID types.UserID,
	symbol string,
	category int8,
	side int8,
	otype int8,
	tif int8,
	price types.Price,
	quantity types.Quantity,
	triggerPrice types.Price,
	reduceOnly bool,
	closeOnTrigger bool,
	stopOrderType int8,
) *types.Order {
	s.mu.Lock()
	defer s.mu.Unlock()

	order := &types.Order{
		ID:             types.OrderID(snowflake.Next()),
		UserID:         userID,
		Symbol:         symbol,
		Category:       category,
		Side:           side,
		Type:           otype,
		TIF:            tif,
		Status:         constants.ORDER_STATUS_NEW,
		Price:          price,
		Quantity:       quantity,
		Filled:         math.Zero,
		TriggerPrice:   triggerPrice,
		ReduceOnly:     reduceOnly,
		CloseOnTrigger: closeOnTrigger,
		StopOrderType:  stopOrderType,
		IsConditional:  !triggerPrice.IsZero(),
		CreatedAt:      utils.NowNano(),
		UpdatedAt:      utils.NowNano(),
	}

	s.all[order.ID] = order

	if order.ReduceOnly {
		s.reduceonly.Add(order)
	}

	if order.IsConditional {
		s.conditional.Add(order)
	}

	return order
}

func (s *Service) Get(id types.OrderID) (*types.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.all[id]
	return order, ok
}

func (s *Service) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.all)
}

func (s *Service) Amend(id types.OrderID, newQty types.Quantity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	if math.Cmp(newQty, order.Quantity) >= 0 {
		return errors.New("new quantity must be less than current")
	}

	delta := math.Sub(order.Quantity, newQty)
	order.Quantity = newQty
	order.UpdatedAt = utils.NowNano()

	if order.ReduceOnly {
		exposure := s.reduceonly.exposure[order.Symbol][order.UserID]
		s.reduceonly.exposure[order.Symbol][order.UserID] = math.Sub(exposure, delta)
	}

	return nil
}

func (s *Service) Cancel(id types.OrderID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		return errors.New("only active orders can be canceled")
	}

	if !order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	order.ClosedAt = utils.NowNano()
	order.UpdatedAt = utils.NowNano()

	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}

	if order.IsConditional {
		s.conditional.Remove(order)
	}

	return nil
}

func (s *Service) Fill(id types.OrderID, qty types.Quantity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	order.Filled = math.Add(order.Filled, qty)
	order.UpdatedAt = utils.NowNano()

	if math.Cmp(order.Filled, order.Quantity) == 0 {
		order.Status = constants.ORDER_STATUS_FILLED
		order.ClosedAt = utils.NowNano()

		if order.IsConditional {
			s.conditional.Remove(order)
		}

		if order.ReduceOnly {
			exposure := s.reduceonly.exposure[order.Symbol][order.UserID]
			s.reduceonly.exposure[order.Symbol][order.UserID] = math.Sub(exposure, qty)
			s.reduceonly.Remove(order)
		}
	} else {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED

		if order.ReduceOnly {
			exposure := s.reduceonly.exposure[order.Symbol][order.UserID]
			s.reduceonly.exposure[order.Symbol][order.UserID] = math.Sub(exposure, qty)
		}
	}

	return nil
}

func (s *Service) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var h *orderHeap
	if positionSize.Sign() > 0 {
		h = s.reduceonly.sellHeaps[symbol]
	} else {
		h = s.reduceonly.buyHeaps[symbol]
	}

	if h == nil || h.Len() == 0 {
		return
	}

	total := s.reduceonly.exposure[symbol][userID]
	if math.Cmp(total, positionSize) <= 0 {
		return
	}

	excess := math.Sub(total, positionSize)

	for excess.Sign() > 0 {
		o := h.Peek()
		if o == nil {
			continue
		}

		if s.reduceonly.deleted[&o.ID] {
			heap.Pop(h)
			delete(s.reduceonly.deleted, &o.ID)
			continue
		}

		remaining := math.Sub(o.Quantity, o.Filled)
		if math.Cmp(remaining, excess) <= 0 {
			s.reduceonly.Remove(o)
			o.Status = constants.ORDER_STATUS_CANCELED
			o.ClosedAt = utils.NowNano()
			excess = math.Sub(excess, remaining)
			continue
		}

		o.Quantity = math.Sub(o.Quantity, excess)
		exposure := s.reduceonly.exposure[symbol][userID]
		s.reduceonly.exposure[symbol][userID] = math.Sub(exposure, excess)
		excess = math.Zero
	}
}

func (s *Service) OnPriceTick(symbol string, price types.Price, callback func(*types.Order)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if h := s.conditional.buyTriggers[symbol]; h != nil {
		for h.Len() > 0 {
			top := h.Peek()
			if top == nil {
				continue
			}

			if s.conditional.deleted[&top.ID] {
				heap.Pop(h)
				delete(s.conditional.deleted, &top.ID)
				continue
			}

			if math.Cmp(top.TriggerPrice, price) >= 0 {
				heap.Pop(h)
				top.Status = constants.ORDER_STATUS_TRIGGERED
				top.UpdatedAt = utils.NowNano()
				callback(top)
				continue
			}

			break
		}
	}

	if h := s.conditional.sellTriggers[symbol]; h != nil {
		for h.Len() > 0 {
			top := h.Peek()
			if top == nil {
				continue
			}

			if s.conditional.deleted[&top.ID] {
				heap.Pop(h)
				delete(s.conditional.deleted, &top.ID)
				continue
			}

			if math.Cmp(top.TriggerPrice, price) <= 0 {
				heap.Pop(h)
				top.Status = constants.ORDER_STATUS_TRIGGERED
				top.UpdatedAt = utils.NowNano()
				callback(top)
				continue
			}

			break
		}
	}
}
