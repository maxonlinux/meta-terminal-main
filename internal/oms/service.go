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

	// Single timestamp call for CreatedAt/UpdatedAt (optimization)
	now := utils.NowNano()

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
		Status:         constants.ORDER_STATUS_NEW,
		Price:          price,
		Quantity:       quantity,
		Filled:         math.Zero,
		TriggerPrice:   triggerPrice,
		ReduceOnly:     reduceOnly,
		CloseOnTrigger: closeOnTrigger,
		StopOrderType:  stopOrderType,
		// IsConditional: true if triggerPrice is set (non-zero)
		IsConditional: !triggerPrice.IsZero(),
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
	// Cache map lookup: exposureMap := s.reduceonly.exposure[order.Symbol]
	// This avoids repeated map lookups in tight loops
	if order.ReduceOnly {
		exposureMap := s.reduceonly.exposure[order.Symbol]
		exposure := exposureMap[order.UserID]
		exposureMap[order.UserID] = math.Sub(exposure, delta)
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

	// Remove from indices if registered
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}

	if order.IsConditional {
		s.conditional.Remove(order)
	}

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
	// Cache map lookup once for performance (avoids repeated map access)
	if order.ReduceOnly {
		exposureMap := s.reduceonly.exposure[order.Symbol]
		exposure := exposureMap[order.UserID]
		exposureMap[order.UserID] = math.Sub(exposure, qty)
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

	if order.IsConditional {
		s.conditional.Remove(order)
	}

	// Only remove from reduceOnly index if this was a reduceOnly order
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}

	return nil
}

func (s *Service) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Select heap based on position direction:
	// LONG positions (size > 0) reduce by SELL orders
	// SHORT positions (size < 0) reduce by BUY orders
	var h *orderHeap
	if positionSize.Sign() > 0 {
		h = s.reduceonly.sellHeaps[symbol]
	} else {
		h = s.reduceonly.buyHeaps[symbol]
	}

	// Early exit if no orders to process
	if h == nil || h.Len() == 0 {
		return
	}

	// Get user's current reduceOnly exposure for this symbol
	total := s.reduceonly.exposure[symbol][userID]
	// If exposure doesn't exceed position, nothing to reduce
	if math.Cmp(total, positionSize) <= 0 {
		return
	}

	// Calculate excess exposure that needs to be canceled
	excess := math.Sub(total, positionSize)

	// Process orders until excess is eliminated
	for excess.Sign() > 0 {
		o := h.Peek()
		if o == nil {
			continue
		}

		// Skip and clean up logically deleted orders
		if s.reduceonly.deleted[&o.ID] {
			heap.Pop(h)
			delete(s.reduceonly.deleted, &o.ID)
			continue
		}

		remaining := math.Sub(o.Quantity, o.Filled)
		// If order fits entirely within excess, cancel it fully
		if math.Cmp(remaining, excess) <= 0 {
			s.reduceonly.Remove(o)
			o.Status = constants.ORDER_STATUS_CANCELED
			o.ClosedAt = utils.NowNano()
			excess = math.Sub(excess, remaining)
			continue
		}

		// Partial cancel: reduce order quantity by excess amount
		o.Quantity = math.Sub(o.Quantity, excess)
		exposureMap := s.reduceonly.exposure[symbol]
		exposure := exposureMap[userID]
		exposureMap[userID] = math.Sub(exposure, excess)
		excess = math.Zero
	}
}

func (s *Service) OnPriceTick(symbol string, price types.Price, callback func(*types.Order)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Local helper: cleanup canceled orders from heap, returns true if cleaned up
	cleanup := func(h *TriggerHeap, top *types.Order) bool {
		if s.conditional.deleted[&top.ID] {
			heap.Pop(h)
			delete(s.conditional.deleted, &top.ID)
			return true
		}
		return false
	}

	// Local helper: trigger order - pop from heap, update status, invoke callback
	trigger := func(h *TriggerHeap, top *types.Order) {
		heap.Pop(h)
		top.Status = constants.ORDER_STATUS_TRIGGERED
		top.UpdatedAt = utils.NowNano()
		callback(top)
	}

	// BUY triggers - trigger when price drops to/below triggerPrice
	if h := s.conditional.buyTriggers[symbol]; h != nil {
		for h.Len() > 0 {
			top := h.Peek()

			if cleanup(h, top) {
				continue
			}

			// BUY: trigger when triggerPrice >= price (price dropped to/below trigger)
			if math.Cmp(top.TriggerPrice, price) < 0 {
				break
			}

			trigger(h, top)
		}
	}

	// SELL triggers - trigger when price rises to/above triggerPrice
	if h := s.conditional.sellTriggers[symbol]; h != nil {
		for h.Len() > 0 {
			top := h.Peek()

			if cleanup(h, top) {
				continue
			}

			// SELL: trigger when triggerPrice <= price (price rose to/above trigger)
			if math.Cmp(top.TriggerPrice, price) > 0 {
				break
			}

			trigger(h, top)
		}
	}
}
