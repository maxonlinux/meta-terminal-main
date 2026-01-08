package spot

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrSpotReduceOnly   = errors.New("spot does not allow reduceOnly")
	ErrSpotCloseTrigger = errors.New("spot does not allow closeOnTrigger")
	ErrSpotConditional  = errors.New("spot does not allow conditional orders")
	ErrInvalidQty       = errors.New("invalid quantity")
	ErrInvalidPrice     = errors.New("invalid price")
	ErrInvalidSide      = errors.New("invalid side")
	ErrInvalidType      = errors.New("invalid order type")
)

type Validator struct{}

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
		return ErrSpotReduceOnly
	}
	if input.CloseOnTrigger {
		return ErrSpotCloseTrigger
	}
	if input.TriggerPrice > 0 {
		return ErrSpotConditional
	}
	return nil
}
