package ws

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/api/shared"
	"github.com/maxonlinux/meta-terminal-go/internal/auth"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type WsHandler struct {
	hub        *wsHub
	jwtService *auth.JWTService
	// cookieName selects the JWT cookie for websocket auth.
	cookieName string
}

func NewWsHandler(readBook func(category int8, symbol string) *orderbook.OrderBook, jwtService *auth.JWTService, cookieName string) *WsHandler {
	hub := newWsHub(readBook)
	return &WsHandler{hub: hub, jwtService: jwtService, cookieName: cookieName}
}

func (h *WsHandler) Publisher() engine.EventPublisher {
	return h.hub
}

func (h *WsHandler) Market(c *echo.Context) error {
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	client := newWsConn(conn)

	go h.hub.readMarketLoop(client)
	return nil
}

func (h *WsHandler) Events(c *echo.Context) error {
	claims, err := getClaims(c, h.jwtService, h.cookieName)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	client := newWsConn(conn)
	userID := claims.UserID
	if !h.hub.subscribeUser(userID, client) {
		_ = conn.Close()
		return nil
	}
	go h.hub.readUserLoop(userID, client)
	return nil
}

func getClaims(c *echo.Context, jwtService *auth.JWTService, cookieName string) (*auth.Claims, error) {
	cookie, err := c.Request().Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	validated, err := jwtService.ValidateToken(cookie.Value)
	if err != nil {
		return nil, err
	}
	return validated, nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsHub struct {
	mu        sync.RWMutex
	userSubs  map[types.UserID]map[*wsConn]struct{}
	topicSubs map[string]map[*wsConn]*topicSub
	seq       map[string]uint64
	bookCache map[string]map[int]bookSnapshot
	readBook  func(category int8, symbol string) *orderbook.OrderBook
}

type topicSub struct {
	depth int
}

type bookSnapshot struct {
	bids map[string]string
	asks map[string]string
}

type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func newWsConn(conn *websocket.Conn) *wsConn {
	return &wsConn{conn: conn}
}

func newWsHub(readBook func(category int8, symbol string) *orderbook.OrderBook) *wsHub {
	return &wsHub{
		userSubs:  make(map[types.UserID]map[*wsConn]struct{}),
		topicSubs: make(map[string]map[*wsConn]*topicSub),
		seq:       make(map[string]uint64),
		bookCache: make(map[string]map[int]bookSnapshot),
		readBook:  readBook,
	}
}

func (h *wsHub) subscribeUser(userID types.UserID, conn *wsConn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.userSubs[userID]
	if set == nil {
		set = make(map[*wsConn]struct{})
		h.userSubs[userID] = set
	}
	set[conn] = struct{}{}
	return true
}

func (h *wsHub) unsubscribeUser(userID types.UserID, conn *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.userSubs[userID]
	if set == nil {
		return
	}
	delete(set, conn)
	if len(set) == 0 {
		delete(h.userSubs, userID)
	}
}

func (h *wsHub) subscribeTopic(topic string, conn *wsConn, depth int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.topicSubs[topic]
	if set == nil {
		set = make(map[*wsConn]*topicSub)
		h.topicSubs[topic] = set
	}
	if depth <= 0 {
		depth = 50
	}
	set[conn] = &topicSub{depth: depth}
}

func (h *wsHub) storeBookSnapshot(topic string, depth int, snap bookSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	byDepth := h.bookCache[topic]
	if byDepth == nil {
		byDepth = make(map[int]bookSnapshot)
		h.bookCache[topic] = byDepth
	}
	byDepth[depth] = snap
}

func (h *wsHub) loadBookSnapshot(topic string, depth int) (bookSnapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	byDepth := h.bookCache[topic]
	if byDepth == nil {
		return bookSnapshot{}, false
	}
	snap, ok := byDepth[depth]
	return snap, ok
}

func buildBookSnapshot(snap orderbook.Snapshot) bookSnapshot {
	bids := make(map[string]string, len(snap.Bids))
	for _, lvl := range snap.Bids {
		bids[lvl.Price.String()] = lvl.Total.String()
	}
	asks := make(map[string]string, len(snap.Asks))
	for _, lvl := range snap.Asks {
		asks[lvl.Price.String()] = lvl.Total.String()
	}
	return bookSnapshot{bids: bids, asks: asks}
}

func diffBookSide(prev map[string]string, next map[string]string) [][2]string {
	updates := make([][2]string, 0)
	for price, qty := range next {
		if prevQty, ok := prev[price]; !ok || prevQty != qty {
			updates = append(updates, [2]string{price, qty})
		}
	}
	for price := range prev {
		if _, ok := next[price]; !ok {
			updates = append(updates, [2]string{price, "0"})
		}
	}
	return updates
}

func (h *wsHub) unsubscribeTopic(topic string, conn *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.topicSubs[topic]
	if set == nil {
		return
	}
	delete(set, conn)
	if len(set) == 0 {
		delete(h.topicSubs, topic)
	}
}

func (h *wsHub) unsubscribeAll(conn *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for topic, set := range h.topicSubs {
		delete(set, conn)
		if len(set) == 0 {
			delete(h.topicSubs, topic)
		}
	}
	for userID, set := range h.userSubs {
		delete(set, conn)
		if len(set) == 0 {
			delete(h.userSubs, userID)
		}
	}
}

func (h *wsHub) nextSeq(topic string) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	seq := h.seq[topic] + 1
	h.seq[topic] = seq
	return seq
}

func (h *wsHub) readMarketLoop(conn *wsConn) {
	defer func() {
		h.unsubscribeAll(conn)
		_ = conn.conn.Close()
	}()

	for {
		messageType, data, err := conn.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		if string(data) == "ping" {
			conn.writeMessage([]byte("pong"))
			continue
		}

		var msg wsMarketMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Op {
		case "subscribe":
			h.handleSubscribe(conn, msg)
		case "unsubscribe":
			if msg.Topic != "" {
				h.unsubscribeTopic(msg.Topic, conn)
			}
		}
	}
}

func (h *wsHub) readUserLoop(userID types.UserID, conn *wsConn) {
	defer func() {
		h.unsubscribeUser(userID, conn)
		h.unsubscribeAll(conn)
		_ = conn.conn.Close()
	}()

	for {
		messageType, data, err := conn.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		if string(data) == "ping" {
			conn.writeMessage([]byte("pong"))
		}
	}
}

type wsMarketMessage struct {
	Op    string `json:"op"`
	Topic string `json:"topic"`
	Depth int    `json:"depth"`
}

func (h *wsHub) handleSubscribe(conn *wsConn, msg wsMarketMessage) {
	if msg.Topic == "" {
		return
	}
	if strings.HasPrefix(msg.Topic, "orderbook:") {
		h.subscribeTopic(msg.Topic, conn, msg.Depth)
		h.sendOrderbookSnapshot(conn, msg.Topic, msg.Depth)
		return
	}
	if strings.HasPrefix(msg.Topic, "trades:") {
		h.subscribeTopic(msg.Topic, conn, 0)
	}
}

func (h *wsHub) sendOrderbookSnapshot(conn *wsConn, topic string, depth int) {
	category, symbol, ok := parseMarketTopic(topic, "orderbook")
	if !ok {
		return
	}
	book := h.readBook(category, symbol)
	if book == nil {
		return
	}
	if depth <= 0 {
		depth = 50
	}
	snap := book.Snapshot(depth)
	seq := h.nextSeq(topic)
	cache := buildBookSnapshot(snap)

	bids := make([][2]string, 0, len(snap.Bids))
	for _, lvl := range snap.Bids {
		bids = append(bids, [2]string{lvl.Price.String(), lvl.Total.String()})
	}
	asks := make([][2]string, 0, len(snap.Asks))
	for _, lvl := range snap.Asks {
		asks = append(asks, [2]string{lvl.Price.String(), lvl.Total.String()})
	}

	payload := map[string]interface{}{
		"event": "orderbook.snapshot",
		"data": map[string]interface{}{
			"s":   symbol,
			"b":   bids,
			"a":   asks,
			"ts":  time.Now().UnixMilli(),
			"u":   seq,
			"seq": seq,
			"cts": time.Now().UnixMilli(),
		},
	}
	conn.writeJSON(payload)
	h.storeBookSnapshot(topic, depth, cache)
}

func (h *wsHub) publishOrderbook(topic string) {
	category, symbol, ok := parseMarketTopic(topic, "orderbook")
	if !ok {
		return
	}

	h.mu.RLock()
	subs := h.topicSubs[topic]
	h.mu.RUnlock()
	if len(subs) == 0 {
		return
	}

	byDepth := make(map[int][]*wsConn)
	for conn, sub := range subs {
		depth := sub.depth
		if depth <= 0 {
			depth = 50
		}
		byDepth[depth] = append(byDepth[depth], conn)
	}

	for depth, conns := range byDepth {
		book := h.readBook(category, symbol)
		if book == nil {
			continue
		}
		if depth <= 0 {
			depth = 50
		}
		snap := book.Snapshot(depth)
		nextSnapshot := buildBookSnapshot(snap)
		prevSnapshot, ok := h.loadBookSnapshot(topic, depth)
		if !ok {
			seq := h.nextSeq(topic)
			bids := make([][2]string, 0, len(snap.Bids))
			for _, lvl := range snap.Bids {
				bids = append(bids, [2]string{lvl.Price.String(), lvl.Total.String()})
			}
			asks := make([][2]string, 0, len(snap.Asks))
			for _, lvl := range snap.Asks {
				asks = append(asks, [2]string{lvl.Price.String(), lvl.Total.String()})
			}
			payload := map[string]interface{}{
				"event": "orderbook.snapshot",
				"data": map[string]interface{}{
					"s":   symbol,
					"b":   bids,
					"a":   asks,
					"ts":  time.Now().UnixMilli(),
					"u":   seq,
					"seq": seq,
					"cts": time.Now().UnixMilli(),
				},
			}
			for _, conn := range conns {
				conn.writeJSON(payload)
			}
			h.storeBookSnapshot(topic, depth, nextSnapshot)
			continue
		}

		bidsDelta := diffBookSide(prevSnapshot.bids, nextSnapshot.bids)
		asksDelta := diffBookSide(prevSnapshot.asks, nextSnapshot.asks)
		if len(bidsDelta) == 0 && len(asksDelta) == 0 {
			continue
		}
		seq := h.nextSeq(topic)
		payload := map[string]interface{}{
			"event": "orderbook.delta",
			"data": map[string]interface{}{
				"s":   symbol,
				"b":   bidsDelta,
				"a":   asksDelta,
				"ts":  time.Now().UnixMilli(),
				"u":   seq,
				"seq": seq,
				"cts": time.Now().UnixMilli(),
			},
		}
		for _, conn := range conns {
			conn.writeJSON(payload)
		}
		h.storeBookSnapshot(topic, depth, nextSnapshot)
	}
}

func (h *wsHub) publishTrades(topic string, trades []types.Trade) {
	if len(trades) == 0 {
		return
	}
	category, symbol, ok := parseMarketTopic(topic, "trades")
	if !ok {
		return
	}
	items := make([]map[string]interface{}, 0, len(trades))
	for _, t := range trades {
		items = append(items, map[string]interface{}{
			"price": t.Price.String(),
			"qty":   t.Quantity.String(),
			"side":  shared.SideToString(t.Side),
			"ts":    int64(t.Timestamp),
		})
	}
	payload := map[string]interface{}{
		"event": "trades",
		"data": map[string]interface{}{
			"s":        symbol,
			"category": shared.CategoryToString(category),
			"trades":   items,
		},
	}
	for conn := range h.topicSubs[topic] {
		conn.writeJSON(payload)
	}
}

func (h *wsHub) writeToUser(userID types.UserID, payload map[string]interface{}) {
	h.mu.RLock()
	set := h.userSubs[userID]
	for conn := range set {
		conn.writeJSON(payload)
	}
	h.mu.RUnlock()
}

func (h *wsHub) OnOrderUpdated(order *types.Order) {
	if order == nil || order.Origin == constants.ORDER_ORIGIN_SYSTEM {
		return
	}
	payload := map[string]interface{}{
		"event": "orders",
		"data": map[string]interface{}{
			"orders": []map[string]interface{}{{
				"orderId": strconv.FormatInt(order.ID, 10),
				"status":  shared.OrderStatusToString(order.Status),
			}},
		},
	}
	h.writeToUser(order.UserID, payload)

	if order.Symbol != "" {
		topic := "orderbook:" + shared.CategoryToString(order.Category) + ":" + order.Symbol
		h.publishOrderbook(topic)
	}
}

func (h *wsHub) OnBalanceUpdated(userID types.UserID, asset string, balance *types.Balance) {
	if balance == nil {
		return
	}
	payload := map[string]interface{}{
		"event": "balances",
		"data": map[string]interface{}{
			"asset":     asset,
			"available": balance.Available.String(),
			"locked":    balance.Locked.String(),
			"margin":    balance.Margin.String(),
		},
	}
	h.writeToUser(userID, payload)
}

func (h *wsHub) OnPublicTrades(category int8, symbol string, trades []types.Trade) {
	if symbol == "" || len(trades) == 0 {
		return
	}
	topic := "trades:" + shared.CategoryToString(category) + ":" + symbol
	h.publishTrades(topic, trades)
}

func (h *wsHub) OnOrderbookUpdated(category int8, symbol string) {
	if symbol == "" {
		return
	}
	topic := "orderbook:" + shared.CategoryToString(category) + ":" + symbol
	h.publishOrderbook(topic)
}

func (h *wsHub) OnLiquidation(event engine.LiquidationEvent) {
	payload := map[string]interface{}{
		"event": "liquidation",
		"data": map[string]interface{}{
			"symbol": event.Symbol,
			"stage":  event.Stage,
			"price":  event.Price.String(),
			"size":   event.Size.String(),
		},
	}
	h.writeToUser(event.UserID, payload)
}

func parseMarketTopic(topic string, prefix string) (int8, string, bool) {
	parts := strings.Split(topic, ":")
	if len(parts) != 3 {
		return 0, "", false
	}
	if parts[0] != prefix {
		return 0, "", false
	}
	category, err := shared.ParseCategoryParam(parts[1])
	if err != nil {
		return 0, "", false
	}
	symbol := parts[2]
	if symbol == "" {
		return 0, "", false
	}
	return category, symbol, true
}

func (c *wsConn) writeJSON(payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	c.writeMessage(data)
}

func (c *wsConn) writeMessage(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.WriteMessage(websocket.TextMessage, data)
}
