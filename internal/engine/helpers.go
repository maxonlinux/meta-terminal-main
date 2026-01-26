package engine

import (
	"sync"

	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

var tradePool = sync.Pool{
	New: func() interface{} {
		return &types.Trade{}
	},
}

func getTrade() *types.Trade {
	return tradePool.Get().(*types.Trade)
}

func putTrade(t *types.Trade) {
	*t = types.Trade{}
	tradePool.Put(t)
}

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

func (e *Engine) applyTrade(book *orderbook.OrderBook, match types.Match, writer outbox.Writer) {
	if match.MakerOrder == nil || match.TakerOrder == nil {
		return
	}

	e.clearing.ExecuteTrade(&match, writer)

	_ = e.store.Fill(match.MakerOrder.ID, match.Quantity, writer)
	_ = e.store.Fill(match.TakerOrder.ID, match.Quantity, writer)
	if book != nil {
		book.ApplyFill(match.MakerOrder.ID, match.Quantity)
	}

	publicTrade := getTrade()
	publicTrade.ID = match.ID
	publicTrade.MatchID = match.ID
	publicTrade.OrderID = 0
	publicTrade.UserID = 0
	publicTrade.Symbol = match.Symbol
	publicTrade.Category = match.Category
	publicTrade.Side = match.TakerOrder.Side
	publicTrade.Price = match.Price
	publicTrade.Quantity = match.Quantity
	publicTrade.IsMaker = false
	publicTrade.Timestamp = match.Timestamp
	e.tradeFeed.Add(match.Category, match.Symbol, *publicTrade)
	putTrade(publicTrade)
}
