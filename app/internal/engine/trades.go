package engine

import (
	"errors"
	"sync"

	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
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

func (e *Engine) applyTrade(book *orderbook.OrderBook, match types.Match, writer outbox.Writer) error {
	if match.MakerOrder == nil || match.TakerOrder == nil {
		return errors.New("match missing orders")
	}

	inst := e.registry.GetInstrument(match.Symbol)
	if inst == nil {
		return constants.ErrInstrumentNotFound
	}
	if err := e.clearing.ExecuteTrade(&match); err != nil {
		return err
	}

	if err := e.store.Fill(match.MakerOrder.UserID, match.MakerOrder.ID, match.Quantity); err != nil {
		return err
	}
	if err := e.store.Fill(match.TakerOrder.UserID, match.TakerOrder.ID, match.Quantity); err != nil {
		return err
	}
	if book != nil {
		book.ApplyFillUnsafe(match.MakerOrder.ID, match.Quantity)
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

	if e.publisher != nil {
		e.publisher.OnPublicTrades(match.Category, match.Symbol, []types.Trade{*publicTrade})
		e.publisher.OnOrderUpdated(match.MakerOrder)
		e.publisher.OnOrderUpdated(match.TakerOrder)
		e.publisher.OnOrderbookUpdated(match.Category, match.Symbol)
	}
	putTrade(publicTrade)

	if writer != nil {
		_ = writer.Record(events.EncodeTrade(events.TradeEvent{
			TradeID:        match.ID,
			MakerUserID:    match.MakerOrder.UserID,
			TakerUserID:    match.TakerOrder.UserID,
			MakerOrderID:   match.MakerOrder.ID,
			TakerOrderID:   match.TakerOrder.ID,
			MakerOrderType: match.MakerOrder.Type,
			TakerOrderType: match.TakerOrder.Type,
			Instrument:     inst,
			Symbol:         match.Symbol,
			Category:       match.Category,
			Price:          match.Price,
			Quantity:       match.Quantity,
			TakerSide:      match.TakerOrder.Side,
			Timestamp:      match.Timestamp,
		}))
	}
	return nil
}
