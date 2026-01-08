package market

import (
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Market interface {
	GetValidator() Validator
	GetClearing() Clearing
	GetCategory() int8
	GetOrderBookState() *orderbook.State
}

type Validator interface {
	Validate(input *types.OrderInput) error
}

type Clearing interface {
	Reserve(userID types.UserID, symbol string, qty types.Quantity, price types.Price) error
	Release(userID types.UserID, symbol string, qty types.Quantity, price types.Price)
	ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order)
}
