# META-TERMINAL-GO Architecture

## Configuration

- Use snowflake id for IDs
- Use recommended GO project structure
- Configure via environment variables or `.env` file:

## Performance Targets

| Operation | Target Latency | Actual | Status |
|-----------|----------------|--------|--------|
| PlaceOrder | < 500μs | **264ns** | ✓ EXCELLENT |
| MatchOrder | < 200μs | **38.5ns** | ✓ EXCELLENT |
| TradeExec | < 300μs | **-** | TODO |
| PriceTick | < 100μs | **-** | TODO |

## Overview

High-performance trading engine with SPOT and LINEAR markets, written in Go.

- For every single implementation you must create bench test that tests all actions so I can see performance.

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              API Layer                                       │
│                         (HTTP Handlers + WebSocket)                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Matching Engine                                    │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                                                                       │  │
│  │   PlaceOrder(input OrderInput, market Market) → OrderResult          │  │
│  │   CancelOrder(orderID OrderID, userID UserID) → error                │  │
│  │   OnPriceTick(symbol string, price Price)                            │  │
│  │                                                                       │  │
│  │   ─────────────────────────────────────────────────────────────────  │  │
│  │   Internal:                                                          │  │
│  │   - handleConditional(order *Order)                                  │  │
│  │   - handleCloseOnTrigger(order *Order)                               │  │
│  │   - executeOrder(input, validator, clearing, orderbook)              │  │
│  │                                                                       │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│         │                    │                    │                          │
│         ▼                    ▼                    ▼                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────┐      │
│  │   SPOT       │    │   LINEAR     │    │   OrderBook State        │      │
│  │  Validator   │    │  Validator   │    │   [symbol][category]     │      │
│  │  + Clearing  │    │  + Clearing  │    │   → *OrderBook           │      │
│  └──────────────┘    └──────────────┘    └──────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Global State                                      │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  Users State                                                          │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ map[UserID]*UserState                                           │  │  │
│  │  │   ├── Balances: map[asset]*UserBalance                         │  │  │
│  │  │   │   └── Buckets: [AVAILABLE=0, LOCKED=1, MARGIN=2]           │  │  │
│  │  │   └── Positions: map[symbol]*Position                          │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  OrderStore                                                          │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ map[UserID]map[OrderID]*Order                                   │  │  │
│  │  │   - Normal orders                                               │  │  │
│  │  │   - Conditional orders (TriggerPrice > 0, status=UNTRIGGERED)  │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  Registry                                                            │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ Instruments: map[symbol]*Instrument                             │  │  │
│  │  │ LastPrices: map[symbol]Price                                    │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  Trigger State (COMMON INDEX)                                        │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ map[symbol]*TriggerMonitor                                      │  │  │
│  │  │   - buyTriggers: MIN heap (price, orderID)                      │  │  │
│  │  │   - sellTriggers: MAX heap (price, orderID)                     │  │  │
│  │  │   NO LOGIC! Just stores indices and returns triggered orderIDs  │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Price Feed (NATS)                                 │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  *PriceFeed                                                          │  │
│  │                                                                       │  │
│  │  OnMessage(symbol string, price Price):                              │  │
│  │      1. registry.SetLastPrice(symbol, price)                        │  │
│  │      2. log.Printf("Checking liquidations for %s @ %d", ...)        │  │
│  │      3. engine.OnPriceTick(symbol, price)                           │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Key Interfaces

### Market Interface

```go
type Market interface {
    GetValidator() Validator
    GetClearing() Clearing
    GetCategory() int8
    GetOrderBookState() *orderbook.State
}
```

### Validator Interface

```go
type Validator interface {
    Validate(input *OrderInput) error
}
```

### Clearing Interface

```go
type Clearing interface {
    Reserve(userID UserID, symbol string, qty Quantity, price Price) error
    Release(userID UserID, symbol string, qty Quantity, price Price)
    ExecuteTrade(trade *Trade, taker *Order, maker *Order)
}
```

```go
type TriggerMonitor struct {
    buyTriggers  *TriggerHeap  // MIN heap: BUY activate when price ≤ trigger
    sellTriggers *TriggerHeap  // MAX heap: SELL activate when price ≥ trigger
}

func (m *TriggerMonitor) Add(order *Order)
func (m *TriggerMonitor) Remove(orderID OrderID)
func (m *TriggerMonitor) Check(currentPrice Price) []OrderID  // Returns triggered orderIDs
```

---

## Order Types

| Type | TriggerPrice | CloseOnTrigger | Description |
|------|--------------|----------------|-------------|
| **Normal** | = 0 | false | Regular LIMIT/MARKET order |
| **Conditional** | > 0 | false | Waits for trigger → creates identical order (without trigger) → status=TRIGGERED |
| **CloseOnTrigger** | > 0 | true | Waits for trigger → creates reduceOnly order with qty=position_size if LIMIT or market NON-reduceOnly order with qty=position_size if MARKET → becomes itself status=TRIGGERED |

---

## Constants
```go
package constants

const (
	CATEGORY_SPOT   = 0
	CATEGORY_LINEAR = 1

	ORDER_TYPE_LIMIT  = 0
	ORDER_TYPE_MARKET = 1

	ORDER_SIDE_BUY  = 0
	ORDER_SIDE_SELL = 1

	TIF_GTC       = 0
	TIF_IOC       = 1
	TIF_FOK       = 2
	TIF_POST_ONLY = 3

	ORDER_STATUS_NEW                       = 0
	ORDER_STATUS_PARTIALLY_FILLED          = 1
	ORDER_STATUS_FILLED                    = 2
	ORDER_STATUS_CANCELED                  = 3
	ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
	ORDER_STATUS_UNTRIGGERED               = 5
	ORDER_STATUS_TRIGGERED                 = 6
	ORDER_STATUS_DEACTIVATED               = 7

	STOP_ORDER_TYPE_NORMAL      = 0
	STOP_ORDER_TYPE_STOP        = 1
	STOP_ORDER_TYPE_TP          = 2
	STOP_ORDER_TYPE_SL          = 3
	STOP_ORDER_TYPE_LIQUIDATION = 4

	MAINTENANCE_MARGIN_RATIO = 10
)
````

## Balance Flow

- Balance is ONLY reserved for the remaining quantity AFTER initial trades when order is ready to go to orderbook!!!!
- We DO NOT reserve balance when order is matching before going to orderbook

1. Clarified Balance Flow Logic

### For SPOT MARKET BUY Orders:
Place Order: Match immediately (NO LOCK/UNLOCK)
Trade Exec:  Available → Deduct (trade_price × trade_qty)
             Maker → Add (trade_qty)
             
### For SPOT MARKET SELL Orders:
Place Order: Match immediately (NO LOCK/UNLOCK)
Trade Exec:  Available → Deduct (trade_qty)
             Maker → Add (trade_qty × trade_price)

### For LINEAR MARKET Orders:
Place Order: Match immediately (NO LOCK/UNLOCK)
Trade Exec:  Available → MARGIN (trade_price × trade_qty / leverage)
             UpdatePosition(trade_size, trade_price, side, leverage)

### For SPOT LIMIT BUY Orders:
Place Order: Available → Locked (order_price × qty)
Trade Exec:  Locked → Available (order_price × trade_qty)
             Available → Deduct (trade_price × trade_qty)
             Maker → Add (trade_qty)
             
### For SPOT LIMIT SELL Orders:
Place Order: Available → Locked (qty)
Trade Exec:  Locked → Available (trade_qty)
             Available → Deduct (trade_qty)
             Maker → Add (trade_price × trade_qty)
             
### For LINEAR LIMIT Orders:
Place Order: Available → Locked (order_price × qty / leverage)
Trade Exec:  Locked → Available (order_price × trade_qty / leverage)
             Available → MARGIN (trade_price × trade_qty / leverage)
             UpdatePosition(trade_size, trade_price, side, leverage)

2. Fixed SPOT Flow
- Proper Locked/Available flow for LIMIT orders
- Deduct/Add for trade execution
- Refund unfilled locked portion on order cancel

3. Fixed LINEAR Flow
- Changed from direct MARGIN to LOCKED → Available → MARGIN flow
- Each trade participant uses their own leverage for margin calculation
- Refund unfilled locked portion on order cancel

---

## Critical Rules

**ORDERBOOKS ARE SEPARATE FOR SPOT AND LINEAR!!!**
- Store separate orderbooks for each symbol and each market (linear and spot), access to orderbook MUST be O(1)

**BALANCES ARE COMMON FOR SPOT AND LINEAR!!!**
- All users share the same balance system
- SPOT uses Available/Locked buckets
- LINEAR uses Available/Locked/MARGIN buckets

---

**Common logic**
- Only LIMIT orders that go to orderbook (become makers) can reserve balance

- Only POST_ONLY/GTC orders can go to book
- Only LIMIT orders can be POST_ONLY/GTC
- MARKET orders can only be IOC/FOK
- LIMIT orders can also be IOC/FOK but they do not go to orderbook and act like MARKET but with fixed price limit
- IOC/FOK are only executed if there is immediate liquidity available
- FOK order MUST be executed entirely or cancelled
- IOC order can be executed partially and remaining is canceled.
- IOC/FOK do not reserve anything at all because they do NOT go to orderbook 

---

## Order Flow

### PlaceOrder(input OrderInput, market Market) → OrderResult

```
1. market.Validator.Validate(input)
   - SPOT: reject reduceOnly, closeOnTrigger, trigger (conditional) orders
   - LINEAR: validate reduceOnly

2. If TriggerPrice > 0:
   - Create Order with status = UNTRIGGERED
   - Add to orderStore
   - Add to triggerState.Get(symbol).TriggerMonitor
   - Return OrderResult with status = UNTRIGGERED
   - END (no matching, no reserve)

3. If input.Type == MARKET and input.TIF not in {IOC, FOK}:
   - Return error: "market orders must be IOC or FOK"

4. Create Order + OrderResult

5. If TIF in {GTC, POST_ONLY}:
   a. market.Clearing.Reserve() for remaining qty
   b. market.GetOrderBookState().Get(symbol, category).AddOrder() → trades
   c. If error → clearing.Release(), remove from book

6. If TIF in {IOC, FOK}:
   a. orderbook.AddOrder() → trades
   b. NO reserve (orders don't go to book)

7. For each trade:
   a. market.Clearing.ExecuteTrade(trade, taker, maker)

8. Set OrderStatus by TIF:
   - GTC/POST_ONLY: FILLED / PARTIALLY_FILLED / NEW
   - IOC: FILLED / PARTIALLY_FILLED_CANCELED / CANCELED
   - FOK: FILLED / CANCELED

9. Return OrderResult
```

### CancelOrder(orderID OrderID, userID UserID) → error

```
1. order := orderStore.Get(userID, orderID)
2. If order == nil or userID mismatch → return nil

3. If order.Status == UNTRIGGERED:
   - triggerState.Get(order.Symbol).Remove(orderID)

4. If order.Status == NEW or PARTIALLY_FILLED:
   - market := GetMarket(order.Category)
   - market.Clearing.Release() for remaining qty
   - market.GetOrderBookState().Get(order.Symbol, order.Category).RemoveOrder(order)

5. order.Status = CANCELED
6. orderStore.Remove(userID, orderID)
7. Return nil
```

### OnPriceTick(symbol string, price Price)

```
1. registry.SetLastPrice(symbol, price)

2. log.Printf("Checking liquidations for %s @ %d", symbol, price)
   (future: real liquidation check with logging)
   we display all the position IDs that need to be liquidated

3. orderIDs := triggerState.Get(symbol).Check(price)

4. For each orderID in orderIDs:
   order := orderStore.GetByID(orderID)
   if order == nil → continue

   if order.CloseOnTrigger:
      handleCloseOnTrigger(order)
   else:
      handleConditional(order)

   triggerState.Get(symbol).Remove(orderID)
```

### handleConditional(order *Order)

```
1. Create OrderInput from order:
   - UserID, Symbol, Category, Side, Type, Quantity, Price = from order
   - TriggerPrice = 0
   - CloseOnTrigger = false
   - TIF = preserve original TIF

2. market := GetMarket(order.Category)

3. engine.executeOrder(input, market.Validator, market.Clearing, orderbook)

4. order.Status = TRIGGERED
```

### handleCloseOnTrigger(order *Order) — LINEAR only

```
1. pos := positions.Get(order.UserID, order.Symbol)
2. If pos.Size == 0:
   - order.Status = TRIGGERED
   - Return

3. side := opposite(pos.Side)  // LONG → SELL, SHORT → BUY

4. Create OrderInput:
   - UserID, Symbol, Category
   - Side - opposite
   - Quantity = pos.Size  // ENTIRE position!
   - Type = same from order
   - Price = (if LIMIT) from order.Price
   - ReduceOnly = if limit = true, if market = false
   - TIF = same

5. linear := GetMarket(CATEGORY_LINEAR)

6. engine.executeOrder(input, linear.Validator, linear.Clearing, orderbook)

7. order.Status = TRIGGERED
```

---

## Constants

```go
// Category
CATEGORY_SPOT   = 0
CATEGORY_LINEAR = 1

// Order Type
ORDER_TYPE_LIMIT  = 0
ORDER_TYPE_MARKET = 1

// Order Side
ORDER_SIDE_BUY  = 0
ORDER_SIDE_SELL = 1

// TIF
TIF_GTC       = 0
TIF_IOC       = 1
TIF_FOK       = 2
TIF_POST_ONLY = 3

// Order Status
ORDER_STATUS_NEW                     = 0
ORDER_STATUS_PARTIALLY_FILLED        = 1
ORDER_STATUS_FILLED                  = 2
ORDER_STATUS_CANCELED                = 3
ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
ORDER_STATUS_UNTRIGGERED             = 5
ORDER_STATUS_TRIGGERED               = 6

// Balance Buckets
BUCKET_AVAILABLE = 0
BUCKET_LOCKED    = 1
BUCKET_MARGIN    = 2

// Position Side
SIDE_NONE  = -1
SIDE_LONG  = 0
SIDE_SHORT = 1
```

---

## Symbol Registry (HTTP Loader)

```
Assets URL: http://146.103.123.216:3000/assets
Multiplexer URL: http://localhost:3333/proxy/multiplexer
Sync Interval: 5 minutes

LoadAssets():
1. GET Assets URL → [{symbol: "BTCUSDT"]}, {symbol: "ETHUSDT"}, ...]
2. For each symbol:
   a. GET Multiplexer URL/prices?symbol=XXX
   b. If price == null or 404 → skip symbol
   c. If price received:
      - instruments[symbol] = FromSymbol(symbol, price)
      - lastPrices[symbol] = price

Start():
- LoadAssets()
- Start periodic sync every 5 minutes
```

---

## Price Feed (NATS)

```
NATS_ADDR: <placeholder>

OnMessage(symbol string, price Price):
1. registry.SetLastPrice(symbol, price)
2. log liquidation check
3. triggers.OnPriceTick(symbol, price)
```

---

## Performance Targets

| Operation | Target | Actual | Status |
|-----------|--------|--------|--------|
| PlaceOrder | < 500μs | **264ns** | ✓ EXCELLENT |
| MatchOrder | < 200μs | **38.5ns** | ✓ EXCELLENT |
| CancelOrder | < 100μs | **6.3ns** | ✓ EXCELLENT |
| BestBidAsk | < 10μs | **7.7ns** | ✓ EXCELLENT |
| ConcurrentMatch | < 200μs | **116ns** | ✓ EXCELLENT |
| Pool GetOrder | < 10μs | **7.3ns** | ✓ EXCELLENT |
| WAL Save | < 100μs | **668ns** | ✓ EXCELLENT |
| WAL Load | < 50μs | **128ns** | ⚠ OPTIMIZE |
| WAL SaveTx | < 100μs | **437ns** | ✓ EXCELLENT |

---

## Zero-Allocation Design

```go
// Object pooling
var orderPool = sync.Pool{
    New: func() interface{} { return new(Order) },
}

func GetOrder() *Order {
    return orderPool.Get().(*Order)
}

func PutOrder(o *Order) {
    o.ID = 0
    o.UserID = 0
    // reset fields
    orderPool.Put(o)
}
```
