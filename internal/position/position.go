package position

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/memory"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func UpdatePosition(s *state.State, userID types.UserID, symbol types.SymbolID, filledQty types.Quantity, price types.Price, side int8, leverage int8) (*types.Position, int64) {
	us := s.GetUserState(userID)
	pos, ok := us.Positions[symbol]
	if !ok {
		pos = &types.Position{
			UserID:     userID,
			Symbol:     symbol,
			Size:       0,
			Side:       -1,
			EntryPrice: 0,
			Leverage:   leverage,
			Version:    0,
		}
		us.Positions[symbol] = pos
	} else if pos.Leverage == 0 {
		pos.Leverage = leverage
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

func ReduceOnlyValidate(s *state.State, userID types.UserID, symbol types.SymbolID, qty types.Quantity, side int8) error {
	us, ok := s.Users[userID]
	if !ok {
		return errors.New("reduceOnly order requires an existing position")
	}

	pos, ok := us.Positions[symbol]
	if !ok || pos.Size == 0 {
		return errors.New("reduceOnly order requires an existing position")
	}

	isClosing := (side == constants.ORDER_SIDE_SELL && pos.Side == constants.ORDER_SIDE_BUY) ||
		(side == constants.ORDER_SIDE_BUY && pos.Side == constants.ORDER_SIDE_SELL)
	if !isClosing {
		return errors.New("reduceOnly order must close existing position")
	}

	if qty > pos.Size {
		return errors.New("reduceOnly quantity exceeds position size")
	}

	return nil
}

func AdjustReduceOnlyOrders(orderStore *memory.OrderStore, s *state.State, userID types.UserID, symbol types.SymbolID) {
	us, ok := s.Users[userID]
	if !ok {
		return
	}

	pos, ok := us.Positions[symbol]
	if !ok {
		return
	}

	totalReduceOnly := types.Quantity(0)
	userOrderIDs := orderStore.GetUserReduceOnlyOrders(userID)

	for _, oid := range userOrderIDs {
		order := orderStore.Get(oid)
		if order == nil {
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
		order := orderStore.Get(oid)
		if order == nil {
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
		orderStore.Remove(oid)
		orderStore.RemoveReduceOnlyOrder(userID, oid)
	}

	for oid, newQty := range ordersToModify {
		if order := orderStore.Get(oid); order != nil {
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

func GetLeverage(s *state.State, userID types.UserID, symbol types.SymbolID) int8 {
	pos := s.GetUserState(userID).Positions[symbol]
	if pos != nil && pos.Leverage > 0 {
		return pos.Leverage
	}
	return 2
}

func CalculateLiquidationPrice(pos *types.Position, leverage int8) types.Price {
	if pos.Size == 0 || leverage == 0 {
		return 0
	}
	ratio := float64(leverage) / 100.0
	distance := float64(pos.EntryPrice) * ratio * 10
	if pos.Side == constants.ORDER_SIDE_BUY {
		return types.Price(float64(pos.EntryPrice) - distance)
	}
	return types.Price(float64(pos.EntryPrice) + distance)
}
