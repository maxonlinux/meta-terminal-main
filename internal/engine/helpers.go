package engine

import (
	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/marketdata"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

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

func (e *Engine) applyMatch(match types.Match) {
	if match.MakerOrder == nil || match.TakerOrder == nil {
		return
	}

	now := utils.NowNano()
	remaining := math.Sub(match.MakerOrder.Quantity, match.MakerOrder.Filled)
	if remaining.Sign() == 0 {
		match.MakerOrder.Status = constants.ORDER_STATUS_FILLED
		match.MakerOrder.UpdatedAt = now
	} else {
		match.MakerOrder.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		match.MakerOrder.UpdatedAt = now
	}

	e.clearing.ExecuteTrade(&match)

	publicTrade := buildPublicTrade(match)
	e.tradeFeed.Add(match.Category, match.Symbol, publicTrade)
}

func buildPublicTrade(match types.Match) types.Trade {
	return types.Trade{
		ID:        match.ID,
		MatchID:   match.ID,
		OrderID:   0,
		UserID:    0,
		Symbol:    match.Symbol,
		Category:  match.Category,
		Side:      match.TakerOrder.Side,
		Price:     match.Price,
		Quantity:  match.Quantity,
		IsMaker:   false,
		Timestamp: match.Timestamp,
	}
}

var _ = clearing.New
var _ = marketdata.NewTradeFeed
