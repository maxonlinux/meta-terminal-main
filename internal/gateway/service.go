package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/gorilla/websocket"
)

type contextKey string

const UserIDKey contextKey = "userID"

func getUserIDFromContext(ctx context.Context) (types.UserID, bool) {
	userID, ok := ctx.Value(UserIDKey).(types.UserID)
	return userID, ok
}

type Config struct {
	Port         int
	NATSURL      string
	StreamPrefix string
}

type Portfolio interface {
	GetBalance(userID types.UserID, asset string) *types.UserBalance
	GetPositions(userID types.UserID) []*types.Position
	SetLeverage(userID types.UserID, symbol string, leverage int8, currentPrice int64) error
}

type OMS interface {
	PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error)
	GetOrders(userID types.UserID) []*types.Order
	GetOrderBook(category int8, symbol string) (bidPrice types.Price, bidQty types.Quantity, askPrice types.Price, askQty types.Quantity)
	GetOrderBookDepth(category int8, symbol string, limit int) ([]types.Price, []types.Quantity, []types.Price, []types.Quantity)
	GetLastPrice(symbol string) types.Price
}

type HistoryService interface {
	GetOrderHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.Order, error)
	GetTradeHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.Trade, error)
	GetRPNLHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.RPNLEvent, error)
}

type Service struct {
	cfg       Config
	httpSrv   *http.Server
	oms       *oms.Service
	portfolio Portfolio
	history   HistoryService

	clients map[int]*wsClient
	mu      sync.RWMutex
	nextID  int
}

type wsClient struct {
	id     int
	userID types.UserID
	symbol string
	socket *websocket.Conn
	send   chan []byte
	mu     sync.Mutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func New(cfg Config, omsService *oms.Service, portfolio Portfolio, history HistoryService) *Service {
	return &Service{
		cfg:       cfg,
		oms:       omsService,
		portfolio: portfolio,
		history:   history,
		clients:   make(map[int]*wsClient),
	}
}

func (s *Service) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/order", s.handlePlaceOrder)
	mux.HandleFunc("/api/v1/order/", s.handleCancelOrder)
	mux.HandleFunc("/api/v1/orders", s.handleGetOrders)
	mux.HandleFunc("/api/v1/balance", s.handleGetBalance)
	mux.HandleFunc("/api/v1/positions", s.handleGetPositions)
	mux.HandleFunc("/api/v1/positions/leverage", s.handleSetLeverage)
	mux.HandleFunc("/api/v1/orderbook/", s.handleGetOrderbook)
	mux.HandleFunc("/api/v1/trades", s.handleGetTrades)

	mux.HandleFunc("/ws", s.handleWebSocket)

	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: s.authMiddleware(mux),
	}

	go func() {
		log.Printf("gateway listening on :%d", s.cfg.Port)
		if err := s.httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("gateway error: %v", err)
		}
	}()

	return nil
}

func (s *Service) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userID := s.extractUserID(r)
		if userID == 0 {
			log.Printf("gateway: no userID in request, using default 1")
			userID = 1
		}

		ctx = context.WithValue(ctx, UserIDKey, types.UserID(userID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) extractUserID(r *http.Request) types.UserID {
	// Try X-User-ID header first (for testing/development)
	if userIDStr := r.Header.Get("X-User-ID"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
			return types.UserID(userID)
		}
	}

	// TODO: Add JWT validation here in the future
	// For now, just return 0 to use default
	return 0
}

func (s *Service) Stop() {
	if s.httpSrv != nil {
		s.httpSrv.Shutdown(context.Background())
	}
}

func (s *Service) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	var input types.OrderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	input.UserID = userID

	result, err := s.oms.PlaceOrder(r.Context(), &input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, result)
}

func (s *Service) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodDelete) {
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	orderIDStr := r.URL.Path[len("/api/v1/order/"):]
	orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order ID", http.StatusBadRequest)
		return
	}

	if err := s.oms.CancelOrder(r.Context(), userID, types.OrderID(orderID)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleGetOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	_ = r.URL.Query().Get("symbol")

	orders := s.oms.GetOrders(userID)
	if orders == nil {
		writeJSON(w, []*types.Order{})
		return
	}

	writeJSON(w, orders)
}

func (s *Service) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	asset := r.URL.Query().Get("asset")
	if asset == "" {
		asset = "USDT"
	}

	balance := s.portfolio.GetBalance(userID, asset)
	writeJSON(w, balance)
}

func (s *Service) handleGetPositions(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	positions := s.portfolio.GetPositions(userID)
	writeJSON(w, positions)
}

func (s *Service) handleSetLeverage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	var input struct {
		Symbol   string `json:"symbol"`
		Leverage int8   `json:"leverage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	currentPrice := int64(s.oms.GetLastPrice(input.Symbol))

	if err := s.portfolio.SetLeverage(userID, input.Symbol, input.Leverage, currentPrice); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleGetOrderbook(w http.ResponseWriter, r *http.Request) {
	symbol, category := parseOrderbookPath(r.URL.Path)

	bidPrice, bidQty, askPrice, askQty := s.oms.GetOrderBook(category, symbol)

	response := map[string]interface{}{
		"symbol":    symbol,
		"category":  category,
		"bid_price": bidPrice,
		"bid_qty":   bidQty,
		"ask_price": askPrice,
		"ask_qty":   askQty,
	}

	writeJSON(w, response)
}

func (s *Service) handleGetOrderbookDepth(w http.ResponseWriter, r *http.Request) {
	symbol, category := parseOrderbookPath(r.URL.Path)
	limit := parseLimit(r, 10)

	bidPrices, bidQtys, askPrices, askQtys := s.oms.GetOrderBookDepth(category, symbol, limit)

	response := map[string]interface{}{
		"symbol":   symbol,
		"category": category,
		"bids":     map[string]interface{}{"prices": bidPrices, "quantities": bidQtys},
		"asks":     map[string]interface{}{"prices": askPrices, "quantities": askQtys},
	}

	writeJSON(w, response)
}

func (s *Service) handleGetTrades(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	limit := parseLimit(r, 100)

	trades, err := s.history.GetTradeHistory(r.Context(), userID, symbol, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, trades)
}

func (s *Service) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	s.mu.Lock()
	client := &wsClient{
		id:   s.nextID,
		send: make(chan []byte, 256),
	}
	s.nextID++
	s.clients[client.id] = client
	s.mu.Unlock()

	go s.writePump(client)
	s.readPump(client, conn)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func requireUser(w http.ResponseWriter, r *http.Request) (types.UserID, bool) {
	userID, ok := getUserIDFromContext(r.Context())
	if ok {
		return userID, true
	}
	http.Error(w, "user not authenticated", http.StatusUnauthorized)
	return 0, false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func parseOrderbookPath(path string) (string, int8) {
	symbol := path[len("/api/v1/orderbook/"):]
	if len(symbol) >= 4 && symbol[len(symbol)-4:] == "SPOT" {
		return symbol[:len(symbol)-4], constants.CATEGORY_SPOT
	}
	return symbol, constants.CATEGORY_LINEAR
}

func parseLimit(r *http.Request, def int) int {
	limit := def
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	return limit
}

func (s *Service) readPump(client *wsClient, conn *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, client.id)
		s.mu.Unlock()
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetReadLimit(512 * 1024)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			s.handleSubscribe(client, msg.Data)
		case "unsubscribe":
			s.handleUnsubscribe(client, msg.Data)
		}
	}
}

func (s *Service) handleSubscribe(client *wsClient, data json.RawMessage) {
	var sub struct {
		Channel string `json:"channel"`
		Symbol  string `json:"symbol"`
	}
	json.Unmarshal(data, &sub)
	client.symbol = sub.Symbol
}

func (s *Service) handleUnsubscribe(client *wsClient, data json.RawMessage) {
	client.symbol = ""
}

func (s *Service) writePump(client *wsClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.socket.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.socket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			client.mu.Lock()
			err := client.socket.WriteMessage(websocket.TextMessage, message)
			client.mu.Unlock()
			if err != nil {
				return
			}

		case <-ticker.C:
			client.socket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Service) broadcastOrderbook(symbol string, bidPrice, bidQty, askPrice, askQty types.Price) {
	msg := map[string]interface{}{
		"type": "orderbook",
		"data": map[string]interface{}{
			"symbol":    symbol,
			"bid_price": bidPrice,
			"bid_qty":   bidQty,
			"ask_price": askPrice,
			"ask_qty":   askQty,
		},
	}

	data, _ := json.Marshal(msg)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if client.symbol == symbol || client.symbol == "" {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

func (s *Service) broadcastOrderUpdate(order *types.Order) {
	msg := map[string]interface{}{
		"type": "order_update",
		"data": order,
	}

	data, _ := json.Marshal(msg)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if client.symbol == order.Symbol || client.symbol == "" {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

func (s *Service) broadcastTrade(trade *types.Trade) {
	msg := map[string]interface{}{
		"type": "trade",
		"data": trade,
	}

	data, _ := json.Marshal(msg)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if client.symbol == trade.Symbol || client.symbol == "" {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}
