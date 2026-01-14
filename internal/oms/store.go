// Package oms provides Order Management System functionality.
// Store is the SINGLE SOURCE OF TRUTH for all orders.
package oms

import (
	"container/heap"
	"errors"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrRONoPosition       = errors.New("reduce-only requires existing position")
	ErrROHedgeMode        = errors.New("reduce-only not allowed in hedge mode")
	ErrROExceedsPosition  = errors.New("reduce-only order exceeds position size")
	ErrROTrimmingRequired = errors.New("position reduced, RO trimming required")
)

// Store holds all orders as single source of truth.
// All other structures (orderbook, triggers) hold pointers to orders in Store.
type Store struct {
	mu sync.RWMutex

	// Primary index: orderID -> Order
	byID map[types.OrderID]*types.Order

	// User index: userID -> symbol -> OrderID set
	byUser map[types.UserID]map[string]map[types.OrderID]struct{}

	// Reduce-Only indices for trimming (per symbol, per side)
	// SELL: max-heap by price (highest first - furthest from market)
	// BUY: min-heap by price (lowest first - furthest from market)
	roSell map[string]*roHeap
	roBuy  map[string]*roHeap

	// Position tracking for RO validation
	positions map[types.UserID]map[string]types.Quantity
}

// roHeapEntry is an entry in the reduce-only priority heap.
type roHeapEntry struct {
	orderID types.OrderID
	price   types.Price
	qty     types.Quantity
	index   int // heap.Interface required
}

// roHeap implements heap.Interface for RO order trimming.
// For SELL orders: highest price first (max-heap)
// For BUY orders: lowest price first (min-heap)
type roHeap struct {
	sell  bool // true for SELL heap, false for BUY
	items []*roHeapEntry
}

func newROHeap(sell bool) *roHeap {
	h := &roHeap{sell: sell}
	heap.Init(h)
	return h
}

func (h *roHeap) Len() int { return len(h.items) }

func (h *roHeap) Less(i, j int) bool {
	if h.sell {
		return h.items[i].price > h.items[j].price // SELL: max-heap
	}
	return h.items[i].price < h.items[j].price // BUY: min-heap
}

func (h *roHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].index = i
	h.items[j].index = j
}

func (h *roHeap) Push(x interface{}) {
	entry := x.(*roHeapEntry)
	entry.index = len(h.items)
	h.items = append(h.items, entry)
}

func (h *roHeap) Pop() interface{} {
	old := h.items
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	h.items = old[0 : n-1]
	return entry
}

func (h *roHeap) Peek() *roHeapEntry {
	if len(h.items) == 0 {
		return nil
	}
	return h.items[0]
}

// New creates a new Store instance.
func New() *Store {
	return &Store{
		byID:      make(map[types.OrderID]*types.Order),
		byUser:    make(map[types.UserID]map[string]map[types.OrderID]struct{}),
		roSell:    make(map[string]*roHeap),
		roBuy:     make(map[string]*roHeap),
		positions: make(map[types.UserID]map[string]types.Quantity),
	}
}

// AddOrder adds a new order to the store.
// Generates OrderID using snowflake.
// Returns the order with generated ID.
func (s *Store) AddOrder(o *types.Order) *types.Order {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID if not set
	if o.ID == 0 {
		o.ID = types.OrderID(snowflake.Next())
	}
	o.CreatedAt = types.NowNano()
	o.UpdatedAt = o.CreatedAt

	// Add to primary index
	s.byID[o.ID] = o

	// Add to user index
	if s.byUser[o.UserID] == nil {
		s.byUser[o.UserID] = make(map[string]map[types.OrderID]struct{})
	}
	if s.byUser[o.UserID][o.Symbol] == nil {
		s.byUser[o.UserID][o.Symbol] = make(map[types.OrderID]struct{})
	}
	s.byUser[o.UserID][o.Symbol][o.ID] = struct{}{}

	// Add to RO index if reduce-only
	if o.ReduceOnly && o.Category == constants.CATEGORY_LINEAR {
		s.addROOrder(o)
	}

	return o
}

// GetOrder retrieves an order by ID.
func (s *Store) GetOrder(id types.OrderID) *types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id]
}

// UpdateOrder updates an existing order's timestamps.
func (s *Store) UpdateOrder(o *types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.byID[o.ID]; ok {
		o.UpdatedAt = types.NowNano()
	}
}

// GetUserOrders returns all order IDs for a user and symbol.
func (s *Store) GetUserOrders(uid types.UserID, symbol string) []types.OrderID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.byUser[uid] == nil || s.byUser[uid][symbol] == nil {
		return nil
	}

	ids := make([]types.OrderID, 0, len(s.byUser[uid][symbol]))
	for id := range s.byUser[uid][symbol] {
		ids = append(ids, id)
	}
	return ids
}

// RemoveOrder removes an order from the store.
func (s *Store) RemoveOrder(id types.OrderID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.byID[id]
	if !ok {
		return false
	}

	// Remove from primary index
	delete(s.byID, id)

	// Remove from user index
	if s.byUser[o.UserID] != nil && s.byUser[o.UserID][o.Symbol] != nil {
		delete(s.byUser[o.UserID][o.Symbol], id)
	}

	// Remove from RO index
	if o.ReduceOnly {
		s.removeROOrder(o)
	}

	return true
}

// SetPosition updates the position size for a user and symbol.
// Returns true if RO trimming is required.
func (s *Store) SetPosition(uid types.UserID, symbol string, size types.Quantity) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.positions[uid] == nil {
		s.positions[uid] = make(map[string]types.Quantity)
	}
	oldSize := s.positions[uid][symbol]
	s.positions[uid][symbol] = size

	// Check if position reduced - trim RO orders if needed
	if oldSize != 0 && size != 0 && ((oldSize > 0 && size > 0 && size < oldSize) ||
		(oldSize < 0 && size < 0 && size > oldSize)) {
		// Position reduced in same direction
		return true
	}
	return false
}

// GetPosition returns the current position size.
func (s *Store) GetPosition(uid types.UserID, symbol string) types.Quantity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.positions[uid] == nil {
		return 0
	}
	return s.positions[uid][symbol]
}

// TrimROOrders trims reduce-only orders to not exceed position size.
// Uses furthest-first priority: SELL highest price first, BUY lowest price first.
// Returns total trimmed quantity.
func (s *Store) TrimROOrders(uid types.UserID, symbol string, maxSize types.Quantity) types.Quantity {
	if maxSize == 0 {
		return s.cancelAllRO(uid, symbol)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var totalTrimmed types.Quantity
	absMax := maxSize
	if absMax < 0 {
		absMax = -absMax
	}

	// Trim SELL RO orders if position is long (positive)
	if maxSize > 0 {
		heap := s.roSell[symbol]
		if heap != nil {
			for heap.Len() > 0 {
				currentQty := s.getTotalROQty(uid, symbol, constants.ORDER_SIDE_SELL)
				if currentQty <= absMax {
					break
				}
				entry := heap.Peek()
				o := s.byID[entry.orderID]
				if o == nil {
					heap.Pop()
					continue
				}

				excess := currentQty - absMax
				if entry.qty <= excess {
					// Trim entire order
					trimQty := entry.qty
					heap.Pop()
					delete(s.byID, entry.orderID)
					totalTrimmed += trimQty
				} else {
					// Partial trim
					o.Quantity -= excess
					o.Filled += excess
					entry.qty -= excess
					if o.Quantity == o.Filled {
						o.Status = constants.ORDER_STATUS_FILLED
					}
					totalTrimmed += excess
					break
				}
			}
		}
	}

	// Trim BUY RO orders if position is short (negative)
	if maxSize < 0 {
		heap := s.roBuy[symbol]
		if heap != nil {
			for heap.Len() > 0 {
				currentQty := s.getTotalROQty(uid, symbol, constants.ORDER_SIDE_BUY)
				if currentQty <= absMax {
					break
				}
				entry := heap.Peek()
				o := s.byID[entry.orderID]
				if o == nil {
					heap.Pop()
					continue
				}

				excess := currentQty - absMax
				if entry.qty <= excess {
					// Trim entire order
					trimQty := entry.qty
					heap.Pop()
					delete(s.byID, entry.orderID)
					totalTrimmed += trimQty
				} else {
					// Partial trim
					o.Quantity -= excess
					o.Filled += excess
					entry.qty -= excess
					if o.Quantity == o.Filled {
						o.Status = constants.ORDER_STATUS_FILLED
					}
					totalTrimmed += excess
					break
				}
			}
		}
	}

	return totalTrimmed
}

// cancelAllRO cancels all reduce-only orders for a user/symbol.
func (s *Store) cancelAllRO(uid types.UserID, symbol string) types.Quantity {
	s.mu.Lock()
	defer s.mu.Unlock()

	var total types.Quantity

	// Cancel SELL RO
	if heap := s.roSell[symbol]; heap != nil {
		for heap.Len() > 0 {
			entry := heap.Pop().(*roHeapEntry)
			if o := s.byID[entry.orderID]; o != nil {
				o.Status = constants.ORDER_STATUS_CANCELED
				total += o.Quantity - o.Filled
				delete(s.byID, entry.orderID)
			}
		}
		delete(s.roSell, symbol)
	}

	// Cancel BUY RO
	if heap := s.roBuy[symbol]; heap != nil {
		for heap.Len() > 0 {
			entry := heap.Pop().(*roHeapEntry)
			if o := s.byID[entry.orderID]; o != nil {
				o.Status = constants.ORDER_STATUS_CANCELED
				total += o.Quantity - o.Filled
				delete(s.byID, entry.orderID)
			}
		}
		delete(s.roBuy, symbol)
	}

	return total
}

// addROOrder adds a reduce-only order to the trimming heap.
func (s *Store) addROOrder(o *types.Order) {
	entry := &roHeapEntry{
		orderID: o.ID,
		price:   o.Price,
		qty:     o.Remaining(),
	}

	var heap *roHeap
	if o.Side == constants.ORDER_SIDE_SELL {
		if s.roSell[o.Symbol] == nil {
			s.roSell[o.Symbol] = newROHeap(true)
		}
		heap = s.roSell[o.Symbol]
	} else {
		if s.roBuy[o.Symbol] == nil {
			s.roBuy[o.Symbol] = newROHeap(false)
		}
		heap = s.roBuy[o.Symbol]
	}
	heap.Push(entry)
}

func (h *roHeap) Fix(i int) {
	heap.Fix(h, i)
}

// removeROOrder removes a reduce-only order from the trimming heap.
func (s *Store) removeROOrder(o *types.Order) {
	// Note: Full removal from heap is O(n), but we mark as inactive
	// and clean up during trimming. For most cases, orders are removed
	// via TrimROOrders which handles heap cleanup.
	if o.Side == constants.ORDER_SIDE_SELL {
		if heap := s.roSell[o.Symbol]; heap != nil {
			for i, entry := range heap.items {
				if entry.orderID == o.ID {
					heap.items[i] = heap.items[len(heap.items)-1]
					heap.items[len(heap.items)-1] = nil
					heap.items = heap.items[:len(heap.items)-1]
					if i < len(heap.items) {
						heap.Fix(i)
					}
					break
				}
			}
		}
	} else {
		if heap := s.roBuy[o.Symbol]; heap != nil {
			for i, entry := range heap.items {
				if entry.orderID == o.ID {
					heap.items[i] = heap.items[len(heap.items)-1]
					heap.items[len(heap.items)-1] = nil
					heap.items = heap.items[:len(heap.items)-1]
					if i < len(heap.items) {
						heap.Fix(i)
					}
					break
				}
			}
		}
	}
}

// getTotalROQty returns total remaining quantity of RO orders for user/symbol/side.
func (s *Store) getTotalROQty(uid types.UserID, symbol string, side int8) types.Quantity {
	var total types.Quantity
	if s.byUser[uid] != nil && s.byUser[uid][symbol] != nil {
		for id := range s.byUser[uid][symbol] {
			if o := s.byID[id]; o != nil && o.ReduceOnly && o.Side == side && o.Status == constants.ORDER_STATUS_NEW {
				total += o.Remaining()
			}
		}
	}
	return total
}

// Count returns total number of orders in store.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}
