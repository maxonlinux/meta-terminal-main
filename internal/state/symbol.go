package state

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Symbol struct {
	ID           types.SymbolID
	Name         string
	Category     int8
	BaseAsset    string
	QuoteAsset   string
	MinQuantity  types.Quantity
	MaxQuantity  types.Quantity
	MinPrice     types.Price
	MaxPrice     types.Price
	TickSize     types.Price
	LotSize      types.Quantity
	LeverageStep int8
}

type SymbolRegistry struct {
	symbols   map[types.SymbolID]*Symbol
	symbolMap map[string]types.SymbolID
}

func NewSymbolRegistry() *SymbolRegistry {
	return &SymbolRegistry{
		symbols:   make(map[types.SymbolID]*Symbol),
		symbolMap: make(map[string]types.SymbolID),
	}
}

func (r *SymbolRegistry) Register(sym *Symbol) {
	r.symbols[sym.ID] = sym
	r.symbolMap[sym.Name] = sym.ID
}

func (r *SymbolRegistry) GetByID(id types.SymbolID) *Symbol {
	return r.symbols[id]
}

func (r *SymbolRegistry) GetByName(name string) *Symbol {
	if id, ok := r.symbolMap[name]; ok {
		return r.symbols[id]
	}
	return nil
}

func (r *SymbolRegistry) GetCategory(id types.SymbolID) int8 {
	if sym := r.symbols[id]; sym != nil {
		return sym.Category
	}
	return -1
}
