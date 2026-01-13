package core

import (
	"container/heap"
	"sync"
	"sync/atomic"

	snowflake "github.com/anomalyco/meta-terminal-go/pkg"
)

// Engine - single-threaded event loop, zero locks in hot path.
// WHY: Single-threaded processing eliminates need for mutexes
// WHY: All events processed sequentially, ensuring consistency
type Engine struct {
	ringBuffer   *RingBuffer[Event]
	spotBooks    map[string]*OrderBook // SPOT market order books
	linearBooks  map[string]*OrderBook // LINEAR market order books
	positions    map[UserID]map[string]*Position
	roHeaps      map[UserID]map[string]*ROHeap
	triggers     *TriggerMonitor
	stopCh       chan struct{}
	wg           sync.WaitGroup
	PositionMode int8 // POSITION_MODE_ONE_WAY or POSITION_MODE_HEDGE
}

// Event - represents an event in the event loop
// WHY: Ring buffer carries events for single-threaded processing
type Event struct {
	Type      EventType
	Order     *OrderRequest
	Trade     *Trade
	Position  *PositionUpdate
	PriceTick *PriceTick
	Cancel    *CancelRequest
}

// EventType - type of event in the loop
type EventType int8

const (
	EventOrder          EventType = iota // PlaceOrder event
	EventTrade                           // Trade execution event
	EventPositionUpdate                  // Position change event
	EventPriceTick                       // Price update event
	EventCancel                          // Cancel order event
)

// NewEngine creates a new trading engine with all components wired
func NewEngine() *Engine {
	return &Engine{
		ringBuffer:   NewRingBuffer[Event](1024),
		spotBooks:    make(map[string]*OrderBook),
		linearBooks:  make(map[string]*OrderBook),
		positions:    make(map[UserID]map[string]*Position),
		roHeaps:      make(map[UserID]map[string]*ROHeap),
		triggers:     NewTriggerMonitor(),
		stopCh:       make(chan struct{}),
		PositionMode: POSITION_MODE_ONE_WAY, // Default: One-Way mode (Bybit-compliant)
	}
}

// Start begins the event loop goroutine
func (e *Engine) Start() {
	e.wg.Add(1)
	go e.run()
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

// SetPositionMode configures position mode (One-Way or Hedge)
// WHY: Reduce-only orders only allowed in One-Way mode per Bybit spec
func (e *Engine) SetPositionMode(mode int8) {
	e.PositionMode = mode
}

// PlaceOrder submits an order to the engine
func (e *Engine) PlaceOrder(req *OrderRequest) {
	seq := e.ringBuffer.Next()
	ev := e.ringBuffer.Get(seq)
	ev.Type = EventOrder
	ev.Order = req
	e.ringBuffer.Publish(seq)
}

// CancelOrder submits a cancellation request
func (e *Engine) CancelOrder(orderID OrderID, userID UserID, symbol string, category int8) {
	seq := e.ringBuffer.Next()
	ev := e.ringBuffer.Get(seq)
	ev.Type = EventCancel
	ev.Cancel = &CancelRequest{
		OrderID:  int64(orderID),
		UserID:   uint64(userID),
		Symbol:   symbol,
		Category: category,
	}
	e.ringBuffer.Publish(seq)
}

// OnPriceTick processes a price update
func (e *Engine) OnPriceTick(symbol string, price Price) {
	seq := e.ringBuffer.Next()
	ev := e.ringBuffer.Get(seq)
	ev.Type = EventPriceTick
	ev.PriceTick = &PriceTick{Symbol: symbol, Price: int64(price)}
	e.ringBuffer.Publish(seq)
}

// run is the main event loop - single-threaded processing
func (e *Engine) run() {
	defer e.wg.Done()
	for {
		select {
		case <-e.stopCh:
			return
		default:
			if e.ringBuffer.IsEmpty() {
				continue
			}
			readSeq := atomic.LoadUint64(&e.ringBuffer.read)
			avail := e.ringBuffer.ReadAvailable()

			for i := uint64(0); i < avail; i++ {
				seq := readSeq + i
				ev := e.ringBuffer.Get(seq)

				switch ev.Type {
				case EventOrder:
					if ev.Order != nil {
						e.handleOrder(ev.Order)
					}
				case EventTrade:
					if ev.Trade != nil {
						e.handleTrade(ev.Trade)
					}
				case EventPositionUpdate:
					if ev.Position != nil {
						e.handlePosition(ev.Position)
					}
				case EventPriceTick:
					if ev.PriceTick != nil {
						e.handlePriceTick(ev.PriceTick)
					}
				case EventCancel:
					if ev.Cancel != nil {
						e.handleCancel(ev.Cancel)
					}
				}
				*ev = Event{}
			}
			atomic.StoreUint64(&e.ringBuffer.read, readSeq+avail)
		}
	}
}

// handleOrder processes an incoming order request
func (e *Engine) handleOrder(req *OrderRequest) {
	// Get order book for validation
	ob := e.getOrderBook(req.Symbol, req.Category)

	// Get position for validations
	pos := e.getPosition(UserID(req.UserID), req.Symbol)

	// Get current price for conditional/post-only validation
	var currentPrice Price
	if ob.lastPrice > 0 {
		currentPrice = Price(ob.lastPrice)
	}

	// Validate SPOT vs LINEAR specific rules
	// WHY: SPOT and LINEAR markets have different rules
	switch req.Category {
	case CATEGORY_SPOT:
		if req.ReduceOnly {
			return // Reject: RO not allowed for SPOT
		}
		if req.TriggerPrice > 0 {
			return // Reject: Conditional orders not supported for SPOT
		}
	case CATEGORY_LINEAR:
		if req.Type == ORDER_TYPE_MARKET && req.TIF != TIF_IOC && req.TIF != TIF_FOK {
			return // Reject: Market orders must be IOC or FOK for LINEAR
		}
	}

	// Handle OCO orders
	if req.OCO != nil {
		e.handleOCOOrder(req)
		return
	}

	// Validate reduce-only orders (CloseOnTrigger doesn't require position)
	// WHY: RO only allowed if position exists, CoT can trigger to open new position
	if req.ReduceOnly {
		if pos.Size == 0 {
			return
		}
		if pos.Size > 0 && req.Side != ORDER_SIDE_SELL {
			return
		}
		if pos.Size < 0 && req.Side != ORDER_SIDE_BUY {
			return
		}
	}

	// Validate TriggerPrice direction
	// BUY conditional: triggerPrice < currentPrice (wait for price DROP)
	// SELL conditional: triggerPrice > currentPrice (wait for price RISE)
	if req.TriggerPrice > 0 {
		if currentPrice == 0 {
			return // Reject: No price data for conditional validation
		}
		if req.Side == ORDER_SIDE_BUY && Price(req.TriggerPrice) >= currentPrice {
			return // Reject: BUY trigger must be below current
		}
		if req.Side == ORDER_SIDE_SELL && Price(req.TriggerPrice) <= currentPrice {
			return // Reject: SELL trigger must be above current
		}
	}

	// Validate Post-Only orders
	// WHY: Post-Only should NOT match immediately
	if req.TIF == TIF_POST_ONLY {
		if req.Side == ORDER_SIDE_BUY {
			if bestAsk := ob.BestAsk(); bestAsk != nil && Price(req.Price) >= bestAsk.Price {
				return
			}
		} else {
			if bestBid := ob.BestBid(); bestBid != nil && Price(req.Price) <= bestBid.Price {
				return
			}
		}
	}

	// Generate order ID using snowflake (always auto-generated)
	orderID := OrderID(snowflake.Next())

	// Create order
	order := &Order{
		ID:             orderID,
		UserID:         UserID(req.UserID),
		Symbol:         req.Symbol,
		Category:       req.Category,
		Side:           req.Side,
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       Quantity(req.Quantity),
		Price:          Price(req.Price),
		ReduceOnly:     req.ReduceOnly,
		CloseOnTrigger: req.CloseOnTrigger,
		Status:         ORDER_STATUS_NEW,
		TriggerPrice:   Price(req.TriggerPrice),
		CreatedAt:      NowNano(),
	}

	// For conditional orders (TriggerPrice > 0), add to trigger monitor
	// WHY: Conditional orders wait in trigger monitor, not in orderbook
	if req.TriggerPrice > 0 {
		order.IsConditional = true
		order.Status = ORDER_STATUS_UNTRIGGERED
		e.triggers.Add(order)

		// If RO or CoT order, add to trimming heap (conditional RO/CoT)
		if req.ReduceOnly || req.CloseOnTrigger {
			e.addROOrder(order)
		}
		return
	}

	// Add order to book (matching happens inside AddOrder)
	// WHY: AddOrder returns all trades created during matching
	trades := ob.AddOrder(order)

	// If RO or CoT order, add to trimming heap
	if req.ReduceOnly || req.CloseOnTrigger {
		e.addROOrder(order)
	}

	// Handle trades
	for _, trade := range trades {
		e.handleTrade(trade)
	}

	// Handle TIF for remaining quantity
	if order.Quantity > 0 {
		switch order.TIF {
		case TIF_IOC:
			// Cancel remaining - trades already executed
			ob.RemoveOrder(order.ID)
		case TIF_FOK:
			// FOK failed - cancel entire order
			ob.RemoveOrder(order.ID)
		}
	}
}

// handleOCOOrder processes OCO (One-Cancels-the-Other) orders
// WHY: OCO creates two linked conditional orders (Take Profit + Stop Loss)
func (e *Engine) handleOCOOrder(req *OrderRequest) {
	// OCO only for LINEAR
	if req.Category != CATEGORY_LINEAR {
		return
	}

	// Check position exists
	pos := e.getPosition(UserID(req.UserID), req.Symbol)
	if pos.Size == 0 {
		return // Reject: OCO requires existing position
	}

	// Determine qty (0 = use full position size)
	ocoQty := Quantity(req.Quantity)
	if ocoQty == 0 {
		ocoQty = absQuantity(pos.Size)
	}

	// Validate TP/SL trigger prices based on position side
	switch pos.Side {
	case SIDE_LONG:
		// LONG: TP trigger > SL trigger
		if req.OCO.TakeProfit.TriggerPrice <= req.OCO.StopLoss.TriggerPrice {
			return // Reject: Invalid TP/SL for LONG
		}
	case SIDE_SHORT:
		// SHORT: TP trigger < SL trigger
		if req.OCO.TakeProfit.TriggerPrice >= req.OCO.StopLoss.TriggerPrice {
			return // Reject: Invalid TP/SL for SHORT
		}
	}

	// Generate group ID for linked orders
	groupID := snowflake.Next()

	// Create Take Profit order
	tpOrder := &Order{
		ID:             OrderID(snowflake.Next()),
		UserID:         UserID(req.UserID),
		Symbol:         req.Symbol,
		Category:       req.Category,
		Side:           ORDER_SIDE_SELL, // OCO always SELL to close
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       ocoQty,
		Price:          Price(req.OCO.TakeProfit.Price),
		ReduceOnly:     true,
		CloseOnTrigger: true,
		Status:         ORDER_STATUS_UNTRIGGERED,
		TriggerPrice:   Price(req.OCO.TakeProfit.TriggerPrice),
		StopOrderType:  STOP_ORDER_TYPE_TAKE_PROFIT,
		OrderLinkId:    groupID,
		IsConditional:  true,
		CreatedAt:      NowNano(),
	}
	e.triggers.Add(tpOrder)

	// Create Stop Loss order
	slOrder := &Order{
		ID:             OrderID(snowflake.Next()),
		UserID:         UserID(req.UserID),
		Symbol:         req.Symbol,
		Category:       req.Category,
		Side:           ORDER_SIDE_SELL, // OCO always SELL to close
		Type:           req.Type,
		TIF:            req.TIF,
		Quantity:       ocoQty,
		Price:          Price(req.OCO.StopLoss.Price),
		ReduceOnly:     true,
		CloseOnTrigger: true,
		Status:         ORDER_STATUS_UNTRIGGERED,
		TriggerPrice:   Price(req.OCO.StopLoss.TriggerPrice),
		StopOrderType:  STOP_ORDER_TYPE_STOP_LOSS,
		OrderLinkId:    groupID,
		IsConditional:  true,
		CreatedAt:      NowNano(),
	}
	e.triggers.Add(slOrder)
}

// handleTrade processes a trade and updates position
func (e *Engine) handleTrade(trade *Trade) {
	pos := e.getPosition(trade.TakerOrder.UserID, trade.Symbol)

	makerSide := trade.MakerOrder.Side

	qty := trade.Quantity

	if pos.Size == 0 {
		pos.Size = qty
		pos.Side = makerSide
	} else if pos.Side == makerSide {
		pos.Size += qty
	} else if qty >= absQuantity(pos.Size) {
		pos.Size = qty - absQuantity(pos.Size)
		if pos.Size > 0 {
			pos.Side = makerSide
		} else {
			pos.Side = SIDE_NONE
		}
	} else {
		pos.Size -= qty
	}

	e.trimRO(trade.TakerOrder.UserID, trade.Symbol, pos.Size)
}

// handlePosition processes a position update
func (e *Engine) handlePosition(ev *PositionUpdate) {
	pos := e.getPosition(UserID(ev.UserID), ev.Symbol)
	pos.Size = Quantity(ev.Size)
	pos.Side = ev.Side
	e.trimRO(UserID(ev.UserID), ev.Symbol, Quantity(ev.Size))
}

// handlePriceTick processes price updates and triggers
func (e *Engine) handlePriceTick(ev *PriceTick) {
	// Update lastPrice in both SPOT and LINEAR books
	if spotOb, ok := e.spotBooks[ev.Symbol]; ok {
		spotOb.lastPrice = ev.Price
	}
	if linearOb, ok := e.linearBooks[ev.Symbol]; ok {
		linearOb.lastPrice = ev.Price
	}

	// Check for triggered conditional orders
	triggered := e.triggers.Check(ev.Symbol, Price(ev.Price))
	for _, order := range triggered {
		// Check position for CoT orders
		// BYBIT: If position = 0, CoT order is cancelled (prevent opening opposite)
		pos := e.getPosition(order.UserID, order.Symbol)
		if order.CloseOnTrigger && pos.Size == 0 {
			// Position closed before trigger - cancel this order
			e.triggers.Remove(order.ID)
			continue
		}

		// Calculate qty for the triggered order
		// BYBIT: CoT qty auto-adjusted to position size if qty > position
		triggerQty := order.Quantity
		if order.CloseOnTrigger && order.Quantity > absQuantity(pos.Size) {
			triggerQty = absQuantity(pos.Size)
		}

		// Create child order from conditional order (synchronous for trigger processing)
		e.handleOrder(&OrderRequest{
			UserID:         uint64(order.UserID),
			Symbol:         order.Symbol,
			Category:       order.Category,
			Side:           order.Side,
			Type:           order.Type,
			TIF:            order.TIF,
			Quantity:       int64(triggerQty),
			Price:          int64(order.Price),
			ReduceOnly:     order.ReduceOnly,
			TriggerPrice:   0,     // No longer conditional
			CloseOnTrigger: false, // Already in book, no longer CoT
		})

		// Remove from trigger monitor after activation
		e.triggers.Remove(order.ID)

		// OCO SIBLING CANCELLATION
		// When one OCO order triggers, cancel the other
		if order.OrderLinkId > 0 {
			e.cancelOCOSibling(order)
		}
	}
}

// cancelOCOSibling cancels the sibling order in an OCO pair
// WHY: Bybit OCO spec - when one triggers, the other is automatically cancelled
func (e *Engine) cancelOCOSibling(triggeredOrder *Order) {
	// Find the sibling order in trigger monitor
	// OCO orders share the same OrderLinkId
	for _, order := range e.triggers.GetOrders() {
		if order.OrderLinkId == triggeredOrder.OrderLinkId && order.ID != triggeredOrder.ID {
			// Found sibling - cancel it
			e.triggers.Remove(order.ID)
			return
		}
	}
}

// handleCancel processes order cancellation
func (e *Engine) handleCancel(req *CancelRequest) {
	ob := e.getOrderBook(req.Symbol, req.Category)
	ob.RemoveOrder(OrderID(req.OrderID))
}

// getOrderBook returns the order book for a symbol and category
// WHY: SPOT and LINEAR markets have separate order books
func (e *Engine) getOrderBook(symbol string, category int8) *OrderBook {
	if category == CATEGORY_SPOT {
		if ob, ok := e.spotBooks[symbol]; ok {
			return ob
		}
		ob := NewOrderBook()
		e.spotBooks[symbol] = ob
		return ob
	}
	// LINEAR category
	if ob, ok := e.linearBooks[symbol]; ok {
		return ob
	}
	ob := NewOrderBook()
	e.linearBooks[symbol] = ob
	return ob
}

// getPosition returns user's position for a symbol
func (e *Engine) getPosition(userID UserID, symbol string) *Position {
	if userPos, ok := e.positions[userID]; ok {
		if pos, ok := userPos[symbol]; ok {
			return pos
		}
	}
	if e.positions[userID] == nil {
		e.positions[userID] = make(map[string]*Position)
	}
	pos := &Position{}
	e.positions[userID][symbol] = pos
	return pos
}

// absQuantity returns absolute value of quantity
func absQuantity(q Quantity) Quantity {
	if q < 0 {
		return -q
	}
	return q
}

// getTotalROQty calculates total reduce-only qty for user's position
// WHY: Bybit requires total RO qty <= position size when placing new RO order
func (e *Engine) getTotalROQty(userID UserID, symbol string, posSize Quantity) Quantity {
	h := e.roHeaps[userID][symbol]
	if h == nil {
		return 0
	}

	var total Quantity
	for _, entry := range *h.sell {
		total += entry.Quantity
	}
	for _, entry := range *h.buy {
		total += entry.Quantity
	}
	return total
}

// addROOrder adds a reduce-only or close-on-trigger order to the trimming heap
// WHY: Both RO and CoT orders need trimming when position is reduced
func (e *Engine) addROOrder(order *Order) {
	if e.roHeaps[order.UserID] == nil {
		e.roHeaps[order.UserID] = make(map[string]*ROHeap)
	}
	if e.roHeaps[order.UserID][order.Symbol] == nil {
		e.roHeaps[order.UserID][order.Symbol] = NewROHeap()
	}
	h := e.roHeaps[order.UserID][order.Symbol]
	if order.Side == ORDER_SIDE_SELL {
		heap.Push(h.sell, &ROEntry{
			OrderID:        order.ID,
			Price:          order.Price,
			Quantity:       order.Quantity,
			Side:           order.Side,
			CloseOnTrigger: order.CloseOnTrigger,
		})
	} else {
		heap.Push(h.buy, &ROEntry{
			OrderID:        order.ID,
			Price:          order.Price,
			Quantity:       order.Quantity,
			Side:           order.Side,
			CloseOnTrigger: order.CloseOnTrigger,
		})
	}
}

// trimRO trims reduce-only orders to fit within position size
// WHY: Bybit trims FURTHEST-FIRST: SELL orders (highest price first),
// BUY orders (lowest price first)
func (e *Engine) trimRO(userID UserID, symbol string, posSize Quantity) {
	h := e.roHeaps[userID][symbol]
	if h == nil || posSize == 0 {
		return
	}

	var total Quantity
	if posSize > 0 {
		// LONG position: trim SELL orders (highest price = furthest)
		for _, entry := range *h.sell {
			total += entry.Quantity
		}
		if total <= posSize {
			return
		}
		for total > posSize && h.sell.Len() > 0 {
			entry := heap.Pop(h.sell).(*ROEntry)
			total -= entry.Quantity
		}
	} else {
		// SHORT position: trim BUY orders (lowest price = furthest)
		absSize := -posSize
		for _, entry := range *h.buy {
			total += entry.Quantity
		}
		if total <= absSize {
			return
		}
		for total > absSize && h.buy.Len() > 0 {
			entry := heap.Pop(h.buy).(*ROEntry)
			total -= entry.Quantity
		}
	}
}

// ROEntry - heap entry for reduce-only order trimming
type ROEntry struct {
	OrderID        OrderID
	Price          Price
	Quantity       Quantity
	Side           int8
	CloseOnTrigger bool
}

// SellHeap - max-heap (highest price = furthest from market for SELL)
// WHY: Bybit trims highest SELL price first (furthest from market)
type SellHeap []*ROEntry

func (h SellHeap) Len() int            { return len(h) }
func (h SellHeap) Less(i, j int) bool  { return h[i].Price > h[j].Price }
func (h SellHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *SellHeap) Push(x interface{}) { *h = append(*h, x.(*ROEntry)) }
func (h *SellHeap) Pop() interface{} {
	old := *h
	n := len(old)
	*h = old[0 : n-1]
	return old[n-1]
}

// BuyHeap - min-heap (lowest price = furthest from market for BUY)
// WHY: Bybit trims lowest BUY price first (furthest from market)
type BuyHeap []*ROEntry

func (h BuyHeap) Len() int            { return len(h) }
func (h BuyHeap) Less(i, j int) bool  { return h[i].Price < h[j].Price }
func (h BuyHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *BuyHeap) Push(x interface{}) { *h = append(*h, x.(*ROEntry)) }
func (h *BuyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	*h = old[0 : n-1]
	return old[n-1]
}

// ROHeap contains separate heaps for buy and sell RO orders
type ROHeap struct {
	sell *SellHeap
	buy  *BuyHeap
}

func NewROHeap() *ROHeap {
	s := &SellHeap{}
	b := &BuyHeap{}
	heap.Init(s)
	heap.Init(b)
	return &ROHeap{sell: s, buy: b}
}

// TriggerMonitor manages conditional (trigger) orders
// WHY: Conditional orders are NOT in orderbook until triggered
type TriggerMonitor struct {
	buyTriggers  *buyTriggerHeap  // Min-heap: lowest triggerPrice first
	sellTriggers *sellTriggerHeap // Max-heap: highest triggerPrice first
	orders       map[OrderID]*Order
}

func NewTriggerMonitor() *TriggerMonitor {
	return &TriggerMonitor{
		buyTriggers:  &buyTriggerHeap{},
		sellTriggers: &sellTriggerHeap{},
		orders:       make(map[OrderID]*Order),
	}
}

// Add adds a conditional order to the monitor
// WHY: Order waits here until price condition is met
func (m *TriggerMonitor) Add(order *Order) {
	m.orders[order.ID] = order
	if order.Side == ORDER_SIDE_BUY {
		// BUY triggers: activate when market price <= triggerPrice
		heap.Push(m.buyTriggers, &triggerNode{
			orderID:      order.ID,
			triggerPrice: order.TriggerPrice,
			timestamp:    order.CreatedAt,
		})
	} else {
		// SELL triggers: activate when market price >= triggerPrice
		heap.Push(m.sellTriggers, &triggerNode{
			orderID:      order.ID,
			triggerPrice: order.TriggerPrice,
			timestamp:    order.CreatedAt,
		})
	}
}

// Remove removes an order from the monitor
func (m *TriggerMonitor) Remove(orderID OrderID) {
	delete(m.orders, orderID)
}

// GetOrders returns all orders in the monitor
// WHY: Used for OCO sibling lookup
func (m *TriggerMonitor) GetOrders() []*Order {
	orders := make([]*Order, 0, len(m.orders))
	for _, order := range m.orders {
		orders = append(orders, order)
	}
	return orders
}

// Check returns orders triggered at the given price
// WHY: For BUY: price <= triggerPrice, For SELL: price >= triggerPrice
func (m *TriggerMonitor) Check(symbol string, currentPrice Price) []*Order {
	var triggered []*Order

	// BUY triggers: activate when price <= triggerPrice (price dropped to/at trigger)
	for m.buyTriggers.Len() > 0 {
		node := (*m.buyTriggers)[0]
		if currentPrice <= node.triggerPrice {
			heap.Pop(m.buyTriggers)
			if order := m.orders[node.orderID]; order != nil {
				triggered = append(triggered, order)
			}
		} else {
			break
		}
	}

	// SELL triggers: activate when price >= triggerPrice (price rose to/at trigger)
	for m.sellTriggers.Len() > 0 {
		node := (*m.sellTriggers)[0]
		if currentPrice >= node.triggerPrice {
			heap.Pop(m.sellTriggers)
			if order := m.orders[node.orderID]; order != nil {
				triggered = append(triggered, order)
			}
		} else {
			break
		}
	}

	return triggered
}

type triggerNode struct {
	orderID      OrderID
	triggerPrice Price
	timestamp    uint64
}

type buyTriggerHeap []*triggerNode

func (h buyTriggerHeap) Len() int            { return len(h) }
func (h buyTriggerHeap) Less(i, j int) bool  { return h[i].triggerPrice < h[j].triggerPrice }
func (h buyTriggerHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *buyTriggerHeap) Push(x interface{}) { *h = append(*h, x.(*triggerNode)) }
func (h *buyTriggerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	*h = old[:n-1]
	return node
}

type sellTriggerHeap []*triggerNode

func (h sellTriggerHeap) Len() int            { return len(h) }
func (h sellTriggerHeap) Less(i, j int) bool  { return h[i].triggerPrice > h[j].triggerPrice }
func (h sellTriggerHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *sellTriggerHeap) Push(x interface{}) { *h = append(*h, x.(*triggerNode)) }
func (h *sellTriggerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	*h = old[:n-1]
	return node
}
