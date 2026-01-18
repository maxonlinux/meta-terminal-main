package engine

import (
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// applyMatch updates maker order status and triggers balance movements.
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

	if e.persist != nil {
		makerTrade := buildTrade(match, match.MakerOrder, true)
		takerTrade := buildTrade(match, match.TakerOrder, false)
		if err := e.persist.AppendTrade(makerTrade); err != nil {
			panic("persist maker trade failed: " + err.Error())
		}
		if err := e.persist.AppendTrade(takerTrade); err != nil {
			panic("persist taker trade failed: " + err.Error())
		}
		if err := e.persist.AppendOrderUpdated(match.MakerOrder); err != nil {
			panic("persist maker update failed: " + err.Error())
		}
	}
	// TODO: Apply balance transfers here using BUSINESS_RULES reserve formulas.
}

func buildTrade(match types.Match, order *types.Order, isMaker bool) types.Trade {
	return types.Trade{
		ID:        types.TradeID(snowflake.Next()),
		MatchID:   match.ID,
		OrderID:   order.ID,
		UserID:    order.UserID,
		Symbol:    match.Symbol,
		Category:  match.Category,
		Side:      order.Side,
		Price:     match.Price,
		Quantity:  match.Quantity,
		IsMaker:   isMaker,
		Timestamp: match.Timestamp,
	}
}

// buildPublicTrade builds a taker-side trade for public feeds.
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
