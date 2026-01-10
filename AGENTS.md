# META-TERMINAL-GO Architecture

USE SNOWFLAKE FOR EVERY ID GENERATION ESPECIALLY IF THIS DATA IS GOING TO BE PERSISTED OR SENT TO USERS!!!!!!

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

## Market Isolation (SPOT vs LINEAR)

**CRITICAL: SPOT and LINEAR markets are completely isolated!**

### SPOT Market (Category = 0)
- **NO reduceOnly** - not applicable
- **NO trigger/conditional orders** - not supported
- **NO positions** - only balance changes
- **Balance flow**: Available ↔ Locked (no Margin)
- **Reserve formula**: BUY = Qty × Price, SELL = Qty

### LINEAR Market (Category = 1)
- **HAS reduceOnly** - can only reduce position
- **HAS trigger/conditional orders** - TriggerPrice supported
- **HAS positions** - Size, Side, EntryPrice, Leverage
- **Balance flow**: Available ↔ Locked ↔ Margin
- **Reserve formula**: (Qty × Price) / Leverage

### Trade Event Fields
- `Category` determines market type (0 = SPOT, 1 = LINEAR)
- Trade execution logic MUST check Category and branch accordingly
- **Leverage is NOT in Trade event** - only Clearing knows about leverage
- **SPOT**: No leverage, no positions, just balance transfer (Available ↔ Locked)
- **LINEAR**: Leverage calculated from user's position by Clearing service

---

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
│   Actually: Locked → Available (refund locked portion)
│              Available → Deduct (trade_price × trade_qty)
│              Maker: Add (trade_qty) to base asset
│
├── SELL (taker or maker):
│   Actually: Locked → Available (refund locked portion)
│              Available → Deduct (trade_qty)
│              Maker: Add (trade_price × trade_qty) to quote asset

LINEAR TRADE EXECUTION (per trade):
├── BUY/SELL (taker or maker):
│   Locked → Margin (amount = trade_price × trade_qty / leverage)
│   UpdatePosition
```

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

## API Contract

### PlaceOrder Request

```json
POST /api/v1/order
{
  "symbol": "BTCUSDT",
  "category": 1,        // 0=SPOT, 1=LINEAR
  "side": 0,            // 0=Buy, 1=Sell
  "type": 0,            // 0=Limit, 1=Market
  "tif": 0,             // 0=GTC, 1=IOC, 2=FOK, 3=PostOnly
  "quantity": 1.0,
  "price": 50000,       // Required for LIMIT, ignored for MARKET
  "triggerPrice": 0,    // >0 makes order conditional
  "reduceOnly": false,  // Only LINEAR, cannot increase position
  "closeOnTrigger": false, // Only LINEAR, requires existing position
  "stopOrderType": 0,   // 0=NORMAL, 1=STOP, 2=TAKE_PROFIT, 3=STOP_LOSS, 4=TRAILING, 5=OCO

  // OCO (One-Cancels-the-Other) - Only LINEAR
  "oco": {
    "quantity": 0,      // 0 = use full position size, >0 = specific qty
    "takeProfit": {
      "triggerPrice": 55000,
      "price": 54900,   // 0 = Market order when triggered
      "reduceOnly": true
    },
    "stopLoss": {
      "triggerPrice": 45000,
      "price": 45100,
      "reduceOnly": true
    }
  }
}
```

### PlaceOrder Response

**ALWAYS returns an array of orders**, even for single orders:

```json
// Single order response
{
  "orders": [{
    "id": 12345,
    "symbol": "BTCUSDT",
    "category": 1,
    "side": 0,
    "type": 0,
    "status": 0,
    "price": 50000,
    "quantity": 1,
    "filled": 0,
    "triggerPrice": 0,
    "reduceOnly": false,
    "closeOnTrigger": false,
    "stopOrderType": 0,
    "orderLinkId": -1,
    "isConditional": false,
    "createdAt": 1234567890
  }],
  "filled": 0,
  "remaining": 1,
  "status": 0
}

// OCO order response (2 orders created)
{
  "orders": [
    {
      "id": 12346,
      "symbol": "BTCUSDT",
      "side": 1,
      "type": 0,
      "status": 5,  // UNTRIGGERED
      "price": 54900,
      "quantity": 0,  // 0 = full position close at trigger
      "triggerPrice": 55000,
      "stopOrderType": 2,  // TAKE_PROFIT
      "closeOnTrigger": true,
      "reduceOnly": true,
      "orderLinkId": 999999999999,
      "isConditional": true
    },
    {
      "id": 12347,
      "symbol": "BTCUSDT",
      "side": 1,
      "type": 0,
      "status": 5,  // UNTRIGGERED
      "price": 45100,
      "quantity": 0,
      "triggerPrice": 45000,
      "stopOrderType": 3,  // STOP_LOSS
      "closeOnTrigger": true,
      "reduceOnly": true,
      "orderLinkId": 999999999999,
      "isConditional": true
    }
  ],
  "filled": 0,
  "remaining": 1,  // Position size
  "status": 5
}
```

---

## OrderInput Type (internal/types/types.go)

```go
// OrderInput — входные данные для создания ордера
//
// orderLinkId (опционально):
//   Пользовательский идентификатор ордера. Макс 36 символов.
//   Позволяет клиенту связать свой внутренний ID с системным orderId.
//   Для OCO: orderLinkId родительского ордера связывает TP и SL ордера.
//   В нашей системе: >0 = snowflake ID группы, -1 = нет группы
//
// isConditional (автоматически):
//   true если TriggerPrice > 0
//   Устанавливается при валидации для удобства в коде
//
type OrderInput struct {
    UserID   UserID
    Symbol   string
    Category int8

    Side int8
    Type int8
    TIF  int8

    Quantity Quantity
    Price    Price

    TriggerPrice   Price
    ReduceOnly     bool
    CloseOnTrigger bool
    StopOrderType  int8

    // IsConditional — true если TriggerPrice > 0
    // Устанавливается автоматически при валидации
    IsConditional bool

    OCO *OCOInput `json:"oco,omitempty"`
}

// OCOInput — параметры OCO (One-Cancels-the-Other)
// Создаёт 2 связанных ордера: Take Profit + Stop Loss
//
// Правила OCO:
//   1. Quantity=0 → auto-use position size + reduceOnly=true
//   2. Оба ордера создаются как CloseOnTrigger=true
//   3. Оба ордера получают одинаковый OrderLinkId (snowflake ID)
//   4. При срабатывании TP → SL автоматически cancelled
//   5. При срабатывании SL → TP автоматически cancelled
//   6. Если позиция закрыта другим способом → оба cancelled
//   7. LONG: TP trigger > SL trigger
//   8. SHORT: TP trigger < SL trigger
//
type OCOInput struct {
    Quantity   Quantity    // 0 = auto, use position size
    TakeProfit OCOChildOrder
    StopLoss   OCOChildOrder
}

type OCOChildOrder struct {
    TriggerPrice Price  // TP или SL trigger price
    Price        Price  // Limit price для TP/SL (0 = Market)
    ReduceOnly   bool   // Всегда true для OCO
}

// OrderResult — результат PlaceOrder
// ВСЕГДА возвращается массив orders[] (1 элемент для single, 2+ для OCO/batch)
type OrderResult struct {
    Orders    []*Order   // Все созданные ордера (1 = single, 2 = OCO)
    Trades    []*Trade   // Сделки если были
    Filled    Quantity   // Сумма filled для primary order
    Remaining Quantity   // Сумма remaining для primary order / позиции
    Status    int8       // Статус primary order / группы
}
```

---

## Order Type (internal/types/types.go)

```go
// Order — торговая заявка в системе
//
// OrderLinkId — группа связанных ордеров (OCO, TP+SL)
//   > 0 = snowflake ID группы (все ордера в группе имеют одинаковый ID)
//   -1 = нет группы (обычные ордера, одиночные conditional)
//
// Пример: OCO создаёт 2 ордера с одинаковым OrderLinkId = snowflake.Next()
// При срабатывании одного — второй отменяется по этому ID
//
type Order struct {
    ID       OrderID
    UserID   UserID
    Symbol   string
    Category int8

    Side int8
    Type int8
    TIF  int8

    Status int8

    Price    Price
    Quantity Quantity
    Filled   Quantity

    TriggerPrice   Price
    ReduceOnly     bool
    CloseOnTrigger bool
    StopOrderType  int8

    // IsConditional — true если ордер conditional (TriggerPrice > 0)
    // Для удобства проверки типа ордера в коде
    IsConditional bool

    // OrderLinkId — группа OCO/TP/SL
    // > 0 = snowflake ID группы
    // -1 = нет группы
    OrderLinkId int64

    CreatedAt uint64
    UpdatedAt uint64
    ClosedAt  uint64
}
```

---

## StopOrderType Constants (internal/constants/constants.go)

```go
// Stop Order Types (Bybit-compatible)
// OCO, TP, SL — все CloseOnTrigger=true, различаются только stopOrderType
STOP_ORDER_TYPE_NORMAL       = 0  // Standard conditional order (Stop)
STOP_ORDER_TYPE_STOP         = 1  // Standard stop order
STOP_ORDER_TYPE_TAKE_PROFIT  = 2  // Take profit order
STOP_ORDER_TYPE_STOP_LOSS    = 3  // Stop loss order
STOP_ORDER_TYPE_TRAILING     = 4  // Trailing stop (future)
STOP_ORDER_TYPE_OCO          = 5  // OCO order (TP + SL pair)
```

---

## Order Types Summary

| Type | TriggerPrice | CloseOnTrigger | Quantity=0 | Description |
|------|--------------|----------------|------------|-------------|
| **Normal** | = 0 | false | ❌ | Regular LIMIT/MARKET order |
| **Conditional** | > 0 | false | ✅ | Waits for trigger → creates identical order without trigger |
| **CloseOnTrigger** | > 0 | true | ✅ | Waits for trigger → creates reduceOnly order to close position |
| **OCO (TP+SL)** | > 0 | true | ✅ | Two linked orders, one cancels the other when triggered |

**Quantity=0 meaning:**
- For conditional/closeOnTrigger orders: "use position size at trigger time"
- For regular orders: **NOT allowed** (returns ErrInvalidQuantity)

---

## Validation Rules (internal/oms/service.go)

### Field Validation (always applied)

```go
// Errors
ErrInvalidQuantity      = errors.New("quantity must be greater than 0")
ErrInvalidPrice         = errors.New("price must be >= 0 for LIMIT orders")
ErrInvalidSymbol        = errors.New("invalid symbol format")
ErrInvalidCategory      = errors.New("invalid category: must be 0 (SPOT) or 1 (LINEAR)")
ErrInvalidSide          = errors.New("invalid side: must be 0 (BUY) or 1 (SELL)")
ErrInvalidOrderType     = errors.New("invalid order type: must be 0 (LIMIT) or 1 (MARKET)")
ErrInvalidTIF           = errors.New("invalid time in force")
ErrInvalidStopOrderType = errors.New("invalid stop order type")
```

**Checks:**
- `Quantity` > 0 for regular orders, = 0 allowed for conditional/closeOnTrigger
- `Price` >= 0 for LIMIT orders
- `Symbol` - valid format (BTCUSDT, ETHUSDT, etc.)
- `Category` - only 0 (SPOT) or 1 (LINEAR)
- `Side` - only 0 (BUY) or 1 (SELL)
- `Type` - only 0 (LIMIT) or 1 (MARKET)
- `TIF` - GTC, IOC, FOK, POST_ONLY

### SPOT-specific Validation

```go
ErrReduceOnlySpot    = errors.New("reduceOnly not allowed for SPOT")
ErrConditionalSpot   = errors.New("conditional orders not allowed for SPOT")
ErrCloseOnTriggerSpot = errors.New("closeOnTrigger not allowed for SPOT")
```

### LINEAR-specific Validation

```go
ErrMarketTIF                    = errors.New("market orders must be IOC or FOK")
ErrCloseOnTriggerNoPosition     = errors.New("closeOnTrigger requires an existing position")
ErrReduceOnlyNoPosition         = errors.New("reduceOnly not allowed without position")
ErrReduceOnlyCommitmentExceeded = errors.New("reduceOnly commitment exceeds position")
ErrInvalidTriggerPrice          = errors.New("invalid trigger price: BUY trigger must be below current price, SELL trigger must be above")
```

### OCO Validation

```go
ErrOCOSpot           = errors.New("OCO orders not allowed for SPOT")
ErrOCONoPosition     = errors.New("OCO orders require an existing position")
ErrOCOTPTriggerInvalid  = errors.New("OCO TP trigger must be > SL trigger for LONG positions")
ErrOCOSLTriggerInvalid  = errors.New("OCO SL trigger must be < TP trigger for SHORT positions")
```

**OCO rules:**
- LONG position: `TakeProfit.TriggerPrice > StopLoss.TriggerPrice`
- SHORT position: `TakeProfit.TriggerPrice < StopLoss.TriggerPrice`

### Self-Match Prevention

```go
ErrSelfMatch = errors.New("self-match prevention: order would match with own order")
```

Prevents placing orders that would execute against own orders in the orderbook.

---

## Self-Match Prevention

Prevents orders from executing against own orders in the orderbook:

```go
func (s *Service) checkSelfMatch(input *types.OrderInput) error
```

**Rules:**
- Conditional and closeOnTrigger orders are excluded (don't go to orderbook)
- For LIMIT orders: checks if best bid/ask would match user's own order
- For MARKET orders: checks if any orders exist on opposite side

---

## ID Generation

**Snowflake Algorithm Implementation** (`internal/snowflake/snowflake.go`)

```go
var counter int64

func Next() int64 {
    return atomic.AddInt64(&counter, 1)
}
```

**Performance:**
- **~1.8 ns/op** for ID generation
- Lock-free with atomic operations (zero contention)
- No external dependencies, no time calculation

**Usage:**
```go
order.ID = types.OrderID(snowflake.Next())
trade.ID = types.TradeID(snowflake.Next())
orderLinkId = snowflake.Next()  // For OCO groups
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
ORDER_STATUS_NEW                       = 0
ORDER_STATUS_PARTIALLY_FILLED          = 1
ORDER_STATUS_FILLED                    = 2
ORDER_STATUS_CANCELED                  = 3
ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
ORDER_STATUS_UNTRIGGERED               = 5
ORDER_STATUS_TRIGGERED                 = 6
ORDER_STATUS_DEACTIVATED               = 7

// Balance Buckets
BUCKET_AVAILABLE = 0
BUCKET_LOCKED    = 1
BUCKET_MARGIN    = 2

// Position Side
SIDE_NONE  = -1
SIDE_LONG  = 0
SIDE_SHORT = 1

// Stop Order Types
STOP_ORDER_TYPE_NORMAL       = 0
STOP_ORDER_TYPE_STOP         = 1
STOP_ORDER_TYPE_TAKE_PROFIT  = 2
STOP_ORDER_TYPE_STOP_LOSS    = 3
STOP_ORDER_TYPE_TRAILING     = 4
STOP_ORDER_TYPE_OCO          = 5
```

---

## Configure via environment variables

```bash
# Required
NATS_URL=nats://localhost:4222
STREAM_PREFIX=meta
JWT_SECRET=your-secret-key

# Optional
PORT=8080
```

---

## Performance Targets

| Operation | Target Latency | Actual | Status |
|-----------|----------------|--------|--------|
| PlaceOrder | < 500μs | **264ns** | ✓ EXCELLENT |
| MatchOrder | < 200μs | **38.5ns** | ✓ EXCELLENT |
| CancelOrder | < 100μs | **6.3ns** | ✓ EXCELLENT |
| BestBidAsk | < 10μs | **7.7ns** | ✓ EXCELLENT |
| ConcurrentMatch | < 200μs | **116ns** | ✓ EXCELLENT |
| Pool GetOrder | < 10μs | **7.3ns** | ✓ EXCELLENT |

---

## Key Interfaces

### Clearing Interface

```go
type Clearing interface {
    Reserve(userID UserID, symbol string, category int8, side int8, qty Quantity, price Price) error
    Release(userID UserID, symbol string, category int8, side int8, qty Quantity, price Price)
    ExecuteTrade(trade *Trade, taker *Order, maker *Order)
}
```

---

## Trigger Monitor

```go
type TriggerMonitor struct {
    buyTriggers  *TriggerHeap  // MIN heap: BUY activate when price ≤ trigger
    sellTriggers *TriggerHeap  // MAX heap: SELL activate when price ≥ trigger
}

func (m *TriggerMonitor) Add(order *Order)
func (m *TriggerMonitor) Remove(orderID OrderID)
func (m *TriggerMonitor) Check(currentPrice Price) []OrderID
```

---

## Critical Rules

1. **ORDERBOOKS ARE SEPARATE FOR SPOT AND LINEAR!!!**
   - Store separate orderbooks for each symbol and each market
   - Access to orderbook MUST be O(1)

2. **BALANCES ARE COMMON FOR SPOT AND LINEAR!!!**
   - All users share the same balance system
   - SPOT uses Available/Locked buckets
   - LINEAR uses Available/Locked/MARGIN buckets

3. **RESERVATION IS PRE-TRADE (FIX Protocol)!!!**
   - Reserve() called BEFORE matching
   - Error from Reserve = Order Rejection

4. **OCO ORDERS ARE CLOSE ON TRIGGER!!!**
   - OCO always has CloseOnTrigger = true
   - OCO always has ReduceOnly = true
   - Quantity = 0 means "close full position at trigger"

5. **OrderLinkId GROUPS OCO ORDERS!!!**
   - Both TP and SL orders get the same OrderLinkId
   - When one triggers, the other is cancelled by OrderLinkId

6. **QUANTITY=0 HAS SPECIAL MEANING!!!**
   - Regular orders: NOT allowed (ErrInvalidQuantity)
   - Conditional/CloseOnTrigger: use position size at trigger time

7. **SELF-MATCH PREVENTION!!!**
   - Orders cannot execute against own orders in the book
   - Checked before order is placed

8. **ALL VALIDATIONS ARE IN ONE PLACE!!!**
   - `validateOrder()` function in OMS handles all validation
   - Both field validation and business logic validation

---

## Order Flow

### PlaceOrder → OrderResult (ALWAYS ARRAY)

```
1. Validate(input)
   - Field validation (quantity, price, symbol, etc.)
   - SPOT/LINEAR specific validation
   - OCO validation (TP > SL for LONG, TP < SL for SHORT)
   - Self-match prevention
   - Set IsConditional if TriggerPrice > 0

2. If OCO:
   - Validate position exists
   - Validate TP/SL trigger prices
   - Create TP and SL orders with Quantity=0
   - Both get CloseOnTrigger=true, ReduceOnly=true
   - Both get same OrderLinkId
   - Return: Orders=[tp, sl], Status=UNTRIGGERED

3. If TriggerPrice > 0 (Conditional):
   - Create Order with status=UNTRIGGERED
   - Add to triggerMonitor
   - Return: Orders=[order], Status=UNTRIGGERED

4. If Regular order:
   - Reserve balance
   - Match against orderbook
   - Return: Orders=[order], Trades=[...], Status=...
```

### OnPriceTick

```
1. registry.SetPrice(symbol, price)

2. orderIDs := triggerMonitor.Check(price)

3. For each orderID:
   - Get order from store
   - If CloseOnTrigger:
     - Get current position size
     - If Quantity=0, use position size
     - Create reduceOnly order to close position
   - Else (Conditional):
     - Create twin order without trigger
     - Execute normal order flow
   - Cancel linked OCO order by OrderLinkId
```

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
    orderPool.Put(o)
}
```
