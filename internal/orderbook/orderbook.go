package orderbook

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderBook struct {
	symbol       types.SymbolID
	category     int8
	state        *state.State
	bids         *PriceTree
	asks         *PriceTree
	getOrderByID func(orderID types.OrderID) *types.Order
}

func New(symbol types.SymbolID, category int8, s *state.State, getOrderByID func(orderID types.OrderID) *types.Order) *OrderBook {
	return &OrderBook{
		symbol:       symbol,
		category:     category,
		state:        s,
		bids:         NewPriceTree(true),
		asks:         NewPriceTree(false),
		getOrderByID: getOrderByID,
	}
}

func (ob *OrderBook) PlaceOrder(order *types.Order) ([]*types.Trade, types.Quantity, error) {
	switch order.Type {
	case constants.ORDER_TYPE_LIMIT:
		return ob.placeLimitOrder(order)
	case constants.ORDER_TYPE_MARKET:
		return ob.placeMarketOrder(order)
	}
	return nil, 0, nil
}

func (ob *OrderBook) placeLimitOrder(order *types.Order) ([]*types.Trade, types.Quantity, error) {
	var trades []*types.Trade
	remaining := order.Quantity

	switch order.TIF {
	case constants.TIF_GTC:
		trades, remaining = ob.match(order)
		if remaining > 0 {
			ob.addToBook(order)
			order.Status = constants.ORDER_STATUS_NEW
		}

	case constants.TIF_IOC:
		trades, remaining = ob.match(order)
		if remaining > 0 {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		} else if len(trades) > 0 {
			order.Status = constants.ORDER_STATUS_FILLED
		} else {
			order.Status = constants.ORDER_STATUS_CANCELED
		}

	case constants.TIF_FOK:
		trades, remaining = ob.match(order)
		if remaining > 0 {
			for _, t := range trades {
				ob.reverseTrade(t)
			}
			trades = nil
			remaining = order.Quantity
			order.Status = constants.ORDER_STATUS_CANCELED
		} else {
			order.Status = constants.ORDER_STATUS_FILLED
		}

	case constants.TIF_POST_ONLY:
		// Check if AT LEAST ONE TRADE POSSIBLE
		// WE DONT NEED TO CHECK THE ENTIRE BOOK HERE OKAY ?????
		if ob.wouldCrossSpread(order) {
			order.Status = constants.ORDER_STATUS_CANCELED
			return nil, 0, nil
		}
		// WTF??? REMAINING???????? POST ONLY??????
		trades, remaining = ob.match(order)
		if remaining > 0 {
			ob.addToBook(order)
			order.Status = constants.ORDER_STATUS_NEW
		}
	}

	order.Filled = order.Quantity - remaining
	return trades, remaining, nil
}

func (ob *OrderBook) placeMarketOrder(order *types.Order) ([]*types.Trade, types.Quantity, error) {
	trades, remaining := ob.match(order)
	order.Filled = order.Quantity - remaining

	if order.Filled > 0 {
		order.Status = constants.ORDER_STATUS_FILLED
		if remaining > 0 {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		}
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	return trades, remaining, nil
}

func (ob *OrderBook) match(order *types.Order) ([]*types.Trade, types.Quantity) {
	var trades []*types.Trade
	remaining := order.Quantity

	for remaining > 0 {
		var bestLevel *TreeNode

		if order.Side == constants.ORDER_SIDE_BUY {
			bestLevel = ob.asks.GetBest()
			if bestLevel == nil {
				break
			}
			if order.Price < bestLevel.Price {
				break
			}
		} else {
			bestLevel = ob.bids.GetBest()
			if bestLevel == nil {
				break
			}
			if order.Price > bestLevel.Price {
				break
			}
		}

		for oid, makerOrder := range bestLevel.Orders {
			fillQty := min(remaining, makerOrder.Quantity-makerOrder.Filled)
			if fillQty == 0 {
				continue
			}

			trade := &types.Trade{
				Symbol:       ob.symbol,
				BuyerID:      order.UserID,
				SellerID:     makerOrder.UserID,
				Price:        bestLevel.Price,
				Quantity:     fillQty,
				TakerOrderID: order.ID,
				MakerOrderID: oid,
				ExecutedAt:   types.NanoTime(),
			}
			trades = append(trades, trade)

			makerOrder.Filled += fillQty
			order.Filled += fillQty
			remaining -= fillQty
			bestLevel.Quantity -= fillQty

			if makerOrder.Filled >= makerOrder.Quantity {
				delete(bestLevel.Orders, oid)
				makerOrder.Status = constants.ORDER_STATUS_FILLED
				if makerOrder.ReduceOnly {
					ss := ob.state.GetSymbolState(ob.symbol)
					ss.RemoveReduceOnlyOrder(makerOrder.UserID, oid)
				}
				if len(bestLevel.Orders) == 0 {
					if order.Side == constants.ORDER_SIDE_BUY {
						ob.asks.Remove(bestLevel.Price, bestLevel.Quantity)
					} else {
						ob.bids.Remove(bestLevel.Price, bestLevel.Quantity)
					}
				}
			}

			if remaining == 0 {
				break
			}
		}
	}

	return trades, remaining
}

func (ob *OrderBook) addToBook(order *types.Order) {
	if order.Side == constants.ORDER_SIDE_BUY {
		ob.bids.Insert(order.Price, order.Quantity, order)
	} else {
		ob.asks.Insert(order.Price, order.Quantity, order)
	}

	ss := ob.state.GetSymbolState(ob.symbol)
	ss.OrderMap[order.ID] = order
	if order.ReduceOnly {
		ss.AddReduceOnlyOrder(order.UserID, order.ID)
	}
}

func (ob *OrderBook) wouldCrossSpread(order *types.Order) bool {
	bestAsk := ob.asks.GetBest()
	bestBid := ob.bids.GetBest()

	if order.Side == constants.ORDER_SIDE_BUY {
		return bestAsk != nil && order.Price >= bestAsk.Price
	}
	return bestBid != nil && order.Price <= bestBid.Price
}

func (ob *OrderBook) reverseTrade(trade *types.Trade) {
	taker := ob.getOrderByID(trade.TakerOrderID)
	maker := ob.getOrderByID(trade.MakerOrderID)

	if taker != nil {
		taker.Filled -= trade.Quantity
	}
	if maker != nil {
		maker.Filled -= trade.Quantity
	}
}

func min(a, b types.Quantity) types.Quantity {
	if a < b {
		return a
	}
	return b
}

func (ob *OrderBook) GetBids() []*TreeNode {
	result := make([]*TreeNode, 0)
	ob.collectBids(ob.bids.root, &result)
	return result
}

func (ob *OrderBook) collectBids(node *TreeNode, result *[]*TreeNode) {
	if node == nil {
		return
	}
	ob.collectBids(node.Left, result)
	*result = append(*result, node)
	ob.collectBids(node.Right, result)
}

func (ob *OrderBook) GetAsks() []*TreeNode {
	result := make([]*TreeNode, 0)
	ob.collectAsks(ob.asks.root, &result)
	return result
}

func (ob *OrderBook) collectAsks(node *TreeNode, result *[]*TreeNode) {
	if node == nil {
		return
	}
	ob.collectAsks(node.Left, result)
	*result = append(*result, node)
	ob.collectAsks(node.Right, result)
}
