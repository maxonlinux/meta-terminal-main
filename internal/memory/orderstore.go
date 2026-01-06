package memory

import (
	"sync"
	"sync/atomic"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderStore struct {
	slots      []*types.Order
	freeList   []uint64
	userOrders map[types.UserID][]types.OrderID
	mu         sync.Mutex
	nextID     uint64
}

func NewOrderStore() *OrderStore {
	return &OrderStore{
		slots:      make([]*types.Order, 1),
		freeList:   make([]uint64, 0, 1024),
		userOrders: make(map[types.UserID][]types.OrderID),
	}
}

func (os *OrderStore) Add(order *types.Order) types.OrderID {
	var idx uint64

	os.mu.Lock()
	if len(os.freeList) > 0 {
		idx = os.freeList[len(os.freeList)-1]
		os.freeList = os.freeList[:len(os.freeList)-1]
	} else {
		idx = uint64(len(os.slots))
		os.slots = append(os.slots, nil)
	}
	orderID := types.OrderID(atomic.AddUint64(&os.nextID, 1))
	os.slots[idx] = order
	order.ID = orderID

	os.userOrders[order.UserID] = append(os.userOrders[order.UserID], orderID)
	os.mu.Unlock()

	return orderID
}

func (os *OrderStore) Get(orderID types.OrderID) *types.Order {
	if orderID == 0 {
		return nil
	}
	idx := uint64(orderID) % uint64(len(os.slots))
	os.mu.Lock()
	defer os.mu.Unlock()
	order := os.slots[idx]
	if order == nil || order.ID != orderID {
		return nil
	}
	return order
}

func (os *OrderStore) Remove(orderID types.OrderID) {
	if orderID == 0 {
		return
	}
	os.mu.Lock()
	defer os.mu.Unlock()
	idx := uint64(orderID) % uint64(len(os.slots))
	if os.slots[idx] != nil && os.slots[idx].ID == orderID {
		order := os.slots[idx]
		os.slots[idx] = nil
		os.freeList = append(os.freeList, idx)

		userOrders := os.userOrders[order.UserID]
		for i, id := range userOrders {
			if id == orderID {
				os.userOrders[order.UserID] = append(userOrders[:i], userOrders[i+1:]...)
				break
			}
		}
	}
}

func (os *OrderStore) GetUserOrders(userID types.UserID) []types.OrderID {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.userOrders[userID]
}

func (os *OrderStore) Count() uint64 {
	return atomic.LoadUint64(&os.nextID) - uint64(len(os.freeList))
}

func (os *OrderStore) GetByIndex(idx uint32) *types.Order {
	os.mu.Lock()
	defer os.mu.Unlock()
	if idx >= uint32(len(os.slots)) {
		return nil
	}
	return os.slots[idx]
}

func (os *OrderStore) UpdateFilled(orderID types.OrderID, filled types.Quantity) {
	order := os.Get(orderID)
	if order != nil {
		order.Filled = filled
	}
}

func (os *OrderStore) UpdateStatus(orderID types.OrderID, status int8) {
	order := os.Get(orderID)
	if order != nil {
		order.Status = status
	}
}
