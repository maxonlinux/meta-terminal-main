package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"nhooyr.io/websocket"
)

type Hub struct {
	mu            sync.RWMutex
	books         *orderbook.State
	conns         map[*websocket.Conn]*connState
	orderbookSubs map[string]map[*websocket.Conn]struct{}
	orderSubs     map[types.UserID]map[*websocket.Conn]struct{}
}

type connState struct {
	orderbookSubs map[string]struct{}
	orderSubs     map[types.UserID]struct{}
}

type subscribeMessage struct {
	Subscribe   string `json:"subscribe"`
	Unsubscribe string `json:"unsubscribe"`
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	UserID      uint64 `json:"userId"`
}

func NewHub(books *orderbook.State) *Hub {
	return &Hub{
		books:         books,
		conns:         make(map[*websocket.Conn]*connState),
		orderbookSubs: make(map[string]map[*websocket.Conn]struct{}),
		orderSubs:     make(map[types.UserID]map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	ctx := context.Background()
	h.mu.Lock()
	h.conns[conn] = &connState{
		orderbookSubs: make(map[string]struct{}),
		orderSubs:     make(map[types.UserID]struct{}),
	}
	h.mu.Unlock()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			h.removeConn(conn)
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		}
		var msg subscribeMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = h.writeJSON(conn, map[string]string{"error": "invalid message"})
			continue
		}
		if msg.Subscribe != "" {
			h.subscribe(conn, msg)
			continue
		}
		if msg.Unsubscribe != "" {
			h.unsubscribe(conn, msg)
			continue
		}
	}
}

func (h *Hub) OnOrderUpdate(order *types.Order) {
	if order == nil {
		return
	}
	h.broadcastOrder(order)
	h.broadcastOrderbook(order.Symbol, order.Category)
}

func (h *Hub) OnTrade(trade *types.Trade) {
	if trade == nil {
		return
	}
	h.broadcastTrade(trade)
}

func (h *Hub) subscribe(conn *websocket.Conn, msg subscribeMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state := h.conns[conn]
	if state == nil {
		return
	}
	switch strings.ToLower(msg.Subscribe) {
	case "orderbook":
		category := strings.ToUpper(msg.Category)
		if category != "SPOT" && category != "LINEAR" {
			_ = h.writeJSON(conn, map[string]string{"error": "invalid category"})
			return
		}
		key := category + ":" + msg.Symbol
		if h.orderbookSubs[key] == nil {
			h.orderbookSubs[key] = make(map[*websocket.Conn]struct{})
		}
		h.orderbookSubs[key][conn] = struct{}{}
		state.orderbookSubs[key] = struct{}{}
		_ = h.writeJSON(conn, map[string]any{"type": "subscribed", "channel": "orderbook", "topic": key})
		h.sendOrderbookSnapshot(conn, msg.Symbol, parseCategoryKey(category))
	case "orders":
		if msg.UserID == 0 {
			_ = h.writeJSON(conn, map[string]string{"error": "userId required"})
			return
		}
		uid := types.UserID(msg.UserID)
		if h.orderSubs[uid] == nil {
			h.orderSubs[uid] = make(map[*websocket.Conn]struct{})
		}
		h.orderSubs[uid][conn] = struct{}{}
		state.orderSubs[uid] = struct{}{}
		_ = h.writeJSON(conn, map[string]any{"type": "subscribed", "channel": "orders", "topic": msg.UserID})
	default:
		_ = h.writeJSON(conn, map[string]string{"error": "invalid subscription"})
	}
}

func (h *Hub) unsubscribe(conn *websocket.Conn, msg subscribeMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state := h.conns[conn]
	if state == nil {
		return
	}
	switch strings.ToLower(msg.Unsubscribe) {
	case "orderbook":
		category := strings.ToUpper(msg.Category)
		key := category + ":" + msg.Symbol
		delete(state.orderbookSubs, key)
		if subs := h.orderbookSubs[key]; subs != nil {
			delete(subs, conn)
			if len(subs) == 0 {
				delete(h.orderbookSubs, key)
			}
		}
		_ = h.writeJSON(conn, map[string]any{"type": "unsubscribed", "channel": "orderbook", "topic": key})
	case "orders":
		uid := types.UserID(msg.UserID)
		delete(state.orderSubs, uid)
		if subs := h.orderSubs[uid]; subs != nil {
			delete(subs, conn)
			if len(subs) == 0 {
				delete(h.orderSubs, uid)
			}
		}
		_ = h.writeJSON(conn, map[string]any{"type": "unsubscribed", "channel": "orders", "topic": msg.UserID})
	default:
		_ = h.writeJSON(conn, map[string]string{"error": "invalid subscription"})
	}
}

func (h *Hub) removeConn(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.conns[conn]
	if state == nil {
		return
	}
	for key := range state.orderbookSubs {
		if subs := h.orderbookSubs[key]; subs != nil {
			delete(subs, conn)
			if len(subs) == 0 {
				delete(h.orderbookSubs, key)
			}
		}
	}
	for uid := range state.orderSubs {
		if subs := h.orderSubs[uid]; subs != nil {
			delete(subs, conn)
			if len(subs) == 0 {
				delete(h.orderSubs, uid)
			}
		}
	}
	delete(h.conns, conn)
}

func (h *Hub) broadcastOrder(order *types.Order) {
	h.mu.RLock()
	subs := h.orderSubs[order.UserID]
	h.mu.RUnlock()
	if len(subs) == 0 {
		return
	}
	msg := map[string]any{
		"type":  "order",
		"order": order,
	}
	h.broadcast(subs, msg)
}

func (h *Hub) broadcastTrade(trade *types.Trade) {
	h.mu.RLock()
	makerSubs := h.orderSubs[trade.MakerID]
	takerSubs := h.orderSubs[trade.TakerID]
	h.mu.RUnlock()

	msg := map[string]any{
		"type":  "trade",
		"trade": trade,
	}
	h.broadcast(makerSubs, msg)
	if trade.MakerID != trade.TakerID {
		h.broadcast(takerSubs, msg)
	}
}

func (h *Hub) broadcastOrderbook(symbol string, category int8) {
	key := categoryKey(category) + ":" + symbol
	h.mu.RLock()
	subs := h.orderbookSubs[key]
	h.mu.RUnlock()
	if len(subs) == 0 {
		return
	}
	book := h.books.Get(symbol, category)
	bids := book.Depth(constants.ORDER_SIDE_BUY, 20)
	asks := book.Depth(constants.ORDER_SIDE_SELL, 20)
	msg := map[string]any{
		"type":     "orderbook",
		"symbol":   symbol,
		"category": categoryKey(category),
		"bids":     depthPairs(bids),
		"asks":     depthPairs(asks),
		"ts":       time.Now().UnixMilli(),
	}
	h.broadcast(subs, msg)
}

func (h *Hub) sendOrderbookSnapshot(conn *websocket.Conn, symbol string, category int8) {
	book := h.books.Get(symbol, category)
	bids := book.Depth(constants.ORDER_SIDE_BUY, 20)
	asks := book.Depth(constants.ORDER_SIDE_SELL, 20)
	msg := map[string]any{
		"type":     "orderbook",
		"symbol":   symbol,
		"category": categoryKey(category),
		"bids":     depthPairs(bids),
		"asks":     depthPairs(asks),
		"ts":       time.Now().UnixMilli(),
	}
	_ = h.writeJSON(conn, msg)
}

func (h *Hub) broadcast(conns map[*websocket.Conn]struct{}, msg any) {
	if len(conns) == 0 {
		return
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for conn := range conns {
		_ = conn.Write(ctx, websocket.MessageText, payload)
	}
}

func (h *Hub) writeJSON(conn *websocket.Conn, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, payload)
}

func categoryKey(category int8) string {
	if category == constants.CATEGORY_LINEAR {
		return "LINEAR"
	}
	return "SPOT"
}

func parseCategoryKey(category string) int8 {
	if strings.ToUpper(category) == "LINEAR" {
		return constants.CATEGORY_LINEAR
	}
	return constants.CATEGORY_SPOT
}

func depthPairs(values []int64) []map[string]string {
	out := make([]map[string]string, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		out = append(out, map[string]string{
			"price": strconv.FormatInt(values[i], 10),
			"qty":   strconv.FormatInt(values[i+1], 10),
		})
	}
	return out
}

var _ engine.EventSink = (*Hub)(nil)
