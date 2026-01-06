package pricefeed

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Engine interface {
	ClosePosition(userID types.UserID, symbol types.SymbolID) error
}

type PriceFeed struct {
	state *state.State
	eng   Engine

	prices map[types.SymbolID]types.Price
}

func NewPriceFeed(s *state.State, eng Engine) *PriceFeed {
	return &PriceFeed{
		state:  s,
		eng:    eng,
		prices: make(map[types.SymbolID]types.Price),
	}
}

func (pf *PriceFeed) UpdatePrice(symbol types.SymbolID, price types.Price) {
	pf.prices[symbol] = price
	pf.checkLiquidation(symbol, price)
}

func (pf *PriceFeed) GetPrice(symbol types.SymbolID) types.Price {
	return pf.prices[symbol]
}

func (pf *PriceFeed) checkLiquidation(symbol types.SymbolID, currentPrice types.Price) {
	for userID, us := range pf.state.Users {
		pos := us.Positions[symbol]
		if pos == nil || pos.Size == 0 {
			continue
		}

		if pf.shouldLiquidate(pos, currentPrice) {
			pf.eng.ClosePosition(userID, symbol)
		}
	}
}

func (pf *PriceFeed) shouldLiquidate(pos *types.Position, currentPrice types.Price) bool {
	if pos.Size == 0 {
		return false
	}

	if pos.InitialMargin == 0 {
		return false
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	if buffer <= 0 {
		return false
	}

	var upnl int64
	if pos.Side == constants.ORDER_SIDE_BUY {
		upnl = (int64(currentPrice) - int64(pos.EntryPrice)) * int64(pos.Size)
	} else {
		upnl = (int64(pos.EntryPrice) - int64(currentPrice)) * int64(pos.Size)
	}

	return upnl < -buffer || upnl > buffer
}
