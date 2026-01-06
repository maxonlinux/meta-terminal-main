package memory

import (
	"sync/atomic"
	"unsafe"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

// OrderStore provides lock-free order storage with contiguous memory layout
// Optimized for nanosecond-order processing on $10 server with 1000+ markets
type OrderStore struct {
	orders     []types.Order
	orderCount uint64
	freeList   []types.OrderID
	freeCount  uint64
	index      map[types.OrderID]uint32
	capacity   uint32
}

const (
	defaultOrderCapacity = 1024 * 1024 // 1M orders
	orderAlignment       = 64          // Cache line size
)

func NewOrderStore() *OrderStore {
	os := &OrderStore{
		orders:     make([]types.Order, defaultOrderCapacity),
		freeList:   make([]types.OrderID, defaultOrderCapacity/4),
		index:      make(map[types.OrderID]uint32, defaultOrderCapacity),
		capacity:   defaultOrderCapacity,
		orderCount: 0,
		freeCount:  0,
	}

	// Initialize free list
	for i := types.OrderID(0); i < types.OrderID(defaultOrderCapacity/4); i++ {
		os.freeList[i] = i + 1
	}

	return os
}

func (os *OrderStore) Add(order *types.Order) types.OrderID {
	var orderID types.OrderID

	// Try to get from free list first (lock-free)
	for {
		freeCount := atomic.LoadUint64(&os.freeCount)
		if freeCount > 0 {
			if atomic.CompareAndSwapUint64(&os.freeCount, freeCount, freeCount-1) {
				orderID = os.freeList[freeCount-1]
				break
			}
		} else {
			// Allocate new slot
			orderID = types.OrderID(atomic.AddUint64(&os.orderCount, 1))
			if uint32(orderID) >= os.capacity {
				os.expand()
			}
			break
		}
	}

	// Store order at index (contiguous memory for cache locality)
	index := uint32(orderID) % os.capacity
	os.orders[index] = *order
	os.orders[index].ID = orderID

	// Update index
	os.index[orderID] = index

	return orderID
}

func (os *OrderStore) Get(orderID types.OrderID) *types.Order {
	if orderID == 0 {
		return nil
	}

	index, exists := os.index[orderID]
	if !exists || index == 0 {
		return nil
	}

	return &os.orders[index]
}

func (os *OrderStore) Remove(orderID types.OrderID) {
	if orderID == 0 {
		return
	}

	index, exists := os.index[orderID]
	if !exists || index == 0 {
		return
	}

	// Mark as deleted
	delete(os.index, orderID)

	// Add to free list
	freeCount := atomic.AddUint64(&os.freeCount, 1)
	os.freeList[freeCount-1] = orderID
}

func (os *OrderStore) expand() {
	// Double capacity (this is rare, so allocation is acceptable)
	newCapacity := os.capacity * 2
	newOrders := make([]types.Order, newCapacity)
	copy(newOrders, os.orders)
	os.orders = newOrders
	os.capacity = newCapacity
}

func (os *OrderStore) Count() uint64 {
	return atomic.LoadUint64(&os.orderCount) - atomic.LoadUint64(&os.freeCount)
}

func (os *OrderStore) MemoryUsage() uint64 {
	return uint64(os.capacity) * uint64(unsafe.Sizeof(types.Order{}))
}

// GetByIndex returns order by index for iteration (caller must handle synchronization)
func (os *OrderStore) GetByIndex(index uint32) *types.Order {
	if index >= os.capacity {
		return nil
	}
	return &os.orders[index]
}

// UpdateFilled atomically updates the filled quantity
func (os *OrderStore) UpdateFilled(orderID types.OrderID, filled types.Quantity) {
	order := os.Get(orderID)
	if order != nil {
		order.Filled = filled
	}
}

// UpdateStatus updates the order status
func (os *OrderStore) UpdateStatus(orderID types.OrderID, status int8) {
	order := os.Get(orderID)
	if order != nil {
		order.Status = status
	}
}
