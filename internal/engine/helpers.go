package engine

import (
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// applyTrade updates maker order status and triggers balance movements.
func (e *Engine) applyTrade(trade types.Trade) {
	if trade.MakerOrder == nil {
		return
	}

	now := utils.NowNano()
	remaining := math.Sub(trade.MakerOrder.Quantity, trade.MakerOrder.Filled)
	if remaining.Sign() == 0 {
		trade.MakerOrder.Status = constants.ORDER_STATUS_FILLED
		trade.MakerOrder.UpdatedAt = now
		return
	}

	trade.MakerOrder.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
	trade.MakerOrder.UpdatedAt = now
	// TODO: Apply balance transfers here using BUSINESS_RULES reserve formulas.
}

// getBook returns the orderbook for the provided market category and symbol.
func (e *Engine) getBook(category int8, symbol string) (*orderbook.OrderBook, error) {
	bookSet, ok := e.books[category]
	if !ok {
		return nil, constants.ErrInvalidCategory
	}
	if book, ok := bookSet[symbol]; ok {
		return book, nil
	}

	book := orderbook.New()
	bookSet[symbol] = book
	return book, nil
}
