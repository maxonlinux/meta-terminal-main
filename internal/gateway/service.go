package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	JWTSecret    string
	JWTCookie    string
}

type Portfolio interface {
	GetBalance(userID types.UserID, asset string) *types.UserBalance
	GetBalances(userID types.UserID) []*types.UserBalance
	GetPositions(userID types.UserID) []*types.Position
	GetPosition(userID types.UserID, symbol string) *types.Position
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

type Instruments interface {
	GetInstruments() []*types.Instrument
	GetInstrument(symbol string) *types.Instrument
}

type Service struct {
	cfg         Config
	httpSrv     *http.Server
	oms         *oms.Service
	portfolio   Portfolio
	history     HistoryService
	instruments Instruments

	clients map[int]*wsClient
	mu      sync.RWMutex
	nextID  int

	jwtSecret []byte
	jwtCookie string
}

type wsClient struct {
	id     int
	symbol string
	socket *websocket.Conn
	send   chan []byte
	mu     sync.Mutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func New(cfg Config, omsService *oms.Service, portfolio Portfolio, history HistoryService, instruments Instruments) *Service {
	cookieName := cfg.JWTCookie
	if cookieName == "" {
		cookieName = "token"
	}
	return &Service{
		cfg:         cfg,
		oms:         omsService,
		portfolio:   portfolio,
		history:     history,
		instruments: instruments,
		clients:     make(map[int]*wsClient),
		jwtSecret:   []byte(cfg.JWTSecret),
		jwtCookie:   cookieName,
	}
}

func (s *Service) Start(ctx context.Context) error {
	handler := s.handler()
	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: handler,
	}

	go func() {
		log.Printf("gateway listening on :%d", s.cfg.Port)
		if err := s.httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("gateway error: %v", err)
		}
	}()

	return nil
}

func (s *Service) Handler() http.Handler {
	return s.handler()
}

func (s *Service) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/order", s.handlePlaceOrder)
	mux.HandleFunc("/api/v1/order/", s.handleCancelOrder)
	mux.HandleFunc("/api/v1/orders", s.handleGetOrders)
	mux.HandleFunc("/api/v1/open-orders", s.handleGetOpenOrders)
	mux.HandleFunc("/api/v1/orders-history", s.handleGetOrdersHistory)
	mux.HandleFunc("/api/v1/balance", s.handleGetBalance)
	mux.HandleFunc("/api/v1/balances", s.handleGetBalances)
	mux.HandleFunc("/api/v1/positions", s.handleGetPositions)
	mux.HandleFunc("/api/v1/positions/leverage", s.handleSetLeverage)
	mux.HandleFunc("/api/v1/orderbook", s.handleGetOrderbook)
	mux.HandleFunc("/api/v1/trades", s.handleGetTrades)
	mux.HandleFunc("/api/v1/closed-pnl", s.handleGetClosedPNL)
	mux.HandleFunc("/api/v1/position", s.handleGetPosition)
	mux.HandleFunc("/api/v1/instruments", s.handleGetInstruments)
	mux.HandleFunc("/api/v1/instrument", s.handleGetInstrument)

	mux.HandleFunc("/ws", s.handleWebSocket)
	return s.authMiddleware(mux)
}

func (s *Service) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/orderbook" || r.URL.Path == "/api/v1/instruments" || r.URL.Path == "/api/v1/instrument" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()

		userID, err := s.extractUserID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx = context.WithValue(ctx, UserIDKey, types.UserID(userID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) extractUserID(r *http.Request) (types.UserID, error) {
	if len(s.jwtSecret) == 0 {
		return 0, errors.New("jwt secret not configured")
	}

	if token, ok := s.tokenFromCookie(r); ok {
		userID, err := parseJWTUserID(token, s.jwtSecret)
		if err == nil {
			return types.UserID(userID), nil
		}
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return 0, errors.New("missing authorization")
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(auth, bearerPrefix) {
		return 0, errors.New("invalid authorization header")
	}
	token := strings.TrimSpace(auth[len(bearerPrefix):])
	userID, err := parseJWTUserID(token, s.jwtSecret)
	if err != nil {
		return 0, err
	}
	return types.UserID(userID), nil
}

func (s *Service) Stop() {
	if s.httpSrv != nil {
		_ = s.httpSrv.Shutdown(context.Background())
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

	writeJSON(w, filterOrdersBySymbol(orders, r.URL.Query().Get("symbol")))
}

func (s *Service) handleGetOpenOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	orders := s.oms.GetOrders(userID)
	if orders == nil {
		writeJSON(w, []*types.Order{})
		return
	}
	openOrders := filterOpenOrders(orders)
	writeJSON(w, filterOrdersBySymbol(openOrders, r.URL.Query().Get("symbol")))
}

func (s *Service) handleGetOrdersHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	symbol := r.URL.Query().Get("symbol")
	limit := parseLimit(r, 100)

	orders, err := s.history.GetOrderHistory(r.Context(), userID, symbol, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func (s *Service) handleGetBalances(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	balances := s.portfolio.GetBalances(userID)
	writeJSON(w, balances)
}

func (s *Service) handleGetPositions(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	positions := s.portfolio.GetPositions(userID)
	writeJSON(w, positions)
}

func (s *Service) handleGetPosition(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	position := s.portfolio.GetPosition(userID, symbol)
	if position == nil {
		http.Error(w, "position not found", http.StatusNotFound)
		return
	}
	writeJSON(w, position)
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
	symbol, category, err := parseOrderbookQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	limit, err := parseOrderbookLimit(r, category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bidPrices, bidQtys, askPrices, askQtys := s.oms.GetOrderBookDepth(category, symbol, limit)
	response := map[string]interface{}{
		"s":   symbol,
		"b":   zipDepth(bidPrices, bidQtys),
		"a":   zipDepth(askPrices, askQtys),
		"ts":  time.Now().UnixMilli(),
		"u":   0,
		"seq": 0,
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

func (s *Service) handleGetClosedPNL(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	limit := parseLimit(r, 100)
	events, err := s.history.GetRPNLHistory(r.Context(), userID, symbol, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}

func (s *Service) handleGetInstruments(w http.ResponseWriter, r *http.Request) {
	_, ok := requireUser(w, r)
	if !ok {
		return
	}

	if s.instruments == nil {
		writeJSON(w, []*types.Instrument{})
		return
	}
	writeJSON(w, s.instruments.GetInstruments())
}

func (s *Service) handleGetInstrument(w http.ResponseWriter, r *http.Request) {
	_, ok := requireUser(w, r)
	if !ok {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}
	if s.instruments == nil {
		http.Error(w, "instrument not found", http.StatusNotFound)
		return
	}
	instrument := s.instruments.GetInstrument(symbol)
	if instrument == nil {
		http.Error(w, "instrument not found", http.StatusNotFound)
		return
	}
	writeJSON(w, instrument)
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
	_ = json.NewEncoder(w).Encode(v)
}

func parseOrderbookQuery(r *http.Request) (string, int8, error) {
	q := r.URL.Query()
	symbol := q.Get("symbol")
	if symbol == "" {
		return "", 0, errors.New("symbol required")
	}
	categoryStr := q.Get("category")
	if categoryStr == "" {
		return "", 0, errors.New("category required")
	}
	switch strings.ToLower(categoryStr) {
	case "spot", "0":
		return symbol, constants.CATEGORY_SPOT, nil
	case "linear", "1":
		return symbol, constants.CATEGORY_LINEAR, nil
	default:
		return "", 0, errors.New("invalid category")
	}
}

func parseLimit(r *http.Request, def int) int {
	limit := def
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	return limit
}

func parseOrderbookLimit(r *http.Request, category int8) (int, error) {
	limitStr := r.URL.Query().Get("limit")
	var limit int
	if limitStr == "" {
		if category == constants.CATEGORY_SPOT {
			return 1, nil
		}
		return 25, nil
	}
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
		return 0, errors.New("invalid limit")
	}
	if category == constants.CATEGORY_SPOT {
		if limit < 1 || limit > 200 {
			return 0, errors.New("limit out of range")
		}
		return limit, nil
	}
	if limit < 1 || limit > 500 {
		return 0, errors.New("limit out of range")
	}
	return limit, nil
}

func (s *Service) tokenFromCookie(r *http.Request) (string, bool) {
	if c, err := r.Cookie(s.jwtCookie); err == nil && c.Value != "" {
		return c.Value, true
	}
	return "", false
}

type jwtClaims struct {
	UserID uint64 `json:"userID"`
	Sub    string `json:"sub"`
	Exp    int64  `json:"exp"`
}

func parseJWTUserID(token string, secret []byte) (uint64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, errors.New("invalid token format")
	}

	payload, err := verifyJWT(parts, secret)
	if err != nil {
		return 0, err
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, errors.New("invalid token payload")
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return 0, errors.New("token expired")
	}
	if claims.UserID > 0 {
		return claims.UserID, nil
	}
	if claims.Sub != "" {
		id, err := strconv.ParseUint(claims.Sub, 10, 64)
		if err != nil {
			return 0, errors.New("invalid sub")
		}
		return id, nil
	}
	return 0, errors.New("missing user id")
}

func verifyJWT(parts []string, secret []byte) ([]byte, error) {
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid token header")
	}
	var headerData struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(header, &headerData); err != nil {
		return nil, errors.New("invalid token header")
	}
	if headerData.Alg != "HS256" {
		return nil, errors.New("unsupported token alg")
	}

	signing := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signing))
	expected := mac.Sum(nil)

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid token signature")
	}
	if !hmac.Equal(signature, expected) {
		return nil, errors.New("invalid token signature")
	}

	return base64.RawURLEncoding.DecodeString(parts[1])
}

func filterOpenOrders(orders []*types.Order) []*types.Order {
	out := make([]*types.Order, 0, len(orders))
	for _, order := range orders {
		if !isOrderClosed(order.Status) {
			out = append(out, order)
		}
	}
	return out
}

func isOrderClosed(status int8) bool {
	switch status {
	case constants.ORDER_STATUS_FILLED,
		constants.ORDER_STATUS_CANCELED,
		constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED,
		constants.ORDER_STATUS_DEACTIVATED:
		return true
	default:
		return false
	}
}

func filterOrdersBySymbol(orders []*types.Order, symbol string) []*types.Order {
	if symbol == "" {
		return orders
	}
	out := make([]*types.Order, 0, len(orders))
	for _, order := range orders {
		if order.Symbol == symbol {
			out = append(out, order)
		}
	}
	return out
}

func zipDepth(prices []types.Price, qtys []types.Quantity) [][]string {
	n := len(prices)
	if len(qtys) < n {
		n = len(qtys)
	}
	out := make([][]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, []string{strconv.FormatInt(int64(prices[i]), 10), strconv.FormatInt(int64(qtys[i]), 10)})
	}
	return out
}

func (s *Service) readPump(client *wsClient, conn *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, client.id)
		s.mu.Unlock()
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetReadLimit(512 * 1024)
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
	_ = json.Unmarshal(data, &sub)
	client.symbol = sub.Symbol
}

func (s *Service) handleUnsubscribe(client *wsClient, data json.RawMessage) {
	client.symbol = ""
}

func (s *Service) writePump(client *wsClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = client.socket.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			_ = client.socket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = client.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			client.mu.Lock()
			err := client.socket.WriteMessage(websocket.TextMessage, message)
			client.mu.Unlock()
			if err != nil {
				return
			}

		case <-ticker.C:
			_ = client.socket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
