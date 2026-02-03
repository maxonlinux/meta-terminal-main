package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/api/shared"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type OrdersHandler struct {
	engine *engine.Engine
	query  *query.Service
}

func NewOrdersHandler(eng *engine.Engine, q *query.Service) *OrdersHandler {
	return &OrdersHandler{engine: eng, query: q}
}

type OrderRequest struct {
	Symbol         string  `json:"symbol"`
	Category       string  `json:"category"`
	Side           string  `json:"side"`
	OrderType      string  `json:"type"`
	TimeInForce    string  `json:"timeInForce"`
	Quantity       string  `json:"qty"`
	Price          *string `json:"price"`
	ReduceOnly     *bool   `json:"reduceOnly"`
	TriggerPrice   *string `json:"triggerPrice"`
	CloseOnTrigger *bool   `json:"closeOnTrigger"`
	StopOrderType  *string `json:"stopOrderType"`
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

	category, err := shared.ParseCategoryParam(req.Category)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.Side == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "side is required"})
	}
	side, err := shared.ParseSide(req.Side)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.OrderType == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type is required"})
	}
	orderType, err := shared.ParseOrderType(req.OrderType)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.TimeInForce == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "timeInForce is required"})
	}
	tif, err := shared.ParseTif(req.TimeInForce)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if orderType == constants.ORDER_TYPE_LIMIT && req.Price == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "price is required for limit orders"})
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
		parsed, err := shared.ParseStopOrderType(*req.StopOrderType)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		stopOrderType = parsed
	}

	result := h.engine.Cmd(&engine.PlaceOrderCmd{
		Req: &types.PlaceOrderRequest{
			UserID:         claims.UserID,
			Symbol:         req.Symbol,
			Category:       category,
			Origin:         constants.ORDER_ORIGIN_USER,
			Side:           side,
			Type:           orderType,
			TIF:            tif,
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

	return c.JSON(http.StatusCreated, shared.OrderResponseFromOrder(result.Order))
}

func (h *OrdersHandler) List(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	category := c.QueryParam("category")

	var cat *int8
	if category != "" {
		parsed, err := shared.ParseCategoryParam(category)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		cat = &parsed
	}

	orders := h.query.GetOrders(claims.UserID, symbol, cat, "", 0, 0)

	resp := make([]shared.OrderResponse, len(orders))
	for i, o := range orders {
		resp[i] = shared.OrderResponseFromOrder(o)
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

	order, ok := h.query.GetOrder(claims.UserID, types.OrderID(id))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
	}

	return c.JSON(http.StatusOK, shared.OrderResponseFromOrder(order))
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

	result := h.engine.Cmd(&engine.CancelOrderCmd{UserID: claims.UserID, OrderID: types.OrderID(id)})
	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.JSON(http.StatusOK, shared.OrderResponseFromOrder(result.Order))
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
		UserID:  claims.UserID,
		OrderID: types.OrderID(id),
		NewQty:  types.Quantity(fixed.NewF(qty)),
	})

	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.JSON(http.StatusOK, shared.OrderResponseFromOrder(result.Order))
}

func getUser(c echo.Context) *users.Claims {
	if v := c.Get("user"); v != nil {
		if claims, ok := v.(*users.Claims); ok {
			return claims
		}
	}
	return nil
}
