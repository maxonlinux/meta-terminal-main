package engine

import (
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// ExportOrders exposes OMS orders for recovery snapshots.
func (e *Engine) ExportOrders() []types.Order {
	return e.store.Orders()
}

// ExportBalances exposes portfolio balances for recovery snapshots.
func (e *Engine) ExportBalances() []types.Balance {
	return e.portfolio.ExportBalances()
}

// ExportPositions exposes portfolio positions for recovery snapshots.
func (e *Engine) ExportPositions() []types.Position {
	return e.portfolio.ExportPositions()
}

// ExportFundings exposes funding requests for recovery snapshots.
func (e *Engine) ExportFundings() []types.FundingRequest {
	return e.portfolio.ExportFundings()
}

// ImportOrders replaces OMS state and rebuilds orderbooks.
func (e *Engine) ImportOrders(orders []types.Order) {
	e.store.LoadOrders(orders)
	e.rebuildOrderbooks(orders)
}

// ImportBalances replaces portfolio balances state.
func (e *Engine) ImportBalances(balances []types.Balance) {
	e.portfolio.ImportBalances(balances)
}

// ImportPositions replaces portfolio positions state.
func (e *Engine) ImportPositions(positions []types.Position) {
	e.portfolio.ImportPositions(positions)
}

// ImportFundings replaces funding requests state.
func (e *Engine) ImportFundings(fundings []types.FundingRequest) {
	e.portfolio.ImportFundings(fundings)
}

func (e *Engine) rebuildOrderbooks(orders []types.Order) {
	e.books = map[int8]map[string]*orderbook.OrderBook{
		constants.CATEGORY_SPOT:   make(map[string]*orderbook.OrderBook),
		constants.CATEGORY_LINEAR: make(map[string]*orderbook.OrderBook),
	}

	for i := range orders {
		order := orders[i]
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		book, err := e.getBook(order.Category, order.Symbol)
		if err != nil {
			continue
		}
		stored, ok := e.store.Get(order.ID)
		if !ok {
			continue
		}
		book.Add(stored)
	}
}
