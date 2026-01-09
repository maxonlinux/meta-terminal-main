# META-TERMINAL-GO Architecture

## Configuration

Configure via environment variables or `.env` file using `godotenv`:

```bash
# Required
NATS_URL=nats://localhost:4222
STREAM_PREFIX=meta
JWT_SECRET=your-secret-key

# OMS shards (comma-separated, 1 shard per symbol)
SHARDS=BTCUSDT,ETHUSDT,SOLUSDT,...

# Optional
PORT=8080
```

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
│                           Matching Engine (OMS)                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                                                                       │  │
│  │   PlaceOrder(input OrderInput) → OrderResult                         │  │
│  │   CancelOrder(userID, orderID) → error                               │  │
│  │   OnPriceTick(symbol string, price Price)                            │  │
│  │                                                                       │  │
│  │   ─────────────────────────────────────────────────────────────────  │  │
│  │   OMS Shard = 1 symbol (e.g., BTCUSDT)                               │  │
│  │   Contains 2 OrderBooks: orderbooks[category]                         │  │
│  │     - category=0: SPOT orderbook                                      │  │
│  │     - category=1: LINEAR orderbook                                    │  │
│  │                                                                       │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│         │                                                                  │
│         ▼                                                                  │
│  ┌──────────────────────────┐                                              │
│  │   OrderBook State        │                                              │
│  │   orderbooks[category]   │                                              │
│  │   → *OrderBook (O(1))    │                                              │
│  └──────────────────────────┘                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Global State                                      │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  Users State (Portfolio Service)                                      │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ map[UserID]*UserState                                           │  │  │
│  │  │   ├── Balances: map[asset]*UserBalance                         │  │  │
│  │  │   │   └── Buckets: [AVAILABLE=0, LOCKED=1, MARGIN=2]           │  │  │
│  │  │   └── Positions: map[symbol]*Position                          │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  OrderStore (OMS Service)                                             │  │
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

## Balance Flow (FIX Protocol Inspired - Pre-Reservation Model)

Based on FIX protocol and traditional exchange practices, we use **pre-reservation** model:
- **Reserve BEFORE placing order** (not after matching)
- Error on Reserve = Order Rejection (no need for separate balance check)
- **Simple, consistent flow** - same logic for LIMIT and MARKET orders

### Key Principles (FIX Protocol)

1. **Pre-Trade Reservation**: Balance is locked BEFORE order enters matching engine
2. **Lock Amount Calculation**: Based on order parameters, not execution details
3. **Trade Execution**: Always from Locked bucket (never from Available directly)
4. **No Maker/Taker Distinction for Locking**: Both lock the same way
5. **Refund on Cancel**: Unfilled locked amount returns to Available

### Balance Buckets

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              User Balance                                    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  AVAILABLE (0)                                                      │    │
│  │  - Free funds for new orders                                        │    │
│  │  - Can be withdrawn                                                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                  │                                           │
│                                  ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  LOCKED (1)                                                         │    │
│  │  - Reserved for open orders                                         │    │
│  │  - Deducted from Available on order placement                       │    │
│  │  - Source of funds for trade execution                              │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                  │                                           │
│                    ┌─────────────┴─────────────┐                            │
│                    ▼                             ▼                            │
│  ┌─────────────────────────────┐   ┌─────────────────────────────┐          │
│  │  SPOT:                      │   │  LINEAR:                    │          │
│  │  Locked → Available         │   │  Locked → Margin            │          │
│  │  (trade execution)          │   │  (trade execution)          │          │
│  └─────────────────────────────┘   └─────────────────────────────┘          │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  MARGIN (2) [LINEAR only]                                           │    │
│  │  - Collateral for open positions                                    │    │
│  │  - Calculated as (Price × Qty) / Leverage                           │    │
│  │  - Released when position is closed                                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Order Reservation Formulas

```
SPOT ORDER RESERVATION:
├── BUY LIMIT/SELL LIMIT/SELL MARKET:
│   Reserved = Qty × Price (quote currency for BUY, base for SELL)
│   Example: BUY 1 BTC @ 50000 USDT → Reserve 50000 USDT
│   Example: SELL 1 BTC @ 50000 USDT → Reserve 1 BTC
│
└── BUY MARKET:
    Reserved = Qty × Current_Best_Ask (estimated)
    Note: For MARKET orders, we reserve maximum possible or reject if insufficient

LINEAR ORDER RESERVATION:
├── BUY/SELL LIMIT/MARKET:
    Reserved = (Qty × Price) / Leverage (in quote currency)
    Example: BUY 1000 BTCUSDT @ 50000, Leverage=10
             Reserved = (1000 × 50000) / 10 = 5,000,000 USDT
```

### Trade Execution Formulas

```
SPOT TRADE EXECUTION (per trade):
├── BUY (taker or maker):
│   Locked[quote] → Margin[quote] (for LINEAR) OR Locked[quote] → Available[quote] (for SPOT)
│   Actually: Locked → Available (refund locked portion)
│              Available → Deduct (trade_price × trade_qty)
│              Maker: Add (trade_qty) to base asset
│
├── SELL (taker or maker):
│   Locked[base] → Margin[base] (for LINEAR) OR Locked[base] → Available[base] (for SPOT)
│   Actually: Locked → Available (refund locked portion)
│              Available → Deduct (trade_qty)
│              Maker: Add (trade_price × trade_qty) to quote asset

LINEAR TRADE EXECUTION (per trade):
├── BUY/SELL (taker or maker):
│   Locked → Margin (amount = trade_price × trade_qty / leverage)
│   UpdatePosition(trade_size, trade_price, side, leverage)
```

### Simplified Flow Diagrams

#### PlaceOrder → Clearing → Reserve → Match → Execute

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PlaceOrder Flow                                    │
│                                                                              │
│  1. OMS receives OrderInput                                                  │
│     │                                                                        │
│     ▼                                                                        │
│  2. OMS publishes to "clearing.reserve" (RAW: symbol, category, side,       │
│     qty, price, leverage)                                                    │
│     │                                                                        │
│     ▼                                                                        │
│  3. Clearing receives request                                                │
│     │                                                                        │
│     ▼                                                                        │
│  4. Clearing.CalculateReserveAmount() → amount, asset                       │
│     │                                                                        │
│     ▼                                                                        │
│  5. Clearing → Portfolio.Reserve(amount, asset)                             │
│     │                                                                        │
│     ├──────────────────────────────────────────────────────────────────┐     │
│     │ IF error (insufficient balance):                                 │     │
│     │     → Clearing returns error to OMS                             │     │
│     │     → OMS rejects order                                         │     │
│     │                                                                  │     │
│     │ IF success:                                                      │     │
│     │     → Portfolio: Available -= amount                            │     │
│     │     → Portfolio: Locked += amount                               │     │
│     │     → Clearing returns success to OMS                           │     │
│     ▼                                                                  │     │
│  6. OMS proceeds with matching                                            │     │
│     │                                                                  │     │
│     ▼                                                                  │     │
│  7. Match order against orderbook                                        │     │
│     │                                                                  │     │
│     ├──────────────────────────────────────────────────────────────┐   │     │
│     │ FOR EACH TRADE:                                                │   │     │
│     │     a. Publish trade to "clearing.trade"                      │   │     │
│     │                                                                  │     │
│     │     b. Clearing processes trade:                               │   │     │
│     │        - SPOT: Balance updates (Deduct/Add)                    │   │     │
│     │        - LINEAR: Margin updates + Position updates             │   │     │
│     │                                                                  │     │
│     │ IF order remaining > 0:                                        │   │     │
│     │     Add to orderbook (GTC/POST_ONLY)                           │   │     │
│     │                                                                  │     │
│     │ IF order fully filled OR IOC/FOK partial:                      │   │     │
│     │     OMS publishes to "clearing.release" (RAW: remaining)       │   │     │
│     │     → Clearing calculates amount                               │   │     │
│     │     → Clearing → Portfolio.Release(amount)                     │   │     │
└─────────────────────────────────────────────────────────────────────────────┘
```
│     └──────────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### CancelOrder → Release

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CancelOrder Flow                                   │
│                                                                              │
│  1. CancelOrder(userID, orderID)                                             │
│     │                                                                        │
│     ▼                                                                        │
│  2. Get order from orderbook/store                                           │
│     │                                                                        │
│     ▼                                                                        │
│  3. Calculate remaining locked amount                                        │
│     remaining = order.qty - order.filled                                     │
│     │                                                                        │
│     ▼                                                                        │
│  4. Release(remaining locked amount)                                         │
│     │                                                                        │
│     ▼                                                                        │
│  5. Locked -= remaining                                                      │
│     Available += remaining                                                   │
│     │                                                                        │
│     ▼                                                                        │
│  6. Remove order from orderbook                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Position Update Formulas (LINEAR)

```
UpdatePosition(userID, symbol, size, price, side, leverage):
├── NEW POSITION (current size = 0):
│   Position = {Size: size, Side: side, EntryPrice: price, Leverage: leverage}
│
├── SAME SIDE (current side = new side):
│   total_size = |current_size| + |size|
│   new_entry_price = (current_entry × current_size + price × size) / total_size
│   Position = {Size: total_size, Side: same, EntryPrice: new_entry_price}
│
├── OPPOSITE SIDE (current side ≠ new side):
│   reduced_size = |current_size| - |size|
│   IF reduced_size > 0:
│       Position = {Size: reduced_size, Side: current_side, EntryPrice: current_entry}
│   ELSE (position closed or flipped):
│       Position = {Size: |reduced_size|, Side: opposite, EntryPrice: price}
│
└── CLOSING TRADE (reduced_size = 0):
    Release all margin to Available
    Delete position
```

### Example: Complete Trade Lifecycle

```
Scenario: User places BUY 1 BTC @ 50000 USDT LIMIT GTC

1. PlaceOrder:
   Reserved = 1 × 50000 = 50000 USDT
   Available: 100000 → 50000
   Locked: 0 → 50000

2. Matching:
   Order sits in book waiting for seller

3. Trade occurs (Seller sells 0.5 BTC @ 50000):
   Trade 1: 0.5 BTC @ 50000
   Locked: 50000 → 45000 (remaining locked)
   Available: 50000 → 55000 (0.5 BTC returned to available)
   Available: 55000 → 52500 (0.5 BTC × 50000 deducted)
   Seller: +0.5 BTC, -25000 USDT

4. Trade occurs (Seller sells remaining 0.5 BTC @ 50000):
   Trade 2: 0.5 BTC @ 50000
   Locked: 45000 → 0 (all locked released)
   Available: 52500 → 55000 (remaining 0.5 BTC returned)
   Available: 55000 → 50000 (0.5 BTC × 50000 deducted)
   Seller: +0.5 BTC, -25000 USDT

5. Order Status: FILLED
   Final Balance:
   Available: 50000 (unchanged from start)
   Locked: 0 (fully released)
   Position: +1 BTC @ 50000
```

### Margin Requirements (LINEAR)

```
Initial Margin = (Price × Qty) / Leverage
Maintenance Margin = Initial Margin × Maintenance Margin Ratio (0.5%)

Liquidation Check:
├── LONG Position:
│   Liquidation Price = EntryPrice × (1 - 1/Leverage + MMR)
│   Example (L=10, MMR=0.005): Liq = Entry × 0.905
│
└── SHORT Position:
    Liquidation Price = EntryPrice × (1 + 1/Leverage - MMR)
    Example (L=10, MMR=0.005): Liq = Entry × 1.095
```

---

## Critical Rules

**ORDERBOOKS ARE SEPARATE FOR SPOT AND LINEAR!!!**
- Store separate orderbooks for each symbol and each market (linear and spot), access to orderbook MUST be O(1)

**BALANCES ARE COMMON FOR SPOT AND LINEAR!!!**
- All users share the same balance system
- SPOT uses Available/Locked buckets
- LINEAR uses Available/Locked/MARGIN buckets

**RESERVATION IS PRE-TRADE (FIX Protocol)!!!**
- Reserve() called BEFORE matching
- Error from Reserve = Order Rejection
- No separate balance check needed

---

**Common logic**
- All orders (LIMIT and MARKET) reserve balance BEFORE matching
- Only POST_ONLY/GTC orders can go to orderbook
- Only LIMIT orders can be POST_ONLY/GTC
- MARKET orders can only be IOC/FOK
- LIMIT orders can also be IOC/FOK but they do not go to orderbook and act like MARKET but with fixed price limit
- IOC/FOK are only executed if there is immediate liquidity available
- FOK order MUST be executed entirely or cancelled
- IOC order can be executed partially and remaining is canceled.
- IOC/FOK DO reserve balance (unlike old model) - we always reserve now

---

---

## Order Flow

### PlaceOrder(input OrderInput) → OrderResult

```
1. Validate(input)
   - SPOT: reject reduceOnly, closeOnTrigger, trigger (conditional) orders
   - LINEAR: validate reduceOnly
   - MARKET: must be IOC or FOK

2. If TriggerPrice > 0:
   - Create Order with status = UNTRIGGERED
   - Add to orderStore
   - Add to triggerMonitor
   - Return OrderResult with status = UNTRIGGERED
   - END (no matching, no reserve)

3. Calculate Reserve Amount:
   - SPOT BUY: Qty × Price
   - SPOT SELL: Qty
   - LINEAR: (Qty × Price) / Leverage

4. Reserve(userID, symbol, category, amount)
   - IF error (insufficient balance):
     → REJECT ORDER (return error to client)

   - IF success:
     Available -= amount
     Locked += amount

5. Match order against orderbook:
   - For GTC/POST_ONLY: trades + possibly add rest to book
   - For IOC/FOK: immediate trades only, no book

6. For each trade:
   - ExecuteTrade(trade, taker, maker)
     * SPOT: Locked → Available (refund), Available → Deduct
     * LINEAR: Locked → Margin

7. If order remaining > 0 and TIF=GTC/POST_ONLY:
   - Add rest to orderbook

8. If order fully filled OR IOC/FOK partial:
   - Release remaining locked amount
     Locked -= remaining
     Available += remaining

9. Set OrderStatus by TIF:
   - GTC/POST_ONLY: FILLED / PARTIALLY_FILLED / NEW
   - IOC: FILLED / PARTIALLY_FILLED_CANCELED / CANCELED
   - FOK: FILLED / CANCELED

10. Return OrderResult
```

### CancelOrder(orderID OrderID, userID UserID) → error

```
1. order := orderStore.Get(userID, orderID)
2. If order == nil or userID mismatch → return nil

3. If order.Status == UNTRIGGERED:
   - triggerMonitor.Remove(orderID)

4. If order.Status == NEW or PARTIALLY_FILLED:
   - Get locked amount for order
   - Release(userID, symbol, locked_amount)
     Locked -= locked_amount
     Available += locked_amount
   - Remove from orderbook

5. order.Status = CANCELED
6. orderStore.Remove(userID, orderID)
7. Return nil
```

### OnPriceTick(symbol string, price Price)

```
1. registry.SetPrice(symbol, price)

2. checkLiquidations(price)
   - For each position in positions:
     - Calculate liquidation price
     - IF liquidation condition met:
       → Publish liquidation event

3. orderIDs := triggerMonitor.Check(price)

4. For each orderID in orderIDs:
   order := orderStore.GetByID(orderID)
   if order == nil → continue

   if order.CloseOnTrigger:
      handleCloseOnTrigger(order)
   else:
      handleConditional(order)

   triggerMonitor.Remove(orderID)
```

### handleConditional(order *Order)

```
1. order.Status = TRIGGERED

2. Create OrderInput from order:
   - UserID, Symbol, Category, Side, Type, Quantity, Price = from order
   - TriggerPrice = 0
   - CloseOnTrigger = false
   - TIF = preserve original TIF

3. Reserve(userID, symbol, category, calculated_amount)
   - IF error → order.Status = REJECTED, return

4. PlaceOrder(input) → executes the twin order

5. Original order stays as TRIGGERED (for record)
```

### handleCloseOnTrigger(order *Order) — LINEAR only

```
1. pos := positions.Get(order.UserID, order.Symbol)
2. If pos.Size == 0:
   - order.Status = TRIGGERED
   - Return

3. side := opposite(pos.Side)  // LONG → SELL, SHORT → BUY

4. qty := pos.Size  // ENTIRE position!

5. Create OrderInput:
   - UserID, Symbol, Category
   - Side = opposite
   - Quantity = qty
   - Type = same from order
   - Price = (if LIMIT) from order.Price
   - ReduceOnly = true
   - TIF = same

6. Reserve(userID, symbol, category, calculated_amount)
   - IF error → order.Status = REJECTED, return

7. PlaceOrder(input) → executes close order

8. order.Status = TRIGGERED
```

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
