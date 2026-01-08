package state

import (
	"github.com/anomalyco/meta-terminal-go/types"
)

type OrderStore struct {
	orders         map[types.OrderID]*types.Order
	userOrders     map[types.UserID][]types.OrderID
	userReduceOnly map[types.UserID][]types.OrderID
}

func NewOrderStore() *OrderStore {
	return &OrderStore{
		orders:         make(map[types.OrderID]*types.Order),
		userOrders:     make(map[types.UserID][]types.OrderID),
		userReduceOnly: make(map[types.UserID][]types.OrderID),
	}
}

func (os *OrderStore) Add(order *types.Order) {
	os.orders[order.ID] = order
	os.userOrders[order.UserID] = append(os.userOrders[order.UserID], order.ID)
	if order.ReduceOnly {
		os.userReduceOnly[order.UserID] = append(os.userReduceOnly[order.UserID], order.ID)
	}
}

func (os *OrderStore) Get(orderID types.OrderID) *types.Order {
	return os.orders[orderID]
}

func (os *OrderStore) Remove(orderID types.OrderID) {
	order := os.orders[orderID]
	if order == nil {
		return
	}
	delete(os.orders, orderID)

	// Remove from reduce only
	if ro, ok := os.userReduceOnly[order.UserID]; ok {
		os.userReduceOnly[order.UserID] = removeFromSlice(ro, orderID)
		if len(os.userReduceOnly[order.UserID]) == 0 {
			delete(os.userReduceOnly, order.UserID)
		}
	}

	os.userOrders[order.UserID] = removeFromSlice(os.userOrders[order.UserID], orderID)
}

func (os *OrderStore) GetUserOrders(userID types.UserID) []types.OrderID {
	return os.userOrders[userID]
}

func (os *OrderStore) GetUserReduceOnlyOrders(userID types.UserID) []types.OrderID {
	return os.userReduceOnly[userID]
}

func removeFromSlice(slice []types.OrderID, item types.OrderID) []types.OrderID {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
