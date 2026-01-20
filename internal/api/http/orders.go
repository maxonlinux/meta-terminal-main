package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
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
	Symbol         string  `json:"symbol"`
	Category       int8    `json:"category"`
	Side           int8    `json:"side"`
	OrderType      int8    `json:"type"`
	TimeInForce    int8    `json:"timeInForce"`
	Quantity       string  `json:"qty"`
	Price          *string `json:"price"`
	TriggerPrice   *string `json:"triggerPrice"`
	ReduceOnly     *bool   `json:"reduceOnly"`
	CloseOnTrigger *bool   `json:"closeOnTrigger"`
	StopOrderType  *int8   `json:"stopOrderType"`
}

func (h *OrdersHandler) Create(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	var req OrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	qty, err := strconv.ParseFloat(req.Quantity, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid quantity"})
	}

	price := types.Price(fixed.NewI(0, 0))
	if req.Price != nil {
		p, err := strconv.ParseFloat(*req.Price, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid price"})
		}
		price = types.Price(fixed.NewF(p))
	}

	triggerPrice := types.Price(fixed.NewI(0, 0))
	if req.TriggerPrice != nil {
		tp, err := strconv.ParseFloat(*req.TriggerPrice, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid trigger price"})
		}
		triggerPrice = types.Price(fixed.NewF(tp))
	}

	stopOrderType := int8(0)
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
			Quantity:       types.Quantity(fixed.NewF(qty)),
			Price:          price,
			TriggerPrice:   triggerPrice,
			ReduceOnly:     req.ReduceOnly != nil && *req.ReduceOnly,
			CloseOnTrigger: req.CloseOnTrigger != nil && *req.CloseOnTrigger,
			StopOrderType:  stopOrderType,
		},
	})

	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.JSON(http.StatusCreated, orderToMap(result.Order))
}

func (h *OrdersHandler) List(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	category := c.QueryParam("category")

	var cat int8
	if category != "" {
		v, err := strconv.ParseInt(category, 10, 8)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid category"})
		}
		cat = int8(v)
	}

	orders := h.engine.GetOrders(claims.UserID, symbol, cat)

	resp := make([]map[string]interface{}, len(orders))
	for i, o := range orders {
		resp[i] = orderToMap(o)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"orders": resp})
}

func (h *OrdersHandler) Get(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
	}

	order, ok := h.engine.GetOrder(types.OrderID(id))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
	}

	if order.UserID != claims.UserID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
	}

	return c.JSON(http.StatusOK, orderToMap(order))
}

func (h *OrdersHandler) Cancel(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
	}

	result := h.engine.Cmd(&engine.CancelOrderCmd{OrderID: types.OrderID(id)})
	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.JSON(http.StatusOK, orderToMap(result.Order))
}

type AmendRequest struct {
	Quantity string `json:"qty"`
}

func (h *OrdersHandler) Amend(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
	}

	var req AmendRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	qty, err := strconv.ParseFloat(req.Quantity, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid quantity"})
	}

	result := h.engine.Cmd(&engine.AmendOrderCmd{
		OrderID: types.OrderID(id),
		NewQty:  types.Quantity(fixed.NewF(qty)),
	})

	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.JSON(http.StatusOK, orderToMap(result.Order))
}

func orderToMap(o *types.Order) map[string]interface{} {
	if o == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":             uint64(o.ID),
		"userId":         uint64(o.UserID),
		"symbol":         o.Symbol,
		"category":       o.Category,
		"side":           o.Side,
		"type":           o.Type,
		"timeInForce":    o.TIF,
		"status":         o.Status,
		"qty":            o.Quantity.String(),
		"filled":         o.Filled.String(),
		"price":          o.Price.String(),
		"reduceOnly":     o.ReduceOnly,
		"closeOnTrigger": o.CloseOnTrigger,
		"isConditional":  o.IsConditional,
		"createdAt":      o.CreatedAt,
		"updatedAt":      o.UpdatedAt,
	}
	if o.TriggerPrice.Sign() > 0 {
		m["triggerPrice"] = o.TriggerPrice.String()
	}
	if o.StopOrderType != 0 {
		m["stopOrderType"] = o.StopOrderType
	}
	return m
}

func getUser(c echo.Context) *users.Claims {
	if v := c.Get("user"); v != nil {
		if claims, ok := v.(*users.Claims); ok {
			return claims
		}
	}
	return nil
}
