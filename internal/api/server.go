package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type Server struct {
	cfg    *config.Config
	engine *engine.Engine
	router *chi.Mux
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

type PlaceOrderRequest struct {
	UserID         int64 `json:"userId"`
	Symbol         int32 `json:"symbol"`
	Category       int8  `json:"category"`
	Side           int8  `json:"side"`
	Type           int8  `json:"type"`
	TIF            int8  `json:"tif"`
	Quantity       int64 `json:"quantity"`
	Price          int64 `json:"price,omitempty"`
	TriggerPrice   int64 `json:"triggerPrice,omitempty"`
	StopOrderType  int8  `json:"stopOrderType,omitempty"`
	ReduceOnly     bool  `json:"reduceOnly"`
	CloseOnTrigger bool  `json:"closeOnTrigger"`
}

type OrderResponse struct {
	OrderID   int64 `json:"orderId"`
	Status    int8  `json:"status"`
	Filled    int64 `json:"filled"`
	Remaining int64 `json:"remaining"`
}

type BalanceResponse struct {
	Asset     string `json:"asset"`
	Available int64  `json:"available"`
	Locked    int64  `json:"locked"`
	Margin    int64  `json:"margin,omitempty"`
}

type PositionResponse struct {
	Symbol      int32 `json:"symbol"`
	Size        int64 `json:"size"`
	Side        int8  `json:"side"`
	EntryPrice  int64 `json:"entryPrice"`
	Leverage    int8  `json:"leverage"`
	RealizedPnl int64 `json:"realizedPnl"`
}

type OrderBookResponse struct {
	Symbol   int32     `json:"symbol"`
	Category int8      `json:"category"`
	Bids     [][]int64 `json:"bids"`
	Asks     [][]int64 `json:"asks"`
}

type TPSLRequest struct {
	UserID      int64 `json:"userId"`
	Symbol      int32 `json:"symbol"`
	TPOrderID   int64 `json:"tpOrderId,omitempty"`
	SLOrderID   int64 `json:"slOrderId,omitempty"`
	TPPrice     int64 `json:"tpPrice,omitempty"`
	SLPrice     int64 `json:"slPrice,omitempty"`
	TPTriggered bool  `json:"tpTriggered,omitempty"`
	SLTriggered bool  `json:"slTriggered,omitempty"`
}

type LeverageRequest struct {
	UserID   int64 `json:"userId"`
	Symbol   int32 `json:"symbol,omitempty"`
	Leverage int8  `json:"leverage"`
}

type ClosePositionRequest struct {
	UserID int64 `json:"userId"`
	Symbol int32 `json:"symbol"`
}

func NewServer(cfg *config.Config, e *engine.Engine) *Server {
	s := &Server{
		cfg:    cfg,
		engine: e,
		router: chi.NewRouter(),
	}

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})
	s.router.Use(corsHandler.Handler)

	s.router.Post("/api/v1/orders", s.placeOrder)
	s.router.Delete("/api/v1/orders/{orderId}", s.cancelOrder)
	s.router.Get("/api/v1/orders", s.getUserOrders)
	s.router.Get("/api/v1/balances", s.getAllUserBalances)
	s.router.Get("/api/v1/balance", s.getUserBalanceByAsset)
	s.router.Route("/api/v1/{category}", func(r chi.Router) {
		r.Get("/{symbol}/orderbook", s.getOrderBook)
		r.Get("/{symbol}/positions", s.getPosition)
		r.Delete("/{symbol}/position", s.closePosition)
		r.Patch("/{symbol}/position/leverage", s.editLeverage)
		r.Put("/{symbol}/position/tpsl", s.editTPSL)
	})

	return s
}

func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.ServerHost + ":" + fmt.Sprintf("%d", s.cfg.ServerPort),
		Handler: s.router,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	return srv.ListenAndServe()
}

func (s *Server) placeOrder(w http.ResponseWriter, r *http.Request) {
	var req PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	s.engine.InitSymbolCategory(types.SymbolID(req.Symbol), req.Category)

	input := &types.OrderInput{
		UserID:         types.UserID(req.UserID),
		Symbol:         types.SymbolID(req.Symbol),
		Category:       req.Category,
		Side:           req.Side,
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       types.Quantity(req.Quantity),
		Price:          types.Price(req.Price),
		TriggerPrice:   types.Price(req.TriggerPrice),
		StopOrderType:  req.StopOrderType,
		ReduceOnly:     req.ReduceOnly,
		CloseOnTrigger: req.CloseOnTrigger,
	}

	result, err := s.engine.PlaceOrder(input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if result == nil || result.Order == nil {
		http.Error(w, "order placement failed", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(OrderResponse{
		OrderID:   int64(result.Order.ID),
		Status:    result.Status,
		Filled:    int64(result.Filled),
		Remaining: int64(result.Remaining),
	})
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	orderIDStr := chi.URLParam(r, "orderId")
	orderID, _ := strconv.ParseInt(orderIDStr, 10, 64)
	if err := s.engine.CancelOrder(types.OrderID(orderID), 0); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) getUserOrders(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("userId")
	if userIDStr == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	orders := s.engine.GetUserOrders(types.UserID(userID))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(orders)
}

func (s *Server) getAllUserBalances(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("userId")
	if userIDStr == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	balances := s.engine.GetUserBalances(types.UserID(userID))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(balances)
}

func (s *Server) getUserBalanceByAsset(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("userId")
	asset := r.URL.Query().Get("asset")
	if userIDStr == "" || asset == "" {
		http.Error(w, "userId and asset are required", http.StatusBadRequest)
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	balance := s.engine.GetUserBalanceByAsset(types.UserID(userID), asset)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(balance)
}

func (s *Server) getOrderBook(w http.ResponseWriter, r *http.Request) {
	categoryStr := chi.URLParam(r, "category")
	symbolStr := chi.URLParam(r, "symbol")
	category, _ := strconv.ParseInt(categoryStr, 10, 8)
	symbol, _ := strconv.ParseInt(symbolStr, 10, 32)

	bids, asks := s.engine.GetOrderBook(types.SymbolID(symbol), 20)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(OrderBookResponse{
		Symbol:   int32(symbol),
		Category: int8(category),
		Bids:     flattenToNested(bids),
		Asks:     flattenToNested(asks),
	})
}

func flattenToNested(flat []int64) [][]int64 {
	if len(flat) == 0 {
		return nil
	}
	result := make([][]int64, len(flat)/2)
	for i := 0; i < len(flat); i += 2 {
		result[i/2] = []int64{flat[i], flat[i+1]}
	}
	return result
}

func (s *Server) getPosition(w http.ResponseWriter, r *http.Request) {
	categoryStr := chi.URLParam(r, "category")
	symbolStr := chi.URLParam(r, "symbol")
	category, _ := strconv.ParseInt(categoryStr, 10, 8)
	symbol, _ := strconv.ParseInt(symbolStr, 10, 32)

	userIDStr := r.URL.Query().Get("userId")
	if userIDStr == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	pos := s.engine.GetUserPosition(types.UserID(userID), types.SymbolID(symbol))
	if pos == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"symbol":   int32(symbol),
			"category": int8(category),
			"size":     0,
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PositionResponse{
		Symbol:      int32(symbol),
		Size:        int64(pos.Size),
		Side:        pos.Side,
		EntryPrice:  int64(pos.EntryPrice),
		Leverage:    pos.Leverage,
		RealizedPnl: pos.RealizedPnl,
	})
}

func (s *Server) closePosition(w http.ResponseWriter, r *http.Request) {
	var req ClosePositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	categoryStr := chi.URLParam(r, "category")
	symbolStr := chi.URLParam(r, "symbol")
	category, _ := strconv.ParseInt(categoryStr, 10, 8)
	symbol, _ := strconv.ParseInt(symbolStr, 10, 32)

	if category == constants.CATEGORY_LINEAR {
		err := s.engine.ClosePosition(types.UserID(req.UserID), types.SymbolID(symbol))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) editLeverage(w http.ResponseWriter, r *http.Request) {
	var req LeverageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	categoryStr := chi.URLParam(r, "category")
	category, _ := strconv.ParseInt(categoryStr, 10, 8)

	if category != constants.CATEGORY_LINEAR {
		http.Error(w, "leverage only available for LINEAR markets", http.StatusBadRequest)
		return
	}

	symbol := types.SymbolID(req.Symbol)
	err := s.engine.EditLeverage(types.UserID(req.UserID), symbol, req.Leverage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) editTPSL(w http.ResponseWriter, r *http.Request) {
	var req TPSLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	categoryStr := chi.URLParam(r, "category")
	category, _ := strconv.ParseInt(categoryStr, 10, 8)

	if category != constants.CATEGORY_LINEAR {
		http.Error(w, "TPSL only available for LINEAR markets", http.StatusBadRequest)
		return
	}

	err := s.engine.EditTPSL(types.UserID(req.UserID), types.SymbolID(req.Symbol), req.TPOrderID, req.SLOrderID, types.Price(req.TPPrice), types.Price(req.SLPrice))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
