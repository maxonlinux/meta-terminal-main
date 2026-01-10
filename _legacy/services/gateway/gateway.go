package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/id"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

type Config struct {
	NATSURL      string `env:"NATS_URL" default:"nats://localhost:4222"`
	JWTSecret    string `env:"JWT_SECRET" default:"your-secret-key"`
	Port         string `env:"PORT" default:"8080"`
	StreamPrefix string `env:"STREAM_PREFIX" default:"gateway"`
}

type claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Session struct {
	UserID    uint64
	conn      *websocket.Conn
	send      chan []byte
	closed    bool
	sessionMu sync.Mutex
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Service struct {
	cfg      Config
	nats     *messaging.NATS
	sessions map[string]*Session
	mu       sync.RWMutex

	orderSub  *messaging.Subscription
	priceSubs map[string]*messaging.Subscription
}

func New(cfg Config) (*Service, error) {
	n, err := messaging.New(messaging.Config{
		URL:          cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("nats: %w", err)
	}

	s := &Service{
		cfg:       cfg,
		nats:      n,
		sessions:  make(map[string]*Session),
		priceSubs: make(map[string]*messaging.Subscription),
	}

	return s, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.orderSub = s.nats.Subscribe(ctx, "order.event.>", "gateway-orders", s.handleOrderEvent)
	s.nats.Subscribe(ctx, "positions.event.>", "gateway-positions", s.handlePositionEvent)
	s.nats.Subscribe(ctx, "price.tick.>", "gateway-prices", s.handlePriceEvent)

	go s.cleanupSessions(ctx)

	log.Printf("gateway started on port %s", s.cfg.Port)
	return http.ListenAndServe(":"+s.cfg.Port, s.router())
}

func (s *Service) router() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth/login", s.handleLogin)
	mux.HandleFunc("/auth/register", s.handleRegister)
	mux.HandleFunc("/ws", s.handleWebSocket)

	mux.HandleFunc("/api/v1/portfolio", s.authMiddleware(s.handleGetPortfolio))
	mux.HandleFunc("/api/v1/orders", s.authMiddleware(s.handlePlaceOrder))
	mux.HandleFunc("/api/v1/orders/list", s.authMiddleware(s.handleGetOrders))
	mux.HandleFunc("/api/v1/orders/", s.authMiddleware(s.handleGetOrder))

	mux.HandleFunc("/health", s.handleHealth)

	return mux
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "missing credentials", http.StatusBadRequest)
		return
	}

	userID := id.NewUserID()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims{
		UserID:   uint64(userID),
		Username: req.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   tokenString,
		"user_id": userID,
	})
}

func (s *Service) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userID := id.NewUserID()
	s.nats.Publish(r.Context(), messaging.SubjectUserRegistered, map[string]interface{}{
		"user_id":  userID,
		"username": req.Username,
	})

	s.handleLogin(w, r)
}

func (s *Service) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims := &claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	session := &Session{
		UserID: claims.UserID,
		conn:   conn,
		send:   make(chan []byte, 256),
	}

	s.mu.Lock()
	s.sessions[tokenString] = session
	s.mu.Unlock()

	go s.writePump(session)
	s.readPump(session, tokenString)
}

func (s *Service) readPump(session *Session, token string) {
	defer func() {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		session.conn.Close()
	}()

	for {
		_, msg, err := session.conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "subscribe_price":
			if symbol, ok := wsMsg.Data.(string); ok {
				s.subscribePrice(session, symbol)
			}
		case "unsubscribe_price":
			if symbol, ok := wsMsg.Data.(string); ok {
				s.unsubscribePrice(session, symbol)
			}
		case "subscribe_orders":
			s.subscribeOrders(session)
		case "place_order":
			var req map[string]interface{}
			if m, ok := wsMsg.Data.(map[string]interface{}); ok {
				req = m
			} else {
				// try to decode json raw message
				if b, err := json.Marshal(wsMsg.Data); err == nil {
					json.Unmarshal(b, &req)
				}
			}
			if req == nil {
				continue
			}
			symbol, _ := req["symbol"].(string)
			if symbol == "" {
				continue
			}
			// Build types.OrderInput from JSON map
			input := types.OrderInput{
				UserID:   session.UserID,
				Symbol:   symbol,
				Category: toInt8(req["category"]),
				Side:     toInt8(req["side"]),
				Type:     toInt8(req["type"]),
				TIF:      toInt8(req["tif"]),
				Quantity: types.Quantity(toInt64(req["qty"])),
				Price:    types.Price(toInt64(req["price"])),
			}
			if v, ok := req["trigger_price"].(float64); ok {
				input.TriggerPrice = types.Price(v)
			}
			if v, ok := req["reduce_only"].(bool); ok {
				input.ReduceOnly = v
			}
			if v, ok := req["close_on_trigger"].(bool); ok {
				input.CloseOnTrigger = v
			}
			if v, ok := req["leverage"].(float64); ok {
				input.Leverage = int8(v)
			} else {
				input.Leverage = 1
			}
			subject := messaging.OrderPlaceTopic(symbol)
			s.nats.PublishGob(context.Background(), subject, input)
		}
	}
}

func (s *Service) writePump(session *Session) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		session.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-session.send:
			if !ok {
				session.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := session.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := session.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Service) subscribePrice(session *Session, symbol string) {
	session.send <- []byte(fmt.Sprintf(`{"type":"subscribed","symbol":"%s"}`, symbol))
}

func (s *Service) unsubscribePrice(session *Session, symbol string) {
	session.send <- []byte(fmt.Sprintf(`{"type":"unsubscribed","symbol":"%s"}`, symbol))
}

func (s *Service) subscribeOrders(session *Session) {
	session.send <- []byte(fmt.Sprintf(`{"type":"subscribed_orders","user_id":%d}`, session.UserID))
}

func (s *Service) handleOrderEvent(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		select {
		case session.send <- data:
		default:
		}
	}
}

// handleBalanceEvent is deprecated; balance updates are included in position/order events.
// Kept for compatibility, but no subscription is started in Start().
func (s *Service) handleBalanceEvent(data []byte) {
	// no-op
}

func (s *Service) handlePositionEvent(data []byte) {
	var event struct {
		UserID uint64 `json:"user_id"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		// If parse fails, broadcast to all (fallback)
		s.mu.RLock()
		for _, session := range s.sessions {
			select {
			case session.send <- data:
			default:
			}
		}
		s.mu.RUnlock()
		return
	}

	s.mu.RLock()
	for _, session := range s.sessions {
		if session.UserID == event.UserID {
			select {
			case session.send <- data:
			default:
			}
		}
	}
	s.mu.RUnlock()
}

func (s *Service) handlePriceEvent(data []byte) {
	// Broadcast price tick to all connected WS sessions
	// (In production we'd track per-symbol subscriptions per session)
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		select {
		case session.send <- data:
		default:
		}
	}
}

func (s *Service) cleanupSessions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for token, session := range s.sessions {
				session.sessionMu.Lock()
				if session.closed {
					delete(s.sessions, token)
				}
				session.sessionMu.Unlock()
			}
			s.mu.Unlock()
		}
	}
}

func (s *Service) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing auth header", http.StatusUnauthorized)
			return
		}

		tokenString := authHeader[len("Bearer "):]
		claims := &claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(s.cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "user_id", claims.UserID)
		next(w, r.WithContext(ctx))
	}
}

func (s *Service) handleGetPortfolio(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(uint64)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	data := map[string]interface{}{
		"user_id": userID,
	}
	s.nats.Publish(ctx, messaging.SubjectPortfolioQuery, map[string]interface{}{
		"user_id": userID,
	})

	time.Sleep(50 * time.Millisecond)

	json.NewEncoder(w).Encode(data)
}

func (s *Service) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.Context().Value("user_id").(uint64)

	var req struct {
		Symbol      string  `json:"symbol"`
		Category    int8    `json:"category"`
		Side        int8    `json:"side"`
		Type        int8    `json:"type"`
		TIF         int8    `json:"tif"`
		Qty         int64   `json:"qty"`
		Price       int64   `json:"price"`
		Trigger     int64   `json:"trigger,omitempty"`
		ReduceOnly  bool    `json:"reduce_only,omitempty"`
		CloseOnTrigger bool `json:"close_on_trigger,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	// Build OrderInput and publish Gob
	input := types.OrderInput{
		UserID:         types.UserID(userID),
		Symbol:         req.Symbol,
		Category:       req.Category,
		Side:           req.Side,
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       types.Quantity(req.Qty),
		Price:          types.Price(req.Price),
		TriggerPrice:   types.Price(req.Trigger),
		ReduceOnly:     req.ReduceOnly,
		CloseOnTrigger: req.CloseOnTrigger,
		Leverage:       1,
	}
	subject := messaging.OrderPlaceTopic(req.Symbol)
	if err := s.nats.PublishGob(r.Context(), subject, &input); err != nil {
		log.Printf("gateway: publish error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Respond immediately (actual result will come via WS)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "accepted",
		"symbol": req.Symbol,
	})
}
	userID := r.Context().Value("user_id").(uint64)

	var req struct {
		Symbol      string  `json:"symbol"`
		Category    int8    `json:"category"`
		Side        int8    `json:"side"`
		Type        int8    `json:"type"`
		TIF         int8    `json:"tif"`
		Qty         int64   `json:"qty"`
		Price       int64   `json:"price"`
		Trigger     int64   `json:"trigger,omitempty"`
		ReduceOnly  bool    `json:"reduce_only,omitempty"`
		CloseOnTrigger bool `json:"close_on_trigger,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	// Build OrderInput and publish Gob
	input := types.OrderInput{
		UserID:         types.UserID(userID),
		Symbol:         req.Symbol,
		Category:       req.Category,
		Side:           req.Side,
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       types.Quantity(req.Qty),
		Price:          types.Price(req.Price),
		TriggerPrice:   types.Price(req.Trigger),
		ReduceOnly:     req.ReduceOnly,
		CloseOnTrigger: req.CloseOnTrigger,
		Leverage:       1,
	}
	subject := messaging.OrderPlaceTopic(req.Symbol)
	if err := s.nats.PublishGob(r.Context(), subject, &input); err != nil {
		log.Printf("gateway: publish error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Respond immediately (actual result will come via WS)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "accepted",
		"symbol": req.Symbol,
	})
}
	userID := r.Context().Value("user_id").(uint64)

	var req struct {
		Symbol         string `json:"symbol"`
		Category       int8   `json:"category"`
		Side           int8   `json:"side"`
		Type           int8   `json:"type"`
		TIF            int8   `json:"tif"`
		Qty            int64  `json:"qty"`
		Price          int64  `json:"price"`
		Trigger        int64  `json:"trigger,omitempty"`
		ReduceOnly     bool   `json:"reduce_only,omitempty"`
		CloseOnTrigger bool   `json:"close_on_trigger,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	// Create a minimal order structure for OMS
	// In production we would use a proper proto/encoder; using JSON for simplicity
	// Convert to types.OrderInput and publish Gob to OMS
	input := types.OrderInput{
		UserID:         types.UserID(userID),
		Symbol:         req.Symbol,
		Category:       req.Category,
		Side:           req.Side,
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       types.Quantity(req.Qty),
		Price:          types.Price(req.Price),
		TriggerPrice:   types.Price(req.Trigger),
		ReduceOnly:     req.ReduceOnly,
		CloseOnTrigger: req.CloseOnTrigger,
		Leverage:       1,
	}
	subject := messaging.OrderPlaceTopic(req.Symbol)
	s.nats.PublishGob(r.Context(), subject, input)

	// Respond immediately (actual result will come via WS)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "accepted",
		"symbol": req.Symbol,
	})
}

func (s *Service) handleGetOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(uint64)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s.nats.Publish(ctx, messaging.SubjectOMSQuery, map[string]interface{}{
		"user_id": userID,
	})

	time.Sleep(50 * time.Millisecond)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"orders":  []interface{}{},
	})
}

func (s *Service) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(uint64)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s.nats.Publish(ctx, messaging.SubjectOMSQuery, map[string]interface{}{
		"user_id": userID,
	})

	time.Sleep(50 * time.Millisecond)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "not_implemented",
	})
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Service) Close() {
	if s.orderSub != nil {
		s.orderSub.Close()
	}
	for _, sub := range s.priceSubs {
		sub.Close()
	}
	s.nats.Close()
}
