package position

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

func AdjustReduceOnlyOrders(orderStore *state.OrderStore, s *state.EngineState, userID types.UserID, symbol string) {
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
	}

	for oid, newQty := range ordersToModify {
		if order := orderStore.Get(oid); order != nil {
			order.Quantity = newQty
		}
	}
}
