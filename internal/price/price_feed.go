package price

import (
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

type Feed struct {
	state  *state.EngineState
	prices map[string]types.Price
}

func NewFeed(s *state.EngineState) *Feed {
	return &Feed{
		state:  s,
		prices: make(map[string]types.Price),
	}
}

func (f *Feed) UpdatePrice(symbol string, price types.Price) {
	f.prices[symbol] = price

	for userID, userState := range f.state.Users {
		pos := userState.Positions[symbol]
		if pos == nil || pos.Size == 0 {
			continue
		}
		if position.CheckLiquidation(pos, price) {
			position.LiquidatePosition(f.state, userID, symbol, price)
		}
	}
}

func (f *Feed) GetPrice(symbol string) types.Price {
	return f.prices[symbol]
}
