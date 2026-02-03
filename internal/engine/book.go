package engine

import (
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
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
