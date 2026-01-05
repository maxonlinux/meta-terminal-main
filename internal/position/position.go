package position

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func UpdatePosition(s *state.State, userID types.UserID, symbol types.SymbolID, filledQty types.Quantity, price types.Price, side int8) (*types.Position, int64) {
	us := s.GetUserState(userID)
	pos, ok := us.Positions[symbol]
	if !ok {
		pos = &types.Position{
			UserID:     userID,
			Symbol:     symbol,
			Size:       0,
			Side:       -1,
			EntryPrice: 0,
			Leverage:   1,
			Version:    0,
		}
		us.Positions[symbol] = pos
	}

	var realizedPnl int64

	if pos.Size == 0 {
		pos.Size = filledQty
		pos.Side = side
		pos.EntryPrice = price
	} else if pos.Side == side {
		currentSize := pos.Size
		newSize := currentSize + filledQty
		newEntryPrice := types.Price((int64(pos.EntryPrice)*int64(currentSize) + int64(price)*int64(filledQty)) / int64(newSize))
		pos.EntryPrice = newEntryPrice
		pos.Size = newSize
	} else {
		if filledQty >= pos.Size {
			closedSize := pos.Size
			if pos.Side == constants.ORDER_SIDE_SELL {
				realizedPnl = int64(pos.EntryPrice-price) * int64(closedSize)
			} else {
				realizedPnl = int64(price-pos.EntryPrice) * int64(closedSize)
			}
			pos.Size = filledQty - pos.Size
			pos.Side = side
			pos.EntryPrice = price
		} else {
			if pos.Side == constants.ORDER_SIDE_SELL {
				realizedPnl = int64(pos.EntryPrice-price) * int64(filledQty)
			} else {
				realizedPnl = int64(price-pos.EntryPrice) * int64(filledQty)
			}
			pos.Size = pos.Size - filledQty
		}
	}

	if pos.Size == 0 {
		pos.Side = -1
		pos.EntryPrice = 0
	}

	pos.Version++
	return pos, realizedPnl
}

func GetPosition(s *state.State, userID types.UserID, symbol types.SymbolID) *types.Position {
	us, ok := s.Users[userID]
	if !ok {
		return nil
	}
	return us.Positions[symbol]
}

func ReduceOnlyValidate(s *state.State, userID types.UserID, symbol types.SymbolID, qty types.Quantity, side int8) bool {
	us, ok := s.Users[userID]
	if !ok {
		return false
	}

	pos, ok := us.Positions[symbol]
	if !ok || pos.Size == 0 {
		return false
	}

	if side == constants.ORDER_SIDE_BUY {
		return false
	}

	return qty <= pos.Size
}

func AdjustReduceOnlyOrders(s *state.State, userID types.UserID, symbol types.SymbolID) {
	us, ok := s.Users[userID]
	if !ok {
		return
	}

	pos, ok := us.Positions[symbol]
	if !ok {
		return
	}

	ss := s.GetSymbolState(symbol)

	totalReduceOnly := types.Quantity(0)
	userOrderIDs := ss.GetUserReduceOnlyOrders(userID)

	for _, oid := range userOrderIDs {
		order, ok := ss.OrderMap[oid]
		if !ok {
			continue
		}
		if order.Filled < order.Quantity {
			totalReduceOnly += order.Quantity - order.Filled
		}
	}

	if totalReduceOnly <= pos.Size {
		return
	}

	remainingToAdjust := totalReduceOnly - pos.Size

	var toDelete []types.OrderID
	ordersToModify := make(map[types.OrderID]types.Quantity)

	for _, oid := range userOrderIDs {
		order, ok := ss.OrderMap[oid]
		if !ok {
			continue
		}
		if order.Filled < order.Quantity {
			orderQty := order.Quantity - order.Filled
			if orderQty <= remainingToAdjust {
				remainingToAdjust -= orderQty
				toDelete = append(toDelete, oid)
				order.Status = constants.ORDER_STATUS_CANCELED
			} else {
				if remainingToAdjust > 0 {
					ordersToModify[oid] = order.Quantity - remainingToAdjust
					remainingToAdjust = 0
				}
			}
		}
	}

	for _, oid := range toDelete {
		delete(ss.OrderMap, oid)
		ss.RemoveReduceOnlyOrder(userID, oid)
	}

	for oid, newQty := range ordersToModify {
		if order, ok := ss.OrderMap[oid]; ok {
			order.Quantity = newQty
		}
	}
}

func abs(qty types.Quantity) types.Quantity {
	if qty < 0 {
		return -qty
	}
	return qty
}

func min(a, b types.Quantity) types.Quantity {
	if a < b {
		return a
	}
	return b
}

func calculatePnl(entryPrice, exitPrice types.Price, size types.Quantity, isLong bool) int64 {
	pnl := int64(exitPrice-entryPrice) * int64(size)
	if !isLong {
		pnl = -pnl
	}
	return pnl
}
