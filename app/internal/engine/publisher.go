package engine

import "github.com/maxonlinux/meta-terminal-go/pkg/types"

type LiquidationEvent struct {
	UserID types.UserID
	Symbol string
	Stage  string
	Price  types.Price
	Size   types.Quantity
}

type EventPublisher interface {
	OnPublicTrades(category int8, symbol string, trades []types.Trade)
	OnOrderbookUpdated(category int8, symbol string)
	OnOrderUpdated(order *types.Order)
	OnBalanceUpdated(userID types.UserID, asset string, balance *types.Balance)
	OnLiquidation(event LiquidationEvent)
}
