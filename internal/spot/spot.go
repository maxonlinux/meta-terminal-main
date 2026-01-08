package spot

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/market"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
)

type Market struct {
	validator *Validator
	clearing  *Clearing
	books     *orderbook.State
}

func NewMarket(books *orderbook.State, clearing *Clearing) *Market {
	return &Market{
		validator: &Validator{},
		clearing:  clearing,
		books:     books,
	}
}

func (m *Market) GetValidator() market.Validator { return m.validator }

func (m *Market) GetClearing() market.Clearing { return m.clearing }

func (m *Market) GetCategory() int8 { return constants.CATEGORY_SPOT }

func (m *Market) GetOrderBookState() *orderbook.State { return m.books }
