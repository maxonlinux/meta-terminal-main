package oms

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func setOrderResultOrders(result *types.OrderResult, orders ...*types.Order) {
	if result == nil {
		return
	}
	if len(orders) <= len(result.OrdersBuf) {
		result.Orders = result.OrdersBuf[:len(orders)]
		copy(result.Orders, orders)
		return
	}
	result.Orders = make([]*types.Order, len(orders))
	copy(result.Orders, orders)
}
