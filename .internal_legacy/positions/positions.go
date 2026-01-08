package positions

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/domain"
	"github.com/anomalyco/meta-terminal-go/internal/safemath"
)

type Position struct {
	Symbol string

	Size       domain.Quantity
	Side       int8
	EntryPrice domain.Price
	Leverage   int8

	InitialMargin     int64
	MaintenanceMargin int64
	LiquidationPrice  domain.Price

	Version int64
}

func New(symbol string) *Position {
	return &Position{
		Symbol: symbol,
		Side:   constants.SIDE_NONE,
	}
}

func (p *Position) Update(tradeSize domain.Quantity, tradePrice domain.Price, side int8, leverage int8) {
	if tradeSize <= 0 {
		return
	}
	if leverage <= 0 {
		leverage = 1
	}

	if p.Size == 0 {
		p.Side = side
		p.EntryPrice = tradePrice
		p.Leverage = leverage
		p.Size = tradeSize
	} else if p.Side == side {
		totalSize := p.Size + tradeSize
		totalCost := safemath.AddSaturating(
			safemath.MulSaturating(int64(p.EntryPrice), int64(p.Size)),
			safemath.MulSaturating(int64(tradePrice), int64(tradeSize)),
		)
		p.EntryPrice = domain.Price(safemath.Div(totalCost, int64(totalSize)))
		p.Size = totalSize
	} else {
		if tradeSize >= p.Size {
			p.Size = tradeSize - p.Size
			p.Side = side
			p.EntryPrice = tradePrice
			p.Leverage = leverage
		} else {
			p.Size = p.Size - tradeSize
			if p.Size == 0 {
				p.Side = constants.SIDE_NONE
				p.EntryPrice = 0
			}
		}
	}

	p.InitialMargin = safemath.MulDivSaturating(int64(tradePrice), int64(tradeSize), int64(leverage))
	p.MaintenanceMargin = safemath.Div(p.InitialMargin, constants.MAINTENANCE_MARGIN_RATIO)
	p.LiquidationPrice = p.calculateLiquidationPrice()

	p.Version++
}

func (p *Position) calculateLiquidationPrice() domain.Price {
	if p.Size <= 0 || p.EntryPrice <= 0 || p.InitialMargin <= 0 {
		return 0
	}
	marginPerUnit := safemath.Div(p.InitialMargin-p.MaintenanceMargin, int64(p.Size))
	if p.Side == constants.SIDE_LONG {
		return domain.Price(safemath.SubSaturating(int64(p.EntryPrice), marginPerUnit))
	}
	if p.Side == constants.SIDE_SHORT {
		return domain.Price(safemath.AddSaturating(int64(p.EntryPrice), marginPerUnit))
	}
	return 0
}

func (p *Position) ShouldLiquidate(currentPrice domain.Price) bool {
	if p.Size == 0 || p.LiquidationPrice == 0 {
		return false
	}
	if p.Side == constants.SIDE_LONG {
		return currentPrice <= p.LiquidationPrice
	}
	if p.Side == constants.SIDE_SHORT {
		return currentPrice >= p.LiquidationPrice
	}
	return false
}
