package validation

import (
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

const (
	ERR_SYMBOL_NOT_FOUND      = 2001
	ERR_CATEGORY_MISMATCH     = 2002
	ERR_LEVERAGE_INVALID      = 2003
	ERR_PRICE_OUT_OF_RANGE    = 2004
	ERR_QUANTITY_OUT_OF_RANGE = 2005
	ERR_TICK_SIZE_VIOLATION   = 2006
	ERR_LOT_SIZE_VIOLATION    = 2007
)

func ValidateSymbol(s *state.State, symbolID types.SymbolID, registry *state.SymbolRegistry) error {
	sym := registry.GetByID(symbolID)
	if sym == nil {
		return &ValidationError{ERR_SYMBOL_NOT_FOUND, "symbol not found"}
	}
	return nil
}

func ValidateCategory(sym *state.SymbolState, expectedCategory int8) error {
	if sym.Category != expectedCategory {
		return &ValidationError{ERR_CATEGORY_MISMATCH, "category mismatch"}
	}
	return nil
}

func ValidatePrice(sym *state.Symbol, price types.Price) error {
	if sym == nil {
		return nil
	}
	if price < sym.MinPrice || price > sym.MaxPrice {
		return &ValidationError{ERR_PRICE_OUT_OF_RANGE, "price out of range"}
	}
	if (price-sym.MinPrice)%sym.TickSize != 0 {
		return &ValidationError{ERR_TICK_SIZE_VIOLATION, "price not aligned to tick size"}
	}
	return nil
}

func ValidateQuantity(sym *state.Symbol, qty types.Quantity) error {
	if sym == nil {
		return nil
	}
	if qty < sym.MinQuantity || qty > sym.MaxQuantity {
		return &ValidationError{ERR_QUANTITY_OUT_OF_RANGE, "quantity out of range"}
	}
	if qty%sym.LotSize != 0 {
		return &ValidationError{ERR_LOT_SIZE_VIOLATION, "quantity not aligned to lot size"}
	}
	return nil
}

func GetErrorMessage(code int) string {
	switch code {
	case ERR_SYMBOL_NOT_FOUND:
		return "Symbol not found"
	case ERR_CATEGORY_MISMATCH:
		return "Category mismatch for this operation"
	case ERR_LEVERAGE_INVALID:
		return "Invalid leverage value"
	case ERR_PRICE_OUT_OF_RANGE:
		return "Price is outside allowed range"
	case ERR_QUANTITY_OUT_OF_RANGE:
		return "Quantity is outside allowed range"
	case ERR_TICK_SIZE_VIOLATION:
		return "Price must be aligned to tick size"
	case ERR_LOT_SIZE_VIOLATION:
		return "Quantity must be aligned to lot size"
	default:
		return "Unknown error"
	}
}
