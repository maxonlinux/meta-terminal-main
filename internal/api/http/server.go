package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/persistence"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

type Server struct {
	engine      *engine.Engine
	persistence *persistence.Manager
	registry    *registry.Registry
}

func NewServer(engine *engine.Engine, persistence *persistence.Manager, registry *registry.Registry) *Server {
	return &Server{engine: engine, persistence: persistence, registry: registry}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/trading/order", s.handleOrder)
	mux.HandleFunc("/open-orders", s.handleOpenOrders)
	mux.HandleFunc("/trading/position", s.handlePosition)
	mux.HandleFunc("/trading/position/leverage", s.handleLeverage)
	mux.HandleFunc("/trading/balances", s.handleBalances)
	mux.HandleFunc("/trading/balance", s.handleBalance)
	mux.HandleFunc("/trading/", s.handleTradingDynamic)
}

type placeOrderRequest struct {
	UserID         uint64         `json:"userId"`
	Symbol         string         `json:"symbol"`
	Category       string         `json:"category"`
	Side           string         `json:"side"`
	Type           string         `json:"type"`
	TIF            string         `json:"tif"`
	Quantity       flexibleNumber `json:"qty"`
	Price          flexibleNumber `json:"price"`
	TriggerPrice   flexibleNumber `json:"triggerPrice"`
	ReduceOnly     bool           `json:"reduceOnly"`
	CloseOnTrigger bool           `json:"closeOnTrigger"`
	StopOrderType  int8           `json:"stopOrderType"`
	Leverage       int8           `json:"leverage"`
}

type cancelOrderRequest struct {
	UserID  uint64 `json:"userId"`
	OrderID uint64 `json:"orderId"`
}

type leverageRequest struct {
	UserID   uint64 `json:"userId"`
	Symbol   string `json:"symbol"`
	Leverage int8   `json:"leverage"`
}

func (s *Server) handleOrder(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req placeOrderRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		category, err := parseCategory(req.Category)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category"})
			return
		}
		side, err := parseSide(req.Side)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid side"})
			return
		}
		orderType, err := parseType(req.Type)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid type"})
			return
		}
		tif, err := parseTIF(req.TIF)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tif"})
			return
		}
		qty, err := parseInt64(string(req.Quantity))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty"})
			return
		}
		price := int64(0)
		if req.Price != "" {
			price, err = parseInt64(string(req.Price))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid price"})
				return
			}
		}
		trigger := int64(0)
		if req.TriggerPrice != "" {
			trigger, err = parseInt64(string(req.TriggerPrice))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid triggerPrice"})
				return
			}
		}
		input := &types.OrderInput{
			UserID:         types.UserID(req.UserID),
			Symbol:         req.Symbol,
			Category:       category,
			Side:           side,
			Type:           orderType,
			TIF:            tif,
			Quantity:       types.Quantity(qty),
			Price:          types.Price(price),
			TriggerPrice:   types.Price(trigger),
			ReduceOnly:     req.ReduceOnly,
			CloseOnTrigger: req.CloseOnTrigger,
			StopOrderType:  req.StopOrderType,
			Leverage:       req.Leverage,
		}
		result, err := s.engine.PlaceOrder(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if s.persistence != nil {
			payload := persistence.PlaceOrderPayload(result.Order.ID, input)
			_ = s.persistence.Append(wal.EventPlaceOrder, payload)
		}
		resp := map[string]any{
			"order":  orderToResponse(result.Order),
			"status": result.Status,
		}
		if len(result.Trades) > 0 {
			trades := make([]TradeResponse, 0, len(result.Trades))
			for _, trade := range result.Trades {
				trades = append(trades, tradeToResponse(trade))
			}
			resp["trades"] = trades
		}
		writeJSON(w, http.StatusOK, resp)
		s.engine.ReleaseResult(result)
	case http.MethodDelete:
		var req cancelOrderRequest
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		orderID := types.OrderID(req.OrderID)
		userID := types.UserID(req.UserID)
		if err := s.engine.CancelOrder(orderID, userID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if s.persistence != nil {
			payload := persistence.CancelOrderPayload(orderID, userID)
			_ = s.persistence.Append(wal.EventCancelOrder, payload)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "canceled"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOpenOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := parseUint64(r.URL.Query().Get("userId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid userId"})
		return
	}
	orders := s.engine.OpenOrders(types.UserID(userID))
	out := make([]OrderResponse, 0, len(orders))
	for _, order := range orders {
		out = append(out, orderToResponse(order))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := parseUint64(r.URL.Query().Get("userId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid userId"})
		return
	}
	symbol := r.URL.Query().Get("symbol")
	if symbol != "" {
		pos := s.engine.Position(types.UserID(userID), symbol)
		writeJSON(w, http.StatusOK, pos)
		return
	}
	positions := s.engine.Positions(types.UserID(userID))
	writeJSON(w, http.StatusOK, positions)
}

func (s *Server) handleLeverage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req leverageRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.engine.SetLeverage(types.UserID(req.UserID), req.Symbol, req.Leverage); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.persistence != nil {
		payload := persistence.SetLeveragePayload(types.UserID(req.UserID), req.Symbol, req.Leverage)
		_ = s.persistence.Append(wal.EventSetLeverage, payload)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := parseUint64(r.URL.Query().Get("userId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid userId"})
		return
	}
	balances := s.engine.Balances(types.UserID(userID))
	writeJSON(w, http.StatusOK, balances)
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := parseUint64(r.URL.Query().Get("userId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid userId"})
		return
	}
	asset := r.URL.Query().Get("asset")
	if asset == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset required"})
		return
	}
	balance := s.engine.Balance(types.UserID(userID), asset)
	writeJSON(w, http.StatusOK, balance)
}

func (s *Server) handleTradingDynamic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/trading/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	categoryRaw := parts[0]
	symbol := parts[1]
	resource := parts[2]
	category, err := parseCategory(categoryRaw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category"})
		return
	}

	switch resource {
	case "instrument":
		inst := s.registry.GetInstrument(symbol)
		if inst == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "symbol not found"})
			return
		}
		writeJSON(w, http.StatusOK, inst)
	case "trades":
		userID, err := parseUint64(r.URL.Query().Get("userId"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid userId"})
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		trades := s.engine.TradeHistory(types.UserID(userID), symbol, category, limit)
		out := make([]TradeResponse, 0, len(trades))
		for i := range trades {
			trade := trades[i]
			out = append(out, tradeToResponse(&trade))
		}
		writeJSON(w, http.StatusOK, out)
	case "orders":
		if len(parts) >= 4 && parts[3] == "history" {
			userID, err := parseUint64(r.URL.Query().Get("userId"))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid userId"})
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			orders := s.engine.OrderHistory(types.UserID(userID), symbol, category, limit)
			out := make([]OrderResponse, 0, len(orders))
			for i := range orders {
				order := orders[i]
				out = append(out, orderToResponse(&order))
			}
			writeJSON(w, http.StatusOK, out)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
