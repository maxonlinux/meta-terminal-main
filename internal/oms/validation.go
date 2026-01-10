package oms

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrReduceOnlySpot               = errors.New("reduceOnly not allowed for SPOT")
	ErrConditionalSpot              = errors.New("conditional orders not allowed for SPOT")
	ErrCloseOnTriggerSpot           = errors.New("closeOnTrigger not allowed for SPOT")
	ErrCloseOnTriggerNoPosition     = errors.New("closeOnTrigger requires an existing position")
	ErrMarketTIF                    = errors.New("market orders must be IOC or FOK")
	ErrReduceOnlyNoPosition         = errors.New("reduceOnly not allowed without position")
	ErrReduceOnlyCommitmentExceeded = errors.New("reduceOnly commitment exceeds position")
	ErrSelfMatch                    = errors.New("self-match prevention: order would match with own order")
	ErrOCOSpot                      = errors.New("OCO orders not allowed for SPOT")
	ErrOCONoPosition                = errors.New("OCO orders require an existing position")
	ErrMarketOrderPrice             = errors.New("market orders must have price = 0")
	ErrInvalidTriggerPrice          = errors.New("invalid trigger price: BUY trigger must be below current price, SELL trigger must be above")
	ErrInvalidQuantity              = errors.New("quantity must be greater than 0")
	ErrInvalidPrice                 = errors.New("price must be greater than or equal to 0 for LIMIT orders")
	ErrInvalidSymbol                = errors.New("invalid symbol format")
	ErrInvalidCategory              = errors.New("invalid category: must be 0 (SPOT) or 1 (LINEAR)")
	ErrInvalidSide                  = errors.New("invalid side: must be 0 (BUY) or 1 (SELL)")
	ErrInvalidOrderType             = errors.New("invalid order type: must be 0 (LIMIT) or 1 (MARKET)")
	ErrInvalidTIF                   = errors.New("invalid time in force")
	ErrInvalidStopOrderType         = errors.New("invalid stop order type")
	ErrOCOTPTriggerInvalid          = errors.New("OCO TP trigger must be > SL trigger for LONG positions")
	ErrOCOSLTriggerInvalid          = errors.New("OCO SL trigger must be < TP trigger for SHORT positions")
	ErrFOKInsufficientLiquidity     = errors.New("FOK: insufficient liquidity in orderbook")
)

func (s *Service) validateOrder(input *types.OrderInput) error {
	isConditionalOrCloseOnTrigger := input.TriggerPrice > 0 || input.CloseOnTrigger
	isOCO := input.OCO != nil

	if isConditionalOrCloseOnTrigger {
		input.IsConditional = true
	}

	if !isConditionalOrCloseOnTrigger && !isOCO && input.Quantity <= 0 {
		return ErrInvalidQuantity
	}

	if input.Category != constants.CATEGORY_SPOT && input.Category != constants.CATEGORY_LINEAR {
		return ErrInvalidCategory
	}

	if input.Side != constants.ORDER_SIDE_BUY && input.Side != constants.ORDER_SIDE_SELL {
		return ErrInvalidSide
	}

	if input.Type != constants.ORDER_TYPE_LIMIT && input.Type != constants.ORDER_TYPE_MARKET {
		return ErrInvalidOrderType
	}

	if input.TIF < constants.TIF_GTC || input.TIF > constants.TIF_POST_ONLY {
		return ErrInvalidTIF
	}

	if input.StopOrderType < 0 || input.StopOrderType > 5 {
		return ErrInvalidStopOrderType
	}

	if !s.isValidSymbolFormat(input.Symbol) {
		return ErrInvalidSymbol
	}

	if input.Type == constants.ORDER_TYPE_LIMIT && input.Price <= 0 && input.OCO == nil {
		return ErrInvalidPrice
	}

	if input.Category == constants.CATEGORY_SPOT {
		if input.ReduceOnly {
			return ErrReduceOnlySpot
		}
		if input.TriggerPrice > 0 {
			return ErrConditionalSpot
		}
		if input.CloseOnTrigger {
			return ErrCloseOnTriggerSpot
		}
	} else {
		if input.Type == constants.ORDER_TYPE_MARKET {
			if input.TIF != constants.TIF_IOC && input.TIF != constants.TIF_FOK {
				return ErrMarketTIF
			}
		}

		if input.CloseOnTrigger {
			pos := s.portfolio.GetPosition(input.UserID, input.Symbol)
			if pos == nil || pos.Size == 0 {
				return ErrCloseOnTriggerNoPosition
			}
		}

		if input.ReduceOnly && !isConditionalOrCloseOnTrigger {
			pos := s.portfolio.GetPosition(input.UserID, input.Symbol)
			if pos == nil || pos.Size == 0 {
				return ErrReduceOnlyNoPosition
			}

			currentCommitment := s.reduceOnlyCommitment[input.UserID][input.Symbol]
			newCommitment := currentCommitment + int64(input.Quantity)
			if newCommitment > pos.Size {
				return ErrReduceOnlyCommitmentExceeded
			}
		}

		if input.TriggerPrice > 0 {
			currentPrice := s.lastPrices[input.Symbol]
			if currentPrice > 0 {
				if input.Side == constants.ORDER_SIDE_BUY && input.TriggerPrice >= currentPrice {
					return ErrInvalidTriggerPrice
				}
				if input.Side == constants.ORDER_SIDE_SELL && input.TriggerPrice <= currentPrice {
					return ErrInvalidTriggerPrice
				}
			}
		}
	}

	return nil
}

func (s *Service) isValidSymbolFormat(symbol string) bool {
	if len(symbol) < 4 || len(symbol) > 20 {
		return false
	}

	quoteAssets := []string{"USDT", "USD", "USDC", "BUSD", "DAI"}
	for _, q := range quoteAssets {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			base := symbol[:len(symbol)-len(q)]
			if len(base) < 2 {
				return false
			}
			for _, c := range base {
				if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
					return false
				}
			}
			return true
		}
	}

	for _, c := range symbol {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return len(symbol) >= 3
}

func (s *Service) checkSelfMatch(input *types.OrderInput) error {
	if input.TriggerPrice > 0 || input.CloseOnTrigger {
		return nil
	}

	if input.Type == constants.ORDER_TYPE_MARKET {
		return s.checkSelfMatchMarket(input)
	}

	ob := s.getOrderBook(input.Category, input.Symbol)

	if input.Side == constants.ORDER_SIDE_BUY {
		_, _, askPrices, askQtys := ob.Depth(1)
		if len(askPrices) > 0 && len(askQtys) > 0 && askQtys[0] > 0 {
			if s.userHasOrderAtPriceOrBetter(input.UserID, input.Symbol, constants.ORDER_SIDE_SELL, askPrices[0]) {
				return ErrSelfMatch
			}
		}
	} else {
		bidPrices, _, _, _ := ob.Depth(1)
		if len(bidPrices) > 0 && bidPrices[0] > 0 {
			if s.userHasOrderAtPriceOrBetter(input.UserID, input.Symbol, constants.ORDER_SIDE_BUY, bidPrices[0]) {
				return ErrSelfMatch
			}
		}
	}

	return nil
}

func (s *Service) checkSelfMatchMarket(input *types.OrderInput) error {
	ob := s.getOrderBook(input.Category, input.Symbol)
	_ = ob

	if input.Side == constants.ORDER_SIDE_BUY {
		if s.userHasOrdersOnSide(input.UserID, input.Symbol, constants.ORDER_SIDE_SELL) {
			return ErrSelfMatch
		}
	} else {
		if s.userHasOrdersOnSide(input.UserID, input.Symbol, constants.ORDER_SIDE_BUY) {
			return ErrSelfMatch
		}
	}

	return nil
}

func (s *Service) userHasOrderAtPriceOrBetter(userID types.UserID, symbol string, side int8, price types.Price) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userOrders, ok := s.orders[userID]; ok {
		for _, order := range userOrders {
			if order.Symbol == symbol && order.Side == side && order.Status == constants.ORDER_STATUS_NEW {
				if side == constants.ORDER_SIDE_BUY && order.Price >= price {
					return true
				}
				if side == constants.ORDER_SIDE_SELL && order.Price <= price {
					return true
				}
			}
		}
	}
	return false
}

func (s *Service) userHasOrdersOnSide(userID types.UserID, symbol string, side int8) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userOrders, ok := s.orders[userID]; ok {
		for _, order := range userOrders {
			if order.Symbol == symbol && order.Side == side && order.Status == constants.ORDER_STATUS_NEW {
				return true
			}
		}
	}
	return false
}

func (s *Service) validateOCO(input *types.OrderInput) error {
	if input.Category == constants.CATEGORY_SPOT {
		return ErrOCOSpot
	}

	pos := s.portfolio.GetPosition(input.UserID, input.Symbol)
	if pos.Size == 0 {
		return ErrOCONoPosition
	}

	if input.OCO == nil {
		return nil
	}

	tpTrigger := input.OCO.TakeProfit.TriggerPrice
	slTrigger := input.OCO.StopLoss.TriggerPrice

	if tpTrigger == 0 || slTrigger == 0 {
		return ErrInvalidTriggerPrice
	}

	if pos.Side == constants.ORDER_SIDE_BUY {
		if tpTrigger <= slTrigger {
			return ErrOCOTPTriggerInvalid
		}
	} else {
		if tpTrigger >= slTrigger {
			return ErrOCOSLTriggerInvalid
		}
	}

	return nil
}
