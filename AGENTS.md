# META-TERMINAL-GO Architecture

## Zero-Allocation Sub-Millisecond Trading Engine

---

## Структура проекта

```
meta-terminal-go/
├── cmd/
│   └── main.go                           # Entry point
│
├── types/                                # Все типы - РАЗДЕЛЕНЫ ПО ДОМЕНУ
│   ├── base.go                           # OrderID, UserID, Price, Quantity, NanoTime()
│   ├── order.go                          # Order, Trade, OrderInput, OrderResult
│   ├── position.go                       # Position, SIDE constants
│   ├── balance.go                        # UserBalance, BUCKET constants, методы Get/Add/Deduct
│   └── instrument.go                     # Symbol, PriceFilters
│
├── internal/
│   ├── constants/
│   │   └── constants.go                  # ALL constants (Category, OrderSide, TIF, Status, etc)
│   │
│   ├── snowflake/
│   │   └── snowflake.go                  # ID generation (atomic int64)
│   │
│   ├── utils/
│   │   └── math.go                       # Safe math (Mul, Div, MulDiv, Avg, Add, Sub)
│   │
│   ├── pool/
│   │   └── pool.go                       # Object pooling (Order, Trade, OrderResult)
│   │
│   ├── state/
│   │   ├── engine_state.go               # Global State (Users + Containers map)
│   │   └── order_store.go                # Order storage per user
│   │
│   ├── balance/
│   │   ├── balance.go                    # Add, Deduct, Move, Get (map[int8]int64 buckets)
│   │   └── margin.go                     # CalculateMargin
│   │
│   ├── position/
│   │   ├── position.go                   # Position CRUD, UpdatePosition, CalculateRisk
│   │   └── risk.go                       # CheckLiquidation, AdjustReduceOnlyOrders
│   │
│   ├── orderbook/
│   │   ├── orderbook.go                  # AddOrder, RemoveOrder, GetDepth, WouldCross
│   │   └── heap.go                       # Price-level heap (price-time priority)
│   │
│   ├── matching/
│   │   └── engine.go                     # matchOrder, matchAtLevel
│   │
│   ├── trigger/
│   │   ├── monitor.go                    # TriggerMonitor (BUY/SELL heaps)
│   │   └── handler.go                    # OnTrigger logic
│   │
│   ├── spot/
│   │   └── spot.go                       # Spot market (no positions, direct transfer)
│   │
│   ├── linear/
│   │   └── linear.go                     # Linear market (positions, margin, leverage)
│   │
│   ├── persistence/
│   │   ├── wal/                          # Write-Ahead Log (atomic operations)
│   │   ├── snapshot/                     # Periodic snapshots (recovery)
│   │   └── outbox/                       # Async DB writes
│   │
│   ├── price/
│   │   ├── price_bands.go                # Price precision bands
│   │   └── price_feed.go                 # PriceFeed (liquidation check)
│   │
│   └── api/
│       ├── server.go                     # HTTP server
│       └── handlers/                     # REST API handlers
│
├── config/
│   └── config.go                         # Configuration loader
│
├── docs/
│   └── ARCHITECTURE.md
│
├── .env.example
├── go.mod
└── README.md
```

---

## Ключевые принципы

### 1. Balance Structure (map[int8]int64)

```go
type UserBalance struct {
    UserID  UserID
    Asset   string
    Buckets map[int8]int64  // 0=AVAILABLE, 1=LOCKED, 2=MARGIN
    Version int64
}

// Вместо switch - прямой доступ по ключу
bal.Buckets[types.BUCKET_AVAILABLE] += amount
bal.Buckets[types.BUCKET_LOCKED] -= amount
```

### 2. Spot vs Linear Isolation

```go
// SPOT - только балансы, нет позиций
spot.PlaceOrder(input)  // Deduct/Add между пользователями

// LINEAR - балансы + позиции + маржа
linear.PlaceOrder(input)  // Deduct/Add + UpdatePosition
```

### 3. Per-Symbol Containers

```go
type SymbolContainer struct {
    Symbol          string
    Category        int8  // SPOT=0, LINEAR=1
    
    SpotOrderBook   *OrderBookState
    LinearOrderBook *OrderBookState
    
    SpotTriggers    *TriggerHeap   // только для LINEAR
    LinearTriggers  *TriggerHeap
}

// Доступ по строке (O(1) через map)
container := state.Containers["BTCUSDT"]
container.SpotOrderBook.AddOrder(order)
```

### 4. Zero-Allocation

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
    // сброс полей
    orderPool.Put(o)
}
```

---

## Order Flow

```
API → Validate → Create Order → Match → Execute Trade → Update State → WAL
                   ↓
            (SPOT) → spot.PlaceOrder → balance.Deduct/Add
            (LINEAR) → linear.PlaceOrder → position.UpdatePosition + margin
```

---

## Performance Targets

| Operation | Target | Technique |
|-----------|--------|-----------|
| PlaceOrder | < 500μs | Pooling, no allocations |
| MatchOrder | < 200μs | Lock-free, cache-friendly |
| TradeExec | < 300μs | Atomic ops, minimal sync |
| PriceTick | < 100μs | Direct memory access |

---

## Memory Model

```
Symbols: 1000-2000
Users per symbol: 100
Orders per user: 50-100

Total orders in memory: ~10-20M
Memory with pooling: ~500-700 MB
GC pauses: < 1ms
```

---

## Константы

```go
// Category
CATEGORY_SPOT   = 0
CATEGORY_LINEAR = 1

// Order
ORDER_TYPE_LIMIT  = 0
ORDER_TYPE_MARKET = 1

ORDER_SIDE_BUY  = 0
ORDER_SIDE_SELL = 1

// TIF
TIF_GTC       = 0
TIF_IOC       = 1
TIF_FOK       = 2
TIF_POST_ONLY = 3

// Status
ORDER_STATUS_NEW = 0
ORDER_STATUS_PARTIALLY_FILLED = 1
ORDER_STATUS_FILLED = 2
ORDER_STATUS_CANCELED = 3
ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
ORDER_STATUS_UNTRIGGERED = 5
ORDER_STATUS_TRIGGERED = 6

// Balance Buckets
BUCKET_AVAILABLE = 0
BUCKET_LOCKED = 1
BUCKET_MARGIN = 2

// Position Side
SIDE_NONE  = -1
SIDE_LONG  = 0
SIDE_SHORT = 1
```

---

## Status Codes

| Status | Description | Final? |
|--------|-------------|--------|
| NEW | В книге | No |
| PARTIALLY_FILLED | Частично исполнен | No |
| FILLED | Полностью исполнен | Yes |
| CANCELED | Отменён | Yes |
| PARTIALLY_FILLED_CANCELED | Частично + отмена | Yes |
| UNTRIGGERED | STOP ждёт trigger | No |
| TRIGGERED | STOP сработал | No |
