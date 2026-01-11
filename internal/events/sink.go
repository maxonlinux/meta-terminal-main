package events

import "github.com/anomalyco/meta-terminal-go/internal/types"

type Sink interface {
	OnOrderEvent(event *types.OrderEvent)
	OnTradeEvent(event *types.TradeEvent)
	OnPositionReduced(event *types.PositionReducedEvent)
}

type NopSink struct{}

func (NopSink) OnOrderEvent(*types.OrderEvent)                {}
func (NopSink) OnTradeEvent(*types.TradeEvent)                {}
func (NopSink) OnPositionReduced(*types.PositionReducedEvent) {}
