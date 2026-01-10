# Trading Engine Architecture

## Overview

High-performance trading engine optimized for:
- **1000+ markets** (SPOT + LINEAR)
- **100 users per market** (scaled to 1000+ concurrent)
- **Minimal resource usage** ($10/month server)
- **Atomic consistency** (no partial updates on crash)
- **Eventual durability** (accept small data loss, never corrupt data)

## Design Principles

### 1. Atomic Consistency (Non-Negotiable)

**Guarantee:** Either entire operation succeeds or it fails completely. No partial state.

```
Operation Flow:
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Validate    │ ──► │ WAL START    │ ──► │ Execute      │ ──► │ WAL COMMIT   │
│             │     │ (atomic)     │     │ (in-memory)  │     │              │
└─────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
       │                   │                    │                    │
       ✗                   │                    │                    ▼
       │                   ▼                    ▼            ┌──────────────┐
       │            ┌──────────────┐     ┌──────────────┐    │ Async DB     │
       │            │ WAL ABORT    │     │ Error?       │    │ Write        │
       │            │ (noop)       │ ◄── │              │    │ (eventual)   │
       │            └──────────────┘     └──────────────┘    └──────────────┘
       │                   │                    │
       │                   ▼                    ▼
       │            ┌──────────────┐     ┌──────────────┐
       └──────────►│ Return Error │     │ WAL ABORT    │
                   │ to Client    │     │ (rollback)   │
                   └──────────────┘     └──────────────┘
```

### 2. WAL + Snapshot Architecture

**Memory State** is the source of truth during operation.
**WAL (Write-Ahead Log)** guarantees atomicity.
**Snapshots** provide durability for recovery.
**Outbox** provides eventual persistence to database.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         In-Memory State                              │
│  ┌───────────────┐  ┌───────────────┐  ┌─────────────────────────┐  │
│  │ Order Books   │  │ User Balances │  │ User Positions          │  │
│  │ (1000+ × 2)   │  │ (per user)    │  │ (per symbol × user)     │  │
│  │ Hot in RAM    │  │ Hot in RAM    │  │ Hot in RAM              │  │
│  └───────────────┘  └───────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                              │
               ┌──────────────┴──────────────┐
               ▼                              ▼
       ┌──────────────┐               ┌──────────────┐
       │   WAL Log    │               │   Outbox     │
       │ (append-only)│               │   File       │
       │ Atomicity    │               │ (batched DB) │
       └──────────────┘               └──────────────┘
               │                              │
               ▼                              ▼
       ┌──────────────┐               ┌──────────────┐
       │  Snapshots   │               │   Database   │
       │  (periodic)  │               │  (eventual)  │
       │  Recovery    │               │  Analytics   │
       └──────────────┘               └──────────────┘
```

### 3. Crash Recovery Protocol

```
Startup Sequence:
┌─────────────────────────────────────────────────────────────────────┐
│ 1. Load latest snapshot (if exists)                                 │
│    → Restores in-memory state to snapshot point                     │
│                                                                      │
│ 2. Read WAL log after snapshot                                      │
│    → Replays all operations in order                                │
│                                                                      │
│ 3. Validate state consistency                                       │
│    → Checksum verification                                          │
│                                                                      │
│ 4. Resume normal operation                                          │
│                                                                      │
│ Outcome:                                                            │
│ - At most N seconds of data lost (snapshot interval)               │
│ - Zero corrupted/inconsistent state                                 │
│ - All committed operations preserved                               │
└─────────────────────────────────────────────────────────────────────┘
```

### 4. Performance Optimizations

#### Zero-Allocation Design
```go
// Pre-allocated memory pools
var (
    orderPool = sync.Pool{New: func() interface{} { return new(Order) }}
    levelPool = sync.Pool{New: func() interface{} { return new(OrderPriceLevel) }}
    tradePool = sync.Pool{New: func() interface{} { return new(Trade) }}
)

// Index-based references (no object pointers)
type Order struct {
    ID        OrderID
    UserID    UserID
    Symbol    SymbolID
    Price     Price
    Quantity  Quantity
    Filled    Quantity
    Next      OrderID  // Linked list for price level
    Prev      OrderID
}
```

#### User Queue Serialization
```go
// Per-user operation queue prevents race conditions
type UserQueue struct {
    userID    UserID
    ops       chan Operation // Buffered channel
    processing atomic.Bool
}

// All operations for a user are serialized through this queue
// No locks needed - single goroutine processes all ops
```

## Data Model

### Market Categories

| Category | Positions | Margin | ReduceOnly | Examples |
|----------|-----------|--------|------------|----------|
| **SPOT** | ❌ | ❌ | ❌ | BTC/USDT |
| **LINEAR** | ✅ | ✅ | ✅ | BTC/USDT-PERP |

### Order Types

| Type | TIF | Balance Lock | Order Book | Description |
|------|-----|--------------|------------|-------------|
| **LIMIT** | GTC | ✅ | ✅ | Price + time priority |
| **LIMIT** | IOC | ❌ | ❌ | Fill immediately or cancel |
| **LIMIT** | FOK | ❌ | ❌ | Fill completely or cancel |
| **LIMIT** | POST_ONLY | ✅ | ✅/REJECT | Only as maker, or reject |
| **MARKET** | - | ❌ | ❌ | Fill at best available price |

### Balance Model

```
User Balance = AVAILABLE + LOCKED + MARGIN

- AVAILABLE: Ready for trading
- LOCKED: Reserved for open orders (price × quantity)
- MARGIN: Used for positions (LINEAR only)

Operations:
┌─────────────────────────────────────────────────────────────────────┐
│ Order Create (LIMIT GTC/POST_ONLY):                                 │
│   AVAILABLE → LOCKED (price × quantity)                             │
│                                                                      │
│ Order Fill (partial):                                               │
│   LOCKED → AVAILABLE (remaining)                                    │
│   AVAILABLE → (decrease)                                            │
│                                                                      │
│ Order Cancel:                                                       │
│   LOCKED → AVAILABLE                                                │
│                                                                      │
│ Position Update:                                                    │
│   MARGIN adjusted based on PnL                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Position Model (LINEAR only)

```
Position = size × entry_price

Long Position:  size > 0
Short Position: size < 0

ReduceOnly Validation:
  order.quantity ≤ |position.size|

On Fill:
  newSize = currentSize ± filledQuantity
```

## API Endpoints

### REST API

```
POST   /api/v1/orders                  - Place order
DELETE /api/v1/orders/{id}             - Cancel order
PATCH  /api/v1/orders/{id}             - Amend order
GET    /api/v1/balances?asset=...      - Get balances
GET    /api/v1/category/symbol/position - Get position
PATCH  /api/v1/category/symbol/position/leverage - Set leverage
PATCH  /api/v1/category/symbol/position/tpsl - Set TP/SL
GET    /api/v1/category/symbol/instrument - Get instrument data
```

### WebSocket

```
wss://host/stream
→ Subscribe: {"action": "subscribe", "symbols": ["BTCUSDT", "ETHUSDT"]}
→ Price Update: {"type": "ticker", "symbol": "BTCUSDT", "price": 50000}
→ Order Update: {"type": "order", "orderId": "xxx", "status": "FILLED"}
```

## Outbox Worker Pattern

```
┌─────────────────────────────────────────────────────────────────────┐
│ Outbox File Format (JSONL - JSON Lines)                             │
│                                                                      │
│ {"type":"ORDER_CREATED","ts":1699999999,"data":{...}}              │
│ {"type":"ORDER_FILLED","ts":1699999999,"data":{...}}               │
│ {"type":"POSITION_UPDATED","ts":1699999999,"data":{...}}           │
│                                                                      │
│ Worker Process:                                                     │
│ 1. Read last processed position in file                             │
│ 2. Read N lines (batch size)                                        │
│ 3. Parse and validate each event                                    │
│ 4. Write batch to database                                          │
│ 5. Truncate processed lines from file                               │
│ 6. Repeat                                                           │
└─────────────────────────────────────────────────────────────────────┘
```

## Scaling Strategy

### Current (1000 markets × 100 users)

```
Memory Estimate:
- Order Books: 1000 × 2 × 1KB = 2MB
- User Balances: 1000 × 100 × 128B = 12.5MB
- User Positions: 1000 × 100 × 64B = 6.25MB
- Orders: 1000 × 100 × 10 × 256B = 256MB
─────────────────────────────────────
Total: ~300MB RAM

CPU: 1-2 cores (matching is O(log n))
Disk: Minimal (WAL + snapshots + outbox)
```

### Future Scaling

```
1. Hot/cold market separation (active markets in memory)
2. Distributed cache for user state
3. Read replicas for analytics
```

## Error Handling

### Guaranteed Atomicity

```go
func (e *Engine) ExecuteOperation(op Operation) error {
    // Step 1: Write START to WAL
    if err := e.wal.Start(op.ID); err != nil {
        return err
    }

    // Step 2: Execute in memory
    result, err := op.Execute(e.state)
    if err != nil {
        // Step 3a: ABORT on error
        e.wal.Abort(op.ID)
        return err
    }

    // Step 3b: COMMIT on success
    e.wal.Commit(op.ID, result)

    // Step 4: Async outbox write
    e.outbox.Enqueue(result.Events)

    return nil
}
```

### Crash Safety

```
After crash and recovery:
┌─────────────────────────────────────────────────────────────────────┐
│ ✓ All committed WAL entries are applied                             │
│ ✓ No partially applied operations                                   │
│ ✓ Balance = LOCKED + AVAILABLE + MARGIN (consistent)               │
│ ✓ Position sizes match trade history                                │
│ ✓ Order book matches filled orders                                  │
│                                                                      │
│ Lost Data:                                                          │
│ - Uncommitted operations (last N seconds)                          │
│ - Outbox events not yet written to DB                               │
│ - Market data between snapshot and crash                            │
└─────────────────────────────────────────────────────────────────────┘
```

## Testing Strategy

Based on AGENTS_TEST_CASES.md:

### Critical Tests (Must Pass)

| Rule | Test | Priority |
|------|------|----------|
| 2.1-2.7 | Balance lock/unlock | P0 |
| 3.1-3.5 | Order types (IOC/FOK/POST_ONLY) | P0 |
| 3.6 | Price-time priority | P0 |
| 5.1-5.3 | User serialization | P0 |
| 4.1-4.5 | Position management | P1 |
| 4.3 | ReduceOnly validation | P1 |

## Deployment

### Minimal Server ($10/month)

```
Hardware: 2 vCPU, 4GB RAM, 50GB SSD
OS: Ubuntu 22.04 LTS

Services:
- Trading Engine (Go binary)
- PostgreSQL (for analytics/reporting)
- Redis (caching + pub/sub)
- NATS (optional, for external feed)
```

### Monitoring

```
Key Metrics:
- Order latency (p50, p95, p99)
- Matching engine throughput
- Memory usage (RSS)
- WAL log size
- Outbox lag
- Error rates
```
