package engine

import (
	"github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/internal/types"
)

type Engine struct {
	orderbooks map[int8]map[string]*orderbook.OrderBook // orderbooks by category (spot | linear) and symbol
	// LINEAR ONLY
	positions   map[types.UserID]map[string]*types.Position // positions by user ID and symbol
	reduceOnly  map[types.UserID]map[string]interface{}     // reduce only orders by user ID and symbol (can NOT be applied to hedge mode, only for one way!!!)
	conditional map[string]*types.Trigger                   // triggers by symbol
	// END LINEAR ONLY
}

// if spot & reduce only - reject
// if linear & reduce only - check that: order_qty <= (position.size - sum(existing_reduce_only_orders_qty))
// if trigger price > 0 - order is conditional (set IsConditional=true)
// other checks
//
// if post_only & would cross - reject
// if fok - is not enough liquidity in book - reject
// gtc or post only on spot - reject
// PlaceOrder()

// Change size and quantity of an existing order
// New quantity must not be less than already filled qty (if any)
// AmendOrder()

// Cancel the remaining part of order
// CancelOrder()
