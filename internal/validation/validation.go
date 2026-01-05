package validation

import (
	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type ValidationError struct {
	Code    int
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

const (
	ERR_OK                   = 0
	ERR_INSUFFICIENT_BALANCE = 1001
	ERR_REDUCEONLY_INVALID   = 1002
	ERR_REDUCEONLY_SPOT      = 1003
	ERR_INVALID_QUANTITY     = 1004
	ERR_INVALID_PRICE        = 1005
	ERR_INVALID_SIDE         = 1006
	ERR_INVALID_TYPE         = 1007
	ERR_INVALID_TIF          = 1008
	ERR_INVALID_CATEGORY     = 1009
	ERR_POSITION_EXCEEDED    = 1010
	ERR_MARGIN_INSUFFICIENT  = 1011
)

func ValidateOrderInput(input *types.OrderInput, category int8) error {
	if input.Quantity <= 0 {
		return &ValidationError{ERR_INVALID_QUANTITY, "invalid quantity"}
	}

	if input.Type == constants.ORDER_TYPE_LIMIT && input.Price <= 0 {
		return &ValidationError{ERR_INVALID_PRICE, "invalid price for limit order"}
	}

	if input.Side != constants.ORDER_SIDE_BUY && input.Side != constants.ORDER_SIDE_SELL {
		return &ValidationError{ERR_INVALID_SIDE, "invalid side"}
	}

	if input.Type != constants.ORDER_TYPE_LIMIT && input.Type != constants.ORDER_TYPE_MARKET {
		return &ValidationError{ERR_INVALID_TYPE, "invalid order type"}
	}

	if input.TIF != constants.TIF_GTC && input.TIF != constants.TIF_IOC &&
		input.TIF != constants.TIF_FOK && input.TIF != constants.TIF_POST_ONLY {
		return &ValidationError{ERR_INVALID_TIF, "invalid tif"}
	}

	if input.ReduceOnly && category == constants.CATEGORY_SPOT {
		return &ValidationError{ERR_REDUCEONLY_SPOT, "reduceOnly not supported in SPOT"}
	}

	return nil
}

func ValidateReduceOnly(s *state.State, input *types.OrderInput, category int8) error {
	if !input.ReduceOnly {
		return nil
	}

	if category == constants.CATEGORY_SPOT {
		return &ValidationError{ERR_REDUCEONLY_SPOT, "reduceOnly not supported in SPOT"}
	}

	if input.Side == constants.ORDER_SIDE_BUY {
		return &ValidationError{ERR_REDUCEONLY_INVALID, "reduceOnly buy not allowed"}
	}

	pos := position.GetPosition(s, input.UserID, input.Symbol)
	if pos == nil || pos.Size == 0 {
		return &ValidationError{ERR_REDUCEONLY_INVALID, "no position to reduce"}
	}

	if input.Quantity > abs(pos.Size) {
		return &ValidationError{ERR_POSITION_EXCEEDED, "reduceOnly qty exceeds position"}
	}

	return nil
}

func ValidateBalance(s *state.State, input *types.OrderInput, category int8, price types.Price) error {
	if input.Type == constants.ORDER_TYPE_MARKET {
		return nil
	}

	if input.TIF == constants.TIF_IOC || input.TIF == constants.TIF_FOK {
		return nil
	}

	required := int64(price) * int64(input.Quantity)
	if input.Side == constants.ORDER_SIDE_BUY {
		available := balance.GetAvailable(s, input.UserID, "USDT")
		if available < required {
			return &ValidationError{ERR_INSUFFICIENT_BALANCE, "insufficient balance"}
		}
	}

	return nil
}

func ValidateMargin(s *state.State, input *types.OrderInput, category int8, price types.Price, leverage int8) error {
	if category != constants.CATEGORY_LINEAR {
		return nil
	}

	margin := int64(price) * int64(input.Quantity) / int64(leverage)
	available := balance.GetMargin(s, input.UserID, "USDT")

	if available < margin {
		return &ValidationError{ERR_MARGIN_INSUFFICIENT, "insufficient margin"}
	}

	return nil
}

func abs(qty types.Quantity) types.Quantity {
	if qty < 0 {
		return -qty
	}
	return qty
}
