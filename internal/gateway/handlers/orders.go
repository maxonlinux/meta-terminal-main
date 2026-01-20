package handlers

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type OrdersHandler struct {
	engine *engine.Engine
}

func NewOrdersHandler(eng *engine.Engine) *OrdersHandler {
	return &OrdersHandler{engine: eng}
}

type OrderRequest struct {
	ClientRequestID *string `json:"clientRequestId"`
	Symbol          string  `json:"symbol"`
	Category        int8    `json:"category"`
	Side            int8    `json:"side"`
	OrderType       int8    `json:"type"`
	TimeInForce     int8    `json:"timeInForce"`
	Quantity        string  `json:"qty"`
	Price           *string `json:"price"`
	TriggerPrice    *string `json:"triggerPrice"`
	ReduceOnly      *bool   `json:"reduceOnly"`
	CloseOnTrigger  *bool   `json:"closeOnTrigger"`
	StopOrderType   *int8   `json:"stopOrderType"`
}

type OrderResponse struct {
	ID             types.OrderID `json:"id"`
	UserID         types.UserID  `json:"userId"`
	Symbol         string        `json:"symbol"`
	Category       int8          `json:"category"`
	Side           int8          `json:"side"`
	Type           int8          `json:"type"`
	TimeInForce    int8          `json:"timeInForce"`
	Status         int8          `json:"status"`
	Quantity       string        `json:"qty"`
	Filled         string        `json:"filled"`
	Price          string        `json:"price"`
	TriggerPrice   *string       `json:"triggerPrice,omitempty"`
	ReduceOnly     bool          `json:"reduceOnly"`
	CloseOnTrigger bool          `json:"closeOnTrigger"`
	StopOrderType  *int8         `json:"stopOrderType,omitempty"`
	IsConditional  bool          `json:"isConditional"`
	CreatedAt      uint64        `json:"createdAt"`
	UpdatedAt      uint64        `json:"updatedAt"`
}

func (h *OrdersHandler) CreateOrder(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	var req OrderRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	qty, err := parseFixed(req.Quantity)
	if err != nil {
		return BadRequest(c, "invalid quantity")
	}

	price := types.Price{}
	if req.Price != nil {
		price, err = parseFixed(*req.Price)
		if err != nil {
			return BadRequest(c, "invalid price")
		}
	}

	triggerPrice := types.Price{}
	if req.TriggerPrice != nil {
		triggerPrice, err = parseFixed(*req.TriggerPrice)
		if err != nil {
			return BadRequest(c, "invalid trigger price")
		}
	}

	var stopOrderType int8
	if req.StopOrderType != nil {
		stopOrderType = *req.StopOrderType
	}

	result := h.engine.Cmd(&engine.PlaceOrderCmd{
		Req: &types.PlaceOrderRequest{
			UserID:         claims.UserID,
			Symbol:         req.Symbol,
			Category:       req.Category,
			Side:           req.Side,
			Type:           req.OrderType,
			TIF:            req.TimeInForce,
			Quantity:       qty,
			Price:          price,
			TriggerPrice:   triggerPrice,
			ReduceOnly:     req.ReduceOnly != nil && *req.ReduceOnly,
			CloseOnTrigger: req.CloseOnTrigger != nil && *req.CloseOnTrigger,
			StopOrderType:  stopOrderType,
		},
	})

	if result.Err != nil {
		return BadRequest(c, result.Err.Error())
	}

	return Created(c, orderToResponse(result.Order))
}

func (h *OrdersHandler) GetOrders(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	symbol := c.QueryParam("symbol")
	category := c.QueryParam("category")

	var cat int8
	if category != "" {
		v, err := strconv.ParseInt(category, 10, 8)
		if err != nil {
			return BadRequest(c, "invalid category")
		}
		cat = int8(v)
	}

	orders := h.engine.GetOrders(claims.UserID, symbol, cat)

	resp := make([]OrderResponse, len(orders))
	for i, o := range orders {
		resp[i] = orderToResponse(o)
	}

	return Success(c, map[string]interface{}{
		"orders": resp,
	})
}

func (h *OrdersHandler) GetOrder(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return BadRequest(c, "invalid order id")
	}

	order, ok := h.engine.GetOrder(types.OrderID(id))
	if !ok {
		return NotFound(c, "order not found")
	}

	if order.UserID != claims.UserID {
		return NotFound(c, "order not found")
	}

	return Success(c, orderToResponse(order))
}

func (h *OrdersHandler) CancelOrder(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return BadRequest(c, "invalid order id")
	}

	result := h.engine.Cmd(&engine.CancelOrderCmd{OrderID: types.OrderID(id)})
	if result.Err != nil {
		return BadRequest(c, result.Err.Error())
	}

	return Success(c, orderToResponse(result.Order))
}

type AmendRequest struct {
	Quantity string `json:"qty"`
}

func (h *OrdersHandler) AmendOrder(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return BadRequest(c, "invalid order id")
	}

	var req AmendRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	qty, err := parseFixed(req.Quantity)
	if err != nil {
		return BadRequest(c, "invalid quantity")
	}

	result := h.engine.Cmd(&engine.AmendOrderCmd{
		OrderID: types.OrderID(id),
		NewQty:  qty,
	})

	if result.Err != nil {
		return BadRequest(c, result.Err.Error())
	}

	return Success(c, orderToResponse(result.Order))
}

func orderToResponse(o *types.Order) OrderResponse {
	resp := OrderResponse{
		ID:             o.ID,
		UserID:         o.UserID,
		Symbol:         o.Symbol,
		Category:       o.Category,
		Side:           o.Side,
		Type:           o.Type,
		TimeInForce:    o.TIF,
		Status:         o.Status,
		Quantity:       o.Quantity.String(),
		Filled:         o.Filled.String(),
		Price:          o.Price.String(),
		ReduceOnly:     o.ReduceOnly,
		CloseOnTrigger: o.CloseOnTrigger,
		IsConditional:  o.IsConditional,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
	if o.TriggerPrice.Sign() > 0 {
		resp.TriggerPrice = ptr(o.TriggerPrice.String())
	}
	if o.StopOrderType != 0 {
		resp.StopOrderType = ptr(o.StopOrderType)
	}
	return resp
}

func ptr[T any](v T) *T {
	return &v
}

func getUserFromContext(c echo.Context) *users.Claims {
	if v := c.Get("user"); v != nil {
		if claims, ok := v.(*users.Claims); ok {
			return claims
		}
	}
	return nil
}

func parseFixed(s string) (types.Price, error) {
	if s == "" {
		return types.Price(math.Zero), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return types.Price(math.Zero), err
	}
	return types.Price(fixed.NewF(f)), nil
}
