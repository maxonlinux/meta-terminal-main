package oms

import (
	"errors"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type Service struct {
	mu          sync.RWMutex
	all         map[types.OrderID]*types.Order
	reduceonly  *ReduceOnlyIndex
	conditional *ConditionalIndex
	pebble      *persistence.PebbleKV
}

func (s *Service) removeReduceOnly(order *types.Order) {
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}
}

func (s *Service) removeConditional(order *types.Order) {
	if order.IsConditional {
		s.conditional.Remove(order)
	}
}

func (s *Service) indexOrder(order *types.Order) {
	if order.ReduceOnly {
		s.reduceonly.Add(order)
	}
	if order.IsConditional {
		s.conditional.Add(order)
	}
}

func newOrder(
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
	now uint64,
) *types.Order {
	isConditional := !triggerPrice.IsZero()
	status := int8(constants.ORDER_STATUS_NEW)
	if isConditional {
		status = constants.ORDER_STATUS_UNTRIGGERED
	}

	return &types.Order{
		ID:             types.OrderID(snowflake.Next()),
		UserID:         userID,
		Symbol:         symbol,
		Category:       category,
		Side:           side,
		Type:           otype,
		TIF:            tif,
		Status:         status,
		Price:          price,
		Quantity:       quantity,
		Filled:         math.Zero,
		TriggerPrice:   triggerPrice,
		ReduceOnly:     reduceOnly,
		CloseOnTrigger: closeOnTrigger,
		StopOrderType:  stopOrderType,
		IsConditional:  isConditional,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func NewService(pkv *persistence.PebbleKV) *Service {
	s := &Service{
		all:         make(map[types.OrderID]*types.Order),
		reduceonly:  NewReduceOnlyIndex(),
		conditional: NewConditionalIndex(),
		pebble:      pkv,
	}

	if pkv != nil {
		pkv.RangeOrders(func(order *types.Order) bool {
			s.all[order.ID] = order
			s.indexOrder(order)
			return true
		})
	}

	return s
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

	now := utils.NowNano()
	order := newOrder(
		userID,
		symbol,
		category,
		side,
		otype,
		tif,
		price,
		quantity,
		triggerPrice,
		reduceOnly,
		closeOnTrigger,
		stopOrderType,
		now,
	)

	s.all[order.ID] = order
	s.indexOrder(order)

	if s.pebble != nil {
		s.pebble.PutOrder(order)
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

	remaining := math.Sub(order.Quantity, order.Filled)
	delta := math.Sub(remaining, newQty)

	order.Quantity = newQty
	order.UpdatedAt = utils.NowNano()

	if order.ReduceOnly {
		s.reduceonly.AdjustExposure(order, math.Sub(math.Zero, delta))
	}

	if s.pebble != nil {
		s.pebble.PutOrder(order)
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

	if order.IsConditional {
		order.Status = constants.ORDER_STATUS_DEACTIVATED
	} else if !order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	now := utils.NowNano()
	order.UpdatedAt = now

	s.removeReduceOnly(order)
	s.removeConditional(order)

	if s.pebble != nil {
		s.pebble.PutOrder(order)
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

	s.reduceonly.AdjustExposure(order, math.Sub(math.Zero, qty))

	order.Filled = math.Add(order.Filled, qty)
	order.UpdatedAt = utils.NowNano()

	if math.Cmp(order.Filled, order.Quantity) != 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		if s.pebble != nil {
			s.pebble.PutOrder(order)
		}
		return nil
	}

	order.Status = constants.ORDER_STATUS_FILLED

	s.removeReduceOnly(order)

	if s.pebble != nil {
		s.pebble.PutOrder(order)
	}

	return nil
}

func (s *Service) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reduceonly.OnPositionReduce(userID, symbol, positionSize)
}

func (s *Service) OnPriceTick(symbol string, price types.Price, callback func(*types.Order)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conditional.CheckTriggers(symbol, price, callback)
}

func (s *Service) Iterate(fn func(*types.Order) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, order := range s.all {
		if !fn(order) {
			break
		}
	}
}
