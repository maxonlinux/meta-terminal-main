package engine

import (
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

type Engine struct {
	wal   *wal.WAL
	state *state.State
}

func New(w *wal.WAL, s *state.State) *Engine {
	return &Engine{
		wal:   w,
		state: s,
	}
}

func (e *Engine) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	return nil, nil
}

func (e *Engine) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	return nil
}

func (e *Engine) AmendOrder(orderID types.OrderID, userID types.UserID, newQuantity types.Quantity, newPrice types.Price) error {
	return nil
}

func (e *Engine) GetOrder(orderID types.OrderID) *types.Order {
	return nil
}

func (e *Engine) GetUserBalances(userID types.UserID) []*types.UserBalance {
	return nil
}

func (e *Engine) GetUserPosition(userID types.UserID, symbol types.SymbolID) *types.Position {
	return nil
}

func (e *Engine) GetOrderBook(symbol types.SymbolID, limit int) ([]*types.Order, []*types.Order) {
	return nil, nil
}
