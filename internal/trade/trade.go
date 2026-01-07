package trade

import (
	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/utils"
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
	usTaker := s.GetUserState(taker.UserID)
	oldTakerMargin := int64(0)
	if posTaker := usTaker.Positions[taker.Symbol]; posTaker != nil {
		oldTakerMargin = posTaker.InitialMargin
	}
	_, takerPnl := position.UpdatePosition(s, taker.UserID, taker.Symbol, qty, price, taker.Side, leverage)

	usMaker := s.GetUserState(maker.UserID)
	oldMakerMargin := int64(0)
	if posMaker := usMaker.Positions[maker.Symbol]; posMaker != nil {
		oldMakerMargin = posMaker.InitialMargin
	}
	_, makerPnl := position.UpdatePosition(s, maker.UserID, maker.Symbol, qty, price, maker.Side, leverage)

	tBal := balance.GetOrCreate(s, taker.UserID, "USDT")
	takerNewMargin := usTaker.Positions[taker.Symbol].InitialMargin
	tBal.Margin += takerNewMargin - oldTakerMargin
	tBal.Available += takerPnl
	tBal.Version++

	mBal := balance.GetOrCreate(s, maker.UserID, "USDT")
	makerNewMargin := usMaker.Positions[maker.Symbol].InitialMargin
	mBal.Margin += makerNewMargin - oldMakerMargin
	mBal.Available += makerPnl

	orderMargin := position.CalculateMargin(maker.Quantity, maker.Price, leverage)
	filledRatio := utils.Div(int64(qty), int64(maker.Quantity))
	unlockAmount := utils.Mul(int64(orderMargin), int64(filledRatio))
	mBal.Locked -= unlockAmount
	mBal.Version++
}
