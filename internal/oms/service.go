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

	// Single timestamp call for CreatedAt/UpdatedAt (optimization)
	now := utils.NowNano()

	// Determine initial status based on order type
	// Conditional orders start as UNTRIGGERED
	isConditional := !triggerPrice.IsZero()
	var initialStatus int8
	if isConditional {
		initialStatus = constants.ORDER_STATUS_UNTRIGGERED
	} else {
		initialStatus = constants.ORDER_STATUS_NEW
	}

	// Create order with all fields
	// Note: math.Zero is safe to share because fixed.Fixed is immutable
	order := &types.Order{
		ID:             types.OrderID(snowflake.Next()),
		UserID:         userID,
		Symbol:         symbol,
		Category:       category,
		Side:           side,
		Type:           otype,
		TIF:            tif,
		Status:         initialStatus,
		Price:          price,
		Quantity:       quantity,
		Filled:         math.Zero,
		TriggerPrice:   triggerPrice,
		ReduceOnly:     reduceOnly,
		CloseOnTrigger: closeOnTrigger,
		StopOrderType:  stopOrderType,
		// IsConditional: true if triggerPrice is set (non-zero)
		IsConditional: isConditional,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Store in main order map
	s.all[order.ID] = order

	// Add to indices if applicable
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
		shardIdx := ShardIndex(order.Symbol)
		shard := s.reduceonly.shards[shardIdx]
		shard.exposure[order.UserID] = math.Sub(shard.exposure[order.UserID], delta)
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

	// Determine cancellation status based on fill state
	// Partial fills get PARTIALLY_FILLED_CANCELED, unfilled get CANCELED
	if !order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	// Single timestamp call for both timestamps (optimization)
	now := utils.NowNano()
	order.ClosedAt = now
	order.UpdatedAt = now

	// Remove from reduceonly index if registered
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}

	// For conditional orders, cleanup happens lazily via order.Status check in CheckTriggers()
	// No explicit removal needed - just set the status above

	// Order remains in all map for history/auditing (not deleted)
	// Only removed from active indices
	return nil
}

func (s *Service) Fill(id types.OrderID, qty types.Quantity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	// Common reduceOnly exposure reduction for both full and partial fills
	if order.ReduceOnly {
		shardIdx := ShardIndex(order.Symbol)
		shard := s.reduceonly.shards[shardIdx]
		shard.exposure[order.UserID] = math.Sub(shard.exposure[order.UserID], qty)
	}

	// Flatten: check partial fill first and return early
	order.Filled = math.Add(order.Filled, qty)
	order.UpdatedAt = utils.NowNano()

	if math.Cmp(order.Filled, order.Quantity) != 0 {
		// Partial fill case
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		return nil
	}

	// Full fill case (no else needed due to early return above)
	order.Status = constants.ORDER_STATUS_FILLED
	order.ClosedAt = utils.NowNano()

	// Remove from reduceonly index if this was a reduceOnly order
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}

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
