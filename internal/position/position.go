package position

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/utils"
	"github.com/anomalyco/meta-terminal-go/types"
)

func UpdatePosition(s *state.EngineState, userID types.UserID, symbol string, filledQty types.Quantity, price types.Price, side int8, leverage int8) (*types.Position, int64) {
	us := s.GetUserState(userID)
	pos, ok := us.Positions[symbol]
	if !ok {
		pos = &types.Position{
			UserID:     userID,
			Symbol:     symbol,
			Size:       0,
			Side:       types.SIDE_NONE,
			EntryPrice: 0,
			Leverage:   leverage,
			Version:    0,
		}
		us.Positions[symbol] = pos
	} else if pos.Leverage < 2 {
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
		pos.EntryPrice = types.Price(utils.Avg(int64(pos.EntryPrice), int64(currentSize), int64(price), int64(filledQty)))
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

	CalculatePositionRisk(pos)

	if pos.Size == 0 {
		pos.Side = types.SIDE_NONE
		pos.EntryPrice = 0
		pos.InitialMargin = 0
		pos.MaintenanceMargin = 0
		pos.LiquidationPrice = 0
	}

	pos.Version++
	return pos, realizedPnl
}

func CalculatePositionRisk(pos *types.Position) {
	if pos.Size == 0 {
		pos.InitialMargin = 0
		pos.MaintenanceMargin = 0
		pos.LiquidationPrice = 0
		return
	}

	pos.InitialMargin = utils.MulDiv(int64(pos.Size), int64(pos.EntryPrice), int64(pos.Leverage))
	pos.MaintenanceMargin = utils.Div(pos.InitialMargin, int64(constants.MAINTENANCE_MARGIN_RATIO))

	buffer := utils.Sub(pos.InitialMargin, pos.MaintenanceMargin)

	if pos.Side == constants.ORDER_SIDE_BUY {
		pos.LiquidationPrice = types.Price(utils.Sub(int64(pos.EntryPrice), utils.Div(buffer, int64(pos.Size))))
	} else {
		pos.LiquidationPrice = types.Price(utils.Add(int64(pos.EntryPrice), utils.Div(buffer, int64(pos.Size))))
	}
}

func CheckLiquidation(pos *types.Position, currentPrice types.Price) bool {
	if pos.Size == 0 {
		return false
	}

	var upnl int64
	if pos.Side == constants.ORDER_SIDE_BUY {
		upnl = int64(currentPrice-pos.EntryPrice) * int64(pos.Size)
	} else {
		upnl = int64(pos.EntryPrice-currentPrice) * int64(pos.Size)
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	return upnl < -buffer
}

func LiquidatePosition(s *state.EngineState, userID types.UserID, symbol string, currentPrice types.Price) bool {
	us, ok := s.Users[userID]
	if !ok {
		return false
	}

	pos, ok := us.Positions[symbol]
	if !ok || pos.Size == 0 {
		return false
	}

	if CheckLiquidation(pos, currentPrice) {
		pos.Size = 0
		pos.Side = types.SIDE_NONE
		pos.EntryPrice = 0
		pos.InitialMargin = 0
		pos.MaintenanceMargin = 0
		pos.LiquidationPrice = 0
		pos.Version++
		return true
	}
	return false
}

func GetPosition(s *state.EngineState, userID types.UserID, symbol string) *types.Position {
	us, ok := s.Users[userID]
	if !ok {
		return nil
	}
	return us.Positions[symbol]
}

func ReduceOnlyValidate(s *state.EngineState, userID types.UserID, symbol string, qty types.Quantity, side int8) error {
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

func GetLeverage(s *state.EngineState, userID types.UserID, symbol string) int8 {
	pos := s.GetUserState(userID).Positions[symbol]
	if pos != nil && pos.Leverage > 0 {
		return pos.Leverage
	}
	return 2
}
