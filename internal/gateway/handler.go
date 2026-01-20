package gateway

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/gateway/auth"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type Handler struct {
	engine      *engine.Engine
	jwtService  *auth.JWTService
	authService *auth.Service
}

func NewHandler(eng *engine.Engine, jwtService *auth.JWTService, authService *auth.Service) *Handler {
	return &Handler{
		engine:      eng,
		jwtService:  jwtService,
		authService: authService,
	}
}

func (h *Handler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	userID, err := h.authService.Register(req.Username, req.Password)
	if err != nil {
		return BadRequest(c, err.Error())
	}

	return Success(c, map[string]interface{}{
		"userId": uint64(userID),
	})
}

func (h *Handler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	user, err := h.authService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return Unauthorized(c, "invalid credentials")
	}

	if !h.authService.ValidatePassword(user, req.Password) {
		return Unauthorized(c, "invalid credentials")
	}

	token, err := h.jwtService.CreateToken(user.UserID, user.Username)
	if err != nil {
		return InternalError(c, "failed to create token")
	}

	c.SetCookie(&http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     auth.CookiePath,
		MaxAge:   auth.CookieMaxAge,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	return Success(c, map[string]interface{}{
		"token": token,
	})
}

func (h *Handler) Logout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     auth.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
	})

	return Success(c, nil)
}

func (h *Handler) Recovery(c echo.Context) error {
	return Success(c, map[string]string{
		"message": "recovery email sent",
	})
}

func (h *Handler) CreateOrder(c echo.Context) error {
	claims := GetUserFromContext(c)
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

func (h *Handler) GetOrders(c echo.Context) error {
	claims := GetUserFromContext(c)
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

func (h *Handler) GetOrder(c echo.Context) error {
	claims := GetUserFromContext(c)
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

func (h *Handler) CancelOrder(c echo.Context) error {
	claims := GetUserFromContext(c)
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

func (h *Handler) AmendOrder(c echo.Context) error {
	claims := GetUserFromContext(c)
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

func (h *Handler) GetPositions(c echo.Context) error {
	claims := GetUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	positions := h.engine.GetPositions(claims.UserID)

	resp := make([]PositionResponse, len(positions))
	for i, p := range positions {
		resp[i] = PositionResponse{
			Symbol:     p.Symbol,
			Size:       p.Size.String(),
			EntryPrice: p.EntryPrice.String(),
			Leverage:   p.Leverage.String(),
		}
	}

	return Success(c, map[string]interface{}{
		"positions": resp,
	})
}

func (h *Handler) SetLeverage(c echo.Context) error {
	claims := GetUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return BadRequest(c, "symbol is required")
	}

	var req LeverageRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	lev, err := parseFixed(req.Leverage)
	if err != nil {
		return BadRequest(c, "invalid leverage")
	}

	result := h.engine.Cmd(&engine.SetLeverageCmd{
		UserID:   claims.UserID,
		Symbol:   symbol,
		Leverage: lev,
	})

	if result.Err != nil {
		return BadRequest(c, result.Err.Error())
	}

	return Success(c, nil)
}

func (h *Handler) GetBalances(c echo.Context) error {
	claims := GetUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	balances := h.engine.GetBalances(claims.UserID)

	resp := make([]BalanceResponse, len(balances))
	for i, b := range balances {
		resp[i] = BalanceResponse{
			Asset:     b.Asset,
			Available: b.Available.String(),
			Locked:    b.Locked.String(),
			Margin:    b.Margin.String(),
		}
	}

	return Success(c, map[string]interface{}{
		"balances": resp,
	})
}

func (h *Handler) GetInstruments(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	instruments := h.engine.GetInstruments(symbol)

	resp := make([]InstrumentResponse, len(instruments))
	for i, inst := range instruments {
		resp[i] = InstrumentResponse{
			Symbol:     inst.Symbol,
			BaseAsset:  inst.BaseAsset,
			QuoteAsset: inst.QuoteAsset,
			PricePrec:  inst.PricePrec,
			QtyPrec:    inst.QtyPrec,
			MinQty:     inst.MinQty.String(),
			MaxQty:     inst.MaxQty.String(),
			MinPrice:   inst.MinPrice.String(),
			MaxPrice:   inst.MaxPrice.String(),
			TickSize:   inst.TickSize.String(),
			LotSize:    inst.LotSize.String(),
		}
	}

	return Success(c, map[string]interface{}{
		"instruments": resp,
	})
}

func (h *Handler) GetOrderBook(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return BadRequest(c, "symbol is required")
	}

	book := h.engine.GetOrderBook(symbol)
	if book == nil {
		return NotFound(c, "order book not found")
	}

	return Success(c, book)
}

func (h *Handler) GetTrades(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return BadRequest(c, "symbol is required")
	}

	trades := h.engine.GetPublicTrades(symbol)

	resp := make([]TradeResponse, len(trades))
	for i, t := range trades {
		resp[i] = TradeResponse{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Category:  t.Category,
			Side:      t.Side,
			Price:     t.Price.String(),
			Quantity:  t.Quantity.String(),
			IsMaker:   t.IsMaker,
			Timestamp: t.Timestamp,
		}
	}

	return Success(c, map[string]interface{}{
		"trades": resp,
	})
}

func (h *Handler) Health(c echo.Context) error {
	return Success(c, map[string]string{
		"status": "ok",
	})
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
