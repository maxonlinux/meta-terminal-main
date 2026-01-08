package trigger

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/types"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) OnTrigger(s interface{}, order *types.Order, containers interface{}) {
	order.Status = constants.ORDER_STATUS_TRIGGERED
}

func (h *Handler) ClosePosition(s interface{}, userID types.UserID, symbol string, price types.Price) {
	// Implementation depends on engine reference
}
