package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/id"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
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
	UserID   id.UserID `json:"user_id"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

type Service struct {
	cfg      Config
	nats     *messaging.NATS
	sessions map[string]*Session
	mu       sync.RWMutex

	orderSub  *messaging.Subscription
	priceSubs map[string]*messaging.Subscription
}

type Session struct {
	UserID id.UserID
	conn   *websocket.Conn
	send   chan []byte
	closed bool
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
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
	s.orderSub = s.nats.Subscribe(ctx, "orders.>", "gateway-orders", s.handleOrderEvent)

	go s.cleanupSessions(ctx)

	log.Printf("gateway started on port %s", s.cfg.Port)
	return http.ListenAndServe(":"+s.cfg.Port, s.router())
}

func (s *Service) router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", s.handleLogin)
	mux.HandleFunc("/auth/register", s.handleRegister)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	mux.HandleFunc("/portfolio", s.authMiddleware(s.handlePortfolio))
	mux.HandleFunc("/orders/place", s.authMiddleware(s.handlePlaceOrder))
	mux.HandleFunc("/orders/cancel", s.authMiddleware(s.handleCancelOrder))
	mux.HandleFunc("/orders/", s.authMiddleware(s.handleGetOrder))
	mux.HandleFunc("/market/depth", s.authMiddleware(s.handleMarketDepth))
	mux.HandleFunc("/market/ticker", s.authMiddleware(s.handleTicker))

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
		UserID:   userID,
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
		"token":    tokenString,
		"user_id":  userID,
		"username": req.Username,
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
	s.nats.Publish(r.Context(), "users.registered", map[string]interface{}{
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

func (s *Service) cleanupSessions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for token, session := range s.sessions {
				if session.closed {
					delete(s.sessions, token)
				}
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

func (s *Service) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(id.UserID)

	var portfolio struct {
		UserID    id.UserID           `json:"user_id"`
		Balances  map[string]int64    `json:"balances"`
		Positions map[string]Position `json:"positions"`
	}
	portfolio.UserID = userID
	portfolio.Balances = make(map[string]int64)
	portfolio.Positions = make(map[string]Position)

	s.nats.Publish(r.Context(), "portfolio.request", map[string]interface{}{
		"user_id": userID,
		"request": "get_portfolio",
	})

	time.Sleep(100 * time.Millisecond)

	json.NewEncoder(w).Encode(portfolio)
}

type Position struct {
	Symbol     string `json:"symbol"`
	Size       int64  `json:"size"`
	EntryPrice int64  `json:"entry_price"`
}

func (s *Service) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(id.UserID)

	var input struct {
		Symbol   string `json:"symbol"`
		Category int8   `json:"category"`
		Side     int8   `json:"side"`
		Type     int8   `json:"type"`
		TIF      int8   `json:"tif"`
		Qty      int64  `json:"qty"`
		Price    int64  `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	orderID := id.NewOrderID()

	s.nats.Publish(r.Context(), "orders."+input.Symbol+".PLACED", map[string]interface{}{
		"order_id":  orderID,
		"user_id":   userID,
		"symbol":    input.Symbol,
		"category":  input.Category,
		"side":      input.Side,
		"type":      input.Type,
		"tif":       input.TIF,
		"qty":       input.Qty,
		"price":     input.Price,
		"timestamp": time.Now().UnixNano(),
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   constants.ORDER_STATUS_NEW,
	})
}

func (s *Service) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(id.UserID)

	var req struct {
		OrderID id.OrderID `json:"order_id"`
		Symbol  string     `json:"symbol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	s.nats.Publish(r.Context(), "orders."+req.Symbol+".CANCELLED", map[string]interface{}{
		"order_id": req.OrderID,
		"user_id":  userID,
		"symbol":   req.Symbol,
	})

	json.NewEncoder(w).Encode(map[string]interface{}{"status": "cancelled"})
}

func (s *Service) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "not_implemented",
	})
}

func (s *Service) handleMarketDepth(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	s.nats.Publish(r.Context(), "orderbook.request", map[string]interface{}{
		"symbol": symbol,
		"depth":  10,
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol": symbol,
		"bids":   []interface{}{},
		"asks":   []interface{}{},
	})
}

func (s *Service) handleTicker(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	s.nats.Publish(r.Context(), "ticker.request", map[string]interface{}{
		"symbol": symbol,
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol":     symbol,
		"price":      0,
		"change_24h": 0,
		"volume":     0,
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
