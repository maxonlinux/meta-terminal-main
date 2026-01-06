package trade

import (
	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func ExecuteSpotTrade(s *state.State, taker, maker *types.Order, price types.Price, qty types.Quantity) {
	buyer, seller := taker, maker
	if taker.Side == constants.ORDER_SIDE_SELL {
		buyer, seller = maker, taker
	}

	value := int64(qty) * int64(price)
	balance.Transfer(s, buyer.UserID, seller.UserID, "USDT", value)

	asset := "BTC"
	balance.Transfer(s, seller.UserID, buyer.UserID, asset, int64(qty))
}

func ExecuteLinearTrade(s *state.State, taker, maker *types.Order, price types.Price, qty types.Quantity, leverage int8) {
	margin := position.CalculateMargin(qty, price, leverage)

	_, takerPnl := position.UpdatePosition(s, taker.UserID, taker.Symbol, qty, price, taker.Side, leverage)
	_, makerPnl := position.UpdatePosition(s, maker.UserID, maker.Symbol, qty, price, maker.Side, leverage)

	tBal := balance.GetOrCreate(s, taker.UserID, "USDT")
	if taker.Side == constants.ORDER_SIDE_BUY {
		tBal.Margin += margin
		tBal.Available += takerPnl
	} else {
		tBal.Margin -= margin
		tBal.Available += takerPnl
	}
	tBal.Version++

	mBal := balance.GetOrCreate(s, maker.UserID, "USDT")
	if maker.Side == constants.ORDER_SIDE_BUY {
		mBal.Margin += margin
		mBal.Available += makerPnl
	} else {
		mBal.Margin -= margin
		mBal.Available += makerPnl
	}
	mBal.Version++
}
