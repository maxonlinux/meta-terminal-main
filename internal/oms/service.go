package oms

import (
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

func (s *Service) removeReduceOnly(order *types.Order) {
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
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

func NewService() *Service {
	return &Service{
		all:         make(map[types.OrderID]*types.Order),
		reduceonly:  NewReduceOnlyIndex(),
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

	// Single timestamp for create/update keeps state consistent.
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

	// Store in main order map.
	s.all[order.ID] = order
	s.indexOrder(order)

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

// Orders returns a copy of all orders for engine state export.
func (s *Service) Orders() []types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	orders := make([]types.Order, 0, len(s.all))
	for _, order := range s.all {
		orders = append(orders, *order)
	}
	return orders
}

func (s *Service) Amend(id types.OrderID, newQty types.Quantity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	// Validate new quantity is smaller than current
	if math.Cmp(newQty, order.Quantity) >= 0 {
		return errors.New("new quantity must be less than current")
	}

	// For reduceOnly orders, delta must be calculated based on REMAINING quantity
	// (Quantity - Filled), not the original Quantity. This ensures correct exposure
	// recalculation after partial fills.
	// Example: qty=10, filled=3, remaining=7. Amend to 5.
	// delta = remaining - newQty = 7 - 5 = 2
	remaining := math.Sub(order.Quantity, order.Filled)
	delta := math.Sub(remaining, newQty)

	order.Quantity = newQty
	order.UpdatedAt = utils.NowNano()

	// Only update reduceOnly exposure if this is a reduceOnly order
	if order.ReduceOnly {
		s.reduceonly.AdjustExposure(order, math.Sub(math.Zero, delta))
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

	// Validate order is in cancellable state
	if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		return errors.New("only active orders can be canceled")
	}

	// Partial fills keep history as partially filled + canceled.
	if !order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	// Single timestamp keeps order history consistent.
	now := utils.NowNano()
	order.UpdatedAt = now

	// Remove from reduceonly index if registered.
	s.removeReduceOnly(order)

	// Conditional orders are removed lazily during trigger checks.

	// Order remains in the store for audit/history.

	return nil
}

func (s *Service) Fill(id types.OrderID, qty types.Quantity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	// Reduce-only exposure is reduced by executed quantity.
	s.reduceonly.AdjustExposure(order, math.Sub(math.Zero, qty))

	// Update fill before status to keep state consistent.
	order.Filled = math.Add(order.Filled, qty)
	order.UpdatedAt = utils.NowNano()

	if math.Cmp(order.Filled, order.Quantity) != 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		return nil
	}

	order.Status = constants.ORDER_STATUS_FILLED

	// Remove from reduceonly index if this was a reduceOnly order.
	s.removeReduceOnly(order)

	// For conditional orders, cleanup happens lazily via order.Status check in CheckTriggers()

	return nil
}

func (s *Service) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delegate to the sharded reduceonly index
	s.reduceonly.OnPositionReduce(symbol, positionSize, userID)
}

func (s *Service) OnPriceTick(symbol string, price types.Price, callback func(*types.Order)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use the sharded ConditionalIndex to check triggers with callback
	// CheckTriggers handles Status and UpdatedAt internally
	s.conditional.CheckTriggers(symbol, price, callback)
}

// LoadOrders replaces OMS state with the provided orders.
func (s *Service) LoadOrders(orders []types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.all = make(map[types.OrderID]*types.Order, len(orders))
	s.reduceonly = NewReduceOnlyIndex()
	s.conditional = NewConditionalIndex()

	for i := range orders {
		stored := orders[i]
		order := &stored
		s.all[order.ID] = order
		s.indexOrder(order)
	}
}
