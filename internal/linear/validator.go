package linear

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrInvalidQty   = errors.New("invalid quantity")
	ErrInvalidPrice = errors.New("invalid price")
	ErrInvalidSide  = errors.New("invalid side")
	ErrInvalidType  = errors.New("invalid order type")
	ErrReduceOnly   = errors.New("reduceOnly conflicts with position")
)

type Validator struct {
	users *state.Users
}

func NewValidator(users *state.Users) *Validator {
	return &Validator{users: users}
}

func (v *Validator) Validate(input *types.OrderInput) error {
	if input.Quantity <= 0 {
		return ErrInvalidQty
	}
	if input.Side != constants.ORDER_SIDE_BUY && input.Side != constants.ORDER_SIDE_SELL {
		return ErrInvalidSide
	}
	if input.Type != constants.ORDER_TYPE_LIMIT && input.Type != constants.ORDER_TYPE_MARKET {
		return ErrInvalidType
	}
	if input.Type == constants.ORDER_TYPE_LIMIT && input.Price <= 0 {
		return ErrInvalidPrice
	}
	if input.ReduceOnly {
		pos := v.users.GetPosition(input.UserID, input.Symbol)
		if pos.Size == 0 {
			return ErrReduceOnly
		}
		posSide := pos.Side
		if posSide == constants.SIDE_LONG && input.Side != constants.ORDER_SIDE_SELL {
			return ErrReduceOnly
		}
		if posSide == constants.SIDE_SHORT && input.Side != constants.ORDER_SIDE_BUY {
			return ErrReduceOnly
		}
	}
	return nil
}
