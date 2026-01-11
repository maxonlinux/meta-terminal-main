package oms

import (
	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func poolGetOrderID() uint64 {
	return uint64(snowflake.Next())
}

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
