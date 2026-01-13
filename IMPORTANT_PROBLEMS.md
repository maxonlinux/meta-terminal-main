# IMPORTANT PROBLEMS - Meta Terminal Go Rewrite

## Living Document for Zero-Lock High-Performance Trading Engine

**This document guides the complete rewrite of the trading engine.**

**Permissions granted:**
- Rewrite ENTIRE codebase from scratch
- Change architecture, API, structure
- Delete any file, code, feature
- Create new packages, files, services
- Break backward compatibility
- Simplify and reduce code

**Goals:**
- Zero locks in hot path
- Zero allocations in critical path
- Fewest lines of code possible
- Fewest files possible
- Maximum performance

1. The biggest problem of the app are reduce-only and close-on-trigger orders (ro and cot) and that's why we decided to change the architecture
2. The problem is that when position is reduced, all orders that are tied to this position and which are reduce-only (cot are also reduce only with the only difference that they are not yet in the orderbook and therefore they dont participate in matching yet)
3. Basically, the business-logic restriction is that:
- reduce-only orders cannot increase position size!!
- every maker (order that is in the book waiting for match) must be able to provide all the quantity to taker (new order that is being matched with that maker). That means that if theres an untrimmed reduce-only order in the book and it matches with taker and fully fills (because taker requested full size), the position will be flipped and we cant afford this because it breaks redouce only guarantee!!!
4. So, when position is reduced (or closed which is also reduced) we must loop over all these orders and trim their quantity to be sum(total_qty_of_ro_and_cot) <= position_size
5. During the time orders are trimmed ANY ORDER SHOULD MATCH WITH THOSE ONES THAT ARE BEING TRIMMED AND IN ANY TIME BETWEEN POSITION IS REDUCED AND THE TRIMMING PROCESS STARTED!!! SO WE HAVE TO LOCK THE ENTIRE PROCESS OF POSITION REDUCTION FROM START TO FINISH!!!!

---

## Development Principles (IMPORTANT - Read First)

### We Have Full Freedom to Rewrite

**This document grants permission to:**
- ✅ Rewrite ENTIRE codebase from scratch
- ✅ Change architecture completely (we're already doing this)
- ✅ Modify or replace API contracts
- ✅ Restructure files and packages
- ✅ Delete any file, any code, any feature
- ✅ Create new files, packages, services
- ✅ Break backward compatibility
- ✅ Remove "legacy" code without migration

**Philosophy: If it makes the code simpler, faster, or cleaner - DO IT.**

### Simplification Goals

- **Fewer lines of code** - every line must justify its existence
- **Fewer abstractions** - don't over-engineer
- **Fewer files** - consolidate related logic
- **Fewer locks** - towards zero-lock architecture
- **Fewer allocations** - zero allocation in hot path
- **Fewer complexity** - KISS (Keep It Simple, Stupid)

### Code Style Requirements

**Comments:**
- ✅ ALL new code edits must have English comments
- ✅ Comments explain WHY, not WHAT (WHAT is obvious from code)
- ✅ Comments for non-obvious business logic
- ✅ Comments for complex algorithms
- ✅ No comments for trivial code
- ✅ Update existing comments when editing code

---

## MCP Tools & Research Guidelines

**When working on this project, use MCP tools to:**

### 1. Search for Fresh Information
- Use `codesearch` to find real-world code examples
- Use `websearch` for current best practices
- Use `webfetch` to read articles and documentation
- Use `deepwiki_ask_question` to query GitHub repo docs

### 2. When Something Doesn't Work
- Search for similar issues on GitHub
- Look for solutions in related repositories
- Fetch relevant documentation
- Find working examples of the same pattern

### 3. Research Before Implementing
- Search for existing implementations of patterns
- Find performance benchmarks
- Look for common pitfalls
- Check for updated documentation

### Available MCP Tools
| Tool | Purpose |
|------|---------|
| `codesearch` | Find real-world code examples from GitHub |
| `websearch` | Search the web for articles and docs |
| `webfetch` | Fetch and parse specific URLs |
| `deepwiki_ask_question` | Query GitHub repository docs |
| `github_search_code` | Search code in GitHub repos |
| `context7_query-docs` | Query library documentation |

### Example Usage
```bash
# When implementing a new feature, search for examples first
codesearch query="LMAX Disruptor Go implementation"
codesearch query="lock-free priority queue Go"

# When something doesn't work, search for solutions
websearch query="Go atomic.Value vs mutex performance 2024"

# When researching patterns
webfetch url="https://pkg.go.dev/container/heap"
```

**Rule: Don't guess - SEARCH FIRST.**

**Example:**
```go
// BAD: Redundant comment (what is obvious from code)
// Increment counter
counter++

// GOOD: Explain WHY
// Decrement available balance - this is a reserve release after partial fill
available -= filledQty

// GOOD: Explain complex logic
// Heap ordering: SELL orders - highest price first (furthest from market)
// This matches Bybit's "furthest-first" trimming priority rule
```

**File Structure:**
- Consolidate related logic into fewer files
- One package per domain, not per layer
- Remove unused code immediately (no "we might need it later")

---

## Bybit's Reduce-Only and Close-On-Trigger Trimming Rules

### Reduce-Only Orders (Official Bybit Documentation)

**Source:** https://www.bybit.com/en/help-center/article/Reduce-Only-Order

**Purpose:** Ensure orders only reduce position size, never increase it.

**Core Rules:**

1. **No position = No reduce-only orders allowed**
   - System automatically rejects placing reduce-only orders when no open position exists

2. **Position Mode Restriction**
   - Reduce-Only only available in **One-Way Mode** (not Hedge Mode)
   - In Hedge Mode, you hold both directions, so reduce-only concept doesn't apply

3. **New reduce-only order placement:**
   - Without existing RO orders: New RO order qty + active orders qty (nearer to market) ≤ position size
   - With existing RO orders: New RO order qty + active orders qty (nearer to market) ≤ position size
   - Otherwise: New RO order qty is automatically reduced or cancelled

4. **Priority for trimming/cancellation (CRITICAL):**
   - **WITHOUT existing RO orders:** New RO orders exceeding position are reduced/cancelled
   - **WITH existing RO orders:** RO order with **furthest order price from current market price** is automatically reduced/cancelled **first**
   - This is the OPPOSITE of FIFO - Bybit trims furthest-first!

5. **Position reduction triggers automatic trimming:**
   - When position size decreases, all RO orders are re-evaluated
   - Total RO order quantity must always ≤ current position size
   - Orders furthest from market price are trimmed first

6. **Close-by function (built-in reduce-only):**
   - The "close by limit/market price" function in Position tab has Reduce-only embedded by default
   - Priority of execution
   - In event of insufficient margin: **active orders with FURTHEST order price from current market price** are cancelled first

### Close-On-Trigger Orders (Official Bybit Documentation)

**Source:** https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order

**Purpose:** Guaranteed position close when trigger condition is met, regardless of margin.

**Core Rules:**

1. **Only for Conditional Orders**
   - CoT is an option for Conditional Orders (Market or Limit)
   - When triggered, creates a Reduce-Only order to close position

2. **Auto-adjust on trigger:**
   - If CoT order qty > current position size → qty auto-adjusted to position size
   - If position = 0 → order cancelled (prevents opening opposite position)

3. **No margin required:**
   - CoT orders don't require margin for execution upon trigger
   - System guarantees execution to close position

4. **Conditional Market Order:**
   - Executes at best available price upon trigger
   - Fills even with insufficient margin

5. **Conditional Limit Order:**
   - Places Reduce-Only Limit Order into book upon trigger
   - No guaranteed execution (standard limit order behavior)

6. **Trigger evaluation before execution:**
   - System checks if order will reduce position qty
   - If qty exceeds position → auto-adjust
   - If position = 0 → cancel

### OCO Orders (One-Cancels-the-Other)

**Source:** https://www.bybit.com/en/help-center/article/One-Cancels-the-Other-OCO-Orders

**Purpose:** Pair two conditional orders where triggering one cancels the other.

**Core Rules:**

1. **Two linked conditional orders:** Take Profit + Stop Loss
2. **Automatic sibling cancellation:** When one triggers, other is cancelled
3. **Quantity linkage:** Both share same quantity (or use full position)
4. **Both are Reduce-Only by design:** TP and SL are closing orders

**In our implementation:**
- OCO uses `orderLinkId` to link TP and SL orders
- When one triggers → `deactivateLinkedOrders()` cancels sibling
- Both get `CloseOnTrigger=true` and `ReduceOnly=true`

---

## Our Implementation Analysis

### Current Implementation vs Bybit Spec

| Feature | Implemented | Bybit Spec | Status |
|---------|-------------|------------|--------|
| RO order trimming | ✅ | ✅ | PARTIAL |
| CoT auto-adjust | ✅ | ✅ | MATCH |
| OCO sibling cancellation | ✅ | ✅ | MATCH |
| Position=0 CoT cancel | ✅ | ✅ | MATCH |
| **Trimming priority** | FIFO | **Furthest-first** | ❌ MISMATCH |
| **OCO sibling trimming** | ❌ | N/A (OCO auto-cancels) | ❌ |

### Critical Gap: Trimming Priority

**Current Implementation (WRONG):**
```go
// internal/oms/positions.go
// Trimming in iteration order (FIFO)
for _, order := range reduceOnlyOrders {
    // trim order
}
```

**Bybit Spec (CORRECT):**
```
Trim orders with FURTHEST order price from current market price FIRST
```

**Why this matters:**
- SELL RO orders: Higher price = further from market (trim first)
- BUY RO orders: Lower price = further from market (trim first)
- This ensures orders "nearer to market" get priority to execute

### Position Mode Considerations

**Bybit:**
- Reduce-Only only works in One-Way Mode
- In Hedge Mode, you can hold both long and short

**Our Implementation:**
- Currently doesn't distinguish between One-Way and Hedge Mode
- Should add validation: Reject RO orders if in Hedge Mode

---

## Trimming Algorithm (To Be Fixed)

### Current (INCORRECT - FIFO):

```
1. On OnPositionUpdate:
2. Collect all RO orders
3. Trim in iteration order (WRONG!)
4. Result: Orders near market might get trimmed instead of furthest
```

### Correct (Bybit-Compliant - Furthest-First):

```
1. On OnPositionUpdate:
2. Collect all RO orders
3. Sort by price distance from current market:
   - SELL: highest price first
   - BUY: lowest price first
4. Trim in sorted order (CORRECT!)
5. Result: Orders near market get priority
```

### Algorithm Detail

```go
func (s *Service) adjustReduceOnlyOrdersWithOB(...) {
    // Sort orders by distance from market price
    // SELL orders: sort DESC by price (highest first)
    // BUY orders: sort ASC by price (lowest first)
    
    sort.Slice(orders, func(i, j int) bool {
        if orders[i].Side == constants.ORDER_SIDE_SELL {
            return orders[i].Price > orders[j].Price // furthest first
        }
        return orders[i].Price < orders[j].Price // lowest first
    })
    
    // Then trim in sorted order
    for _, order := range orders {
        // trim logic
    }
}
```

---

## Lock Ordering (CRITICAL - Current Implementation)

**NOTE:** This section describes the CURRENT implementation. The NEW ENGINE ARCHITECTURE (below) proposes a fundamental redesign to single-threaded processing, eliminating locks entirely.

**In the NEW ENGINE, these become OBSOLETE:**
- `posMu` - No concurrent position updates
- `mu` - No concurrent order access
- `ob.mu` - No concurrent orderbook access
- `order.mu` - No concurrent order modifications
- All snapshot methods - Direct field access is safe

To prevent deadlock, always acquire locks in this order (CURRENT only):

```
1. posMu - Position update lock
2. mu - OMS main lock
3. ob.mu - Orderbook lock
4. order.mu - Individual order lock
```

**Why this order:**
- `posMu` first: Position update is the trigger
- `mu` second: Protects order collection/lookup
- `ob.mu` third: Orderbook operations
- `order.mu` last: Individual order modifications

**Transition to New Architecture:**
When implementing the Zero-Lock Engine (Section 1), this lock ordering becomes OBSOLETE because the single-threaded event loop eliminates concurrent access.

---

## Known Issues & TODOs (Prioritized)

### HIGH Priority

1. **Fix trimming priority to furthest-first**
   - Sort RO orders by price distance from market before trimming
   - SELL: highest price first, BUY: lowest price first
   - Impact: Bybit compliance, prevents incorrect trimming

2. **Add Position Mode validation**
   - Reject RO orders if in Hedge Mode
   - Bybit only allows RO in One-Way Mode
   - Impact: Compliance, prevents edge cases

### MEDIUM Priority

3. **OCO sibling handling during trim**
   - When trimming one OCO sibling, should sibling be affected?
   - Bybit: OCO auto-cancels on trigger, not on trim
   - Decision: Keep current behavior (sibling unchanged during trim)

4. **CoT quantity validation on trigger**
   - Bybit: Auto-adjust qty to position size
   - Current: `createChildOrderInputFromSnapshot` does this
   - Verify: Check edge cases (position changed since trigger)

### LOW Priority

5. **Margin handling for CoT**
   - Bybit: CoT doesn't require margin
   - Current: Need to verify clearing service
   - Impact: Edge case for insufficient margin

---

## Reference Links

- Bybit Reduce-Only Orders: https://www.bybit.com/en/help-center/article/Reduce-Only-Order
- Bybit Close-On-Trigger: https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order
- Bybit OCO Orders: https://www.bybit.com/en/help-center/article/One-Cancels-the-Other-OCO-Orders
- Bybit Order Types: https://www.bybit.com/en/help-center/article/Types-of-Orders-Available-on-Bybit
- Bybit Getting Started: https://www.bybit.com/pl-PL/help-center/article/How-to-Get-Started-With-Perpetual-and-Futures-Trading/

---

## Executive Summary: Implementation Gap Analysis

### Current State vs Bybit Specification

| Feature | Status | Implementation | Bybit Spec | Gap |
|---------|--------|----------------|------------|-----|
| RO trimming | ✅ | Works | Works | None |
| CoT auto-adjust | ✅ | Works | Works | None |
| OCO sibling cancel | ✅ | Works | Works | None |
| Position=0 cancel | ✅ | Works | Works | None |
| **Trimming priority** | ✅ | **Furthest-first** | **Furthest-first** | NONE |
| **Position Mode** | ✅ | **Validation added** | **One-Way only** | NONE |
| **SPOT/LINEAR isolation** | ✅ | **Complete isolation** | **Complete isolation** | NONE |

### Priority 1: Fix Trimming Priority - ✅ IMPLEMENTED

**Status:** COMPLETED in `core/engine.go`

**Implementation:**
- Updated `trimRO()` function to use furthest-first trimming
- For LONG positions: trim SELL orders starting with highest price (furthest from market)
- For SHORT positions: trim BUY orders starting with lowest price (furthest from market)
- Uses existing heap structures that maintain correct ordering

**Code Location:** `core/engine.go:279-310`

### Priority 2: Add Position Mode Validation - ✅ IMPLEMENTED

**Status:** COMPLETED in `core/engine.go`

**Implementation:**
- Added position mode constants in `internal/constants/constants.go`
- Added `PositionMode` field to Engine struct
- Added `SetPositionMode()` method for configuration
- Added validation in `handleOrder()` to reject reduce-only orders in Hedge Mode
- Default mode is One-Way (Bybit-compliant)

**Code Location:** 
- Constants: `internal/constants/constants.go:50-51`
- Engine field: `core/engine.go:20`
- Validation: `core/engine.go:171-175`

### Priority 3: Add Test for Trimming Priority - ✅ IMPLEMENTED

**Status:** COMPLETED in `core/engine_test.go`

**Implementation:**
- Added comprehensive test suite in `core/engine_test.go`
- Tests for trimming priority logic
- Tests for position mode validation
- Tests for market isolation

**Code Location:** `core/engine_test.go` and `core/market_isolation_test.go`

## 🎯 IMPLEMENTATION COMPLETE - Summary

### ✅ What Has Been Successfully Implemented

1. **Trimming Priority Fix (HIGH Priority)**
   - ✅ Changed from FIFO to furthest-first trimming
   - ✅ Bybit-compliant: SELL orders trimmed highest price first, BUY orders trimmed lowest price first
   - ✅ Uses efficient heap structures for O(log n) operations

2. **Position Mode Validation (MEDIUM Priority)**
   - ✅ Added position mode constants (One-Way vs Hedge Mode)
   - ✅ Added PositionMode field to Engine
   - ✅ Added SetPositionMode() method
   - ✅ Validation rejects reduce-only orders in Hedge Mode
   - ✅ Default mode is One-Way (Bybit standard)

3. **SPOT vs LINEAR Market Isolation (CRITICAL - Added)**
   - ✅ Complete separation of SPOT and LINEAR order books
   - ✅ Category field added to OrderRequest and Order structs
   - ✅ Market-specific validation rules implemented
   - ✅ SPOT: No reduce-only, no conditional orders
   - ✅ LINEAR: Supports reduce-only, conditional orders, proper TIF validation

4. **Comprehensive Test Coverage**
   - ✅ All tests pass with -race flag (no data races)
   - ✅ Trimming priority tests
   - ✅ Position mode validation tests
   - ✅ Market isolation tests
   - ✅ SPOT/LINEAR validation tests

### 📁 Files Modified/Created

**Modified Files:**
- `core/engine.go` - Main engine with trimming, validation, and market isolation
- `core/orderbook.go` - Added Category field to Order struct
- `internal/constants/constants.go` - Added position mode constants

**New Files Created:**
- `core/engine_test.go` - Basic engine tests
- `core/market_isolation_test.go` - Market isolation and validation tests

### 🚀 Performance Characteristics

- **Zero Locks**: Maintains single-threaded event loop architecture
- **O(log n) Trimming**: Uses heap structures for efficient trimming
- **O(1) Market Routing**: Direct mapping to SPOT/LINEAR books
- **Race-Free**: All tests pass with -race detector

### 🎯 Bybit Compliance Status

| Requirement | Status | Notes |
|------------|--------|-------|
| Furthest-first trimming | ✅ COMPLETE | Implemented in trimRO() |
| Position mode validation | ✅ COMPLETE | Hedge mode rejects RO orders |
| SPOT market rules | ✅ COMPLETE | No RO, no conditional orders |
| LINEAR market rules | ✅ COMPLETE | Supports RO, conditional, proper TIF |
| Market isolation | ✅ COMPLETE | Separate order books and validation |

### 🔮 Next Steps (Optional Enhancements)

1. **Error Handling**: Return proper error messages instead of silent rejection
2. **Integration Tests**: Connect with actual order matching and execution
3. **Performance Benchmarks**: Verify no regression in critical path
4. **Edge Case Testing**: Complex scenarios with simultaneous operations
5. **Documentation**: Add detailed comments for complex algorithms

---

## Test Scenarios to Add

1. **Trimming priority test:**
   - Create 3 SELL RO orders: price 55000, 54000, 53000
   - Position = 4
   - Expect: 55000 trimmed first, then 54000
   - Result: Orders near market (53000) should remain

2. **CoT with shrinking position:**
   - Create CoT order qty=10
   - Position updates to 5 before trigger
   - Expect: Child order qty=5

3. **OCO trigger sibling cancel:**
   - Create OCO TP=55000, SL=45000
   - Price hits 55000
   - Expect: TP triggers, SL cancelled

4. **Position=0 CoT cancel:**
   - Create CoT order
   - Position closes before trigger
   - Expect: Order cancelled on trigger attempt

5. **Position Mode validation test:**
   - Try to place RO order in Hedge Mode
   - Expect: Error returned

---

## Implementation Checklist

### Before Code Freeze

- [x] Fix trimming priority to furthest-first ✅
- [x] Add Position Mode validation ✅
- [x] Add trimming priority test ✅
- [x] Add Position Mode validation test ✅
- [x] All tests pass with `-race` flag ✅

### Code Changes Required

| File | Change | Priority | Status |
|------|--------|----------|--------|
| `internal/oms/positions.go` | Sort orders by price before trimming | HIGH | ❌ OBSOLETE (new architecture) |
| `internal/oms/validation.go` | Add Position Mode check | MEDIUM | ❌ OBSOLETE (new architecture) |
| `internal/oms/service.go` | Add positionMode field | MEDIUM | ❌ OBSOLETE (new architecture) |
| `tests/oms/chaosparallel_test.go` | Add trimming priority test | HIGH | ❌ OBSOLETE (new architecture) |
| `tests/oms/validation_test.go` | Add Position Mode test | MEDIUM | ❌ OBSOLETE (new architecture) |

**NEW ARCHITECTURE IMPLEMENTATION:**
| File | Change | Priority | Status |
|------|--------|----------|--------|
| `core/engine.go` | Fix trimming priority to furthest-first | HIGH | ✅ COMPLETED |
| `core/engine.go` | Add Position Mode validation | MEDIUM | ✅ COMPLETED |
| `core/engine.go` | Add SPOT/LINEAR market isolation | HIGH | ✅ COMPLETED |
| `core/engine_test.go` | Add trimming priority tests | HIGH | ✅ COMPLETED |
| `core/market_isolation_test.go` | Add market isolation tests | HIGH | ✅ COMPLETED |

---

# 🚀 NEW ENGINE ARCHITECTURE: Zero-Lock High-Performance Design

## Executive Summary

Перепроектируем архитектуру с нуля для достижения:
- **Zero locks** - никаких мьютексов в hot path (single-threaded event loop)
- **O(log n)** insert для RO/COT trimming через Priority Queue
- **Single-threaded event loop** как LMAX Disruptor
- **Memory efficient** - zero allocation в critical path
- **No snapshots** - direct field access safe in single thread

**WE HAVE FULL FREEDOM TO REWRITE:**
- Delete ANY code, ANY file, ANY package
- Rewrite architecture from scratch
- Change API contracts
- Consolidate files (fewer is better)
- Remove unused features
- Simplify wherever possible

**What becomes OBSOLETE in new architecture:**

| Component | Current | New Engine |
|-----------|---------|------------|
| `order.mu` (RWMutex) | Thread-safe access | ❌ Not needed |
| `order.active` (atomic.Bool) | Prevent pooled access | ❌ Not needed |
| `order.Snapshot()` | Copy fields safely | ❌ Not needed |
| `order.IsActive()` | Check if pooled | ❌ Not needed |
| `order.Deactivate()/Activate()` | Mark active state | ❌ Not needed |
| All snapshot methods | Thread-safe reads | ❌ Not needed |
| Lock ordering | posMu → mu → ob.mu → order.mu | ❌ Not needed |

**In single-threaded mode:**
- Direct field access is 100% safe (no concurrent goroutines)
- No need for locks, atomics, or snapshots
- Simplifies code significantly

**Code Style for New Implementation:**
- Comments in English for every non-trivial edit
- Consolidate related logic into fewer files
- Remove unused code immediately
- KISS - Keep It Simple, Stupid

---

## 1. LMAX Disruptor Pattern (Core Architecture)

**Source:** https://lmax-exchange.github.io/disruptor/user-guide/index.html

### Concept
```
┌─────────────────────────────────────────────────────────────┐
│                    RING BUFFER                              │
│  ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐         │
│  │ 0 │ │ 1 │ │ 2 │ │ 3 │ │ 4 │ │ 5 │ │ 6 │ │ 7 │ ...      │
│  └───┘ └───┘ └───┘ └───┘ └───┘ └───┘ └───┘ └───┘         │
│     ▲                                           │          │
│     │                                           │          │
│  single writer                            multiple          │
│  (orders, trades)                         consumers         │
│                                              │              │
│                                              ▼              │
│                                   ┌────────────────────┐   │
│                                   │  Event Processor   │   │
│                                   │  (single thread)   │   │
│                                   │                    │   │
│                                   │  • Order matching  │   │
│                                   │  • Position update │   │
│                                   │  • Trimming RO/COT │   │
│                                   │  • Trigger check   │   │
│                                   └────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Why Disruptor?
- **No locks** - single thread processes all events
- **Cache-friendly** - ring buffer with pre-allocated slots
- **Memory pre-allocation** - no GC pressure in hot path
- **10-100x faster** than traditional actor models

### Implementation

```go
// RingBuffer with pre-allocated slots
type RingBuffer struct {
    buffer []Event
    mask   uint64
    write  atomic.Uint64
    read   atomic.Uint64
}

type Event struct {
    Type      EventType
    Order     *Order
    Trade     *Trade
    Position  *PositionUpdate
    PriceTick *PriceTick
}

// Single event processor (runs in one goroutine)
func (s *Engine) runEventLoop() {
    for {
        seq := s.ringBuffer.next()
        event := s.ringBuffer.buffer[seq&s.ringBuffer.mask]
        
        switch event.Type {
        case EVENT_ORDER:
            s.handleOrder(event.Order)
        case EVENT_TRADE:
            s.handleTrade(event.Trade)
        case EVENT_POSITION_UPDATE:
            s.handlePositionUpdate(event.Position)
        case EVENT_PRICE_TICK:
            s.handlePriceTick(event.PriceTick)
        }
        
        s.ringBuffer.publish(seq)
    }
}
```

---

## 2. B-Tree Order Book (Complementing Priority Queue)

**Source:** https://medium.com/@adityaraj_201551/market-depth-simplified-building-an-order-book-engine-in-go-9abb9bcaec9a

### Key Concepts from Article

| Concept | Description | Benefit |
|---------|-------------|---------|
| **B-Tree** | Self-balancing tree with O(log n) operations | Efficient range queries |
| **PriceLevel** | Groups orders at same price | Reduces tree size |
| **BuyTree/SellTree** | Separate trees for each side | O(1) best bid/ask |
| **Less Method** | Defines sorting: buy DESC, sell ASC | Correct priority |

### B-Tree Structure from Article

```
+--------------------------------------------------------------------+
|                            Node                                    |
+--------------------------------------------------------------------+
|                         BuyTree Node                               |
+--------------------------------------------------------------------+
| Keys (Prices):                                                      |
|   [Price: 102.00]                                                   |
|   [Price: 100.00]                                                   |
+--------------------------------------------------------------------+
| Orders at Price 102.00:                                             |
|   - OrderID: B2, Quantity: 7.00                                    |
|   - OrderID: B5, Quantity: 3.00                                    |
+--------------------------------------------------------------------+
| Children Pointers:                                                  |
|   Left Child   (Prices > 102.00): Node Address                     |
|   Middle Child (Prices between 100.00 and 102.00): Node Address    |
|   Right Child  (Prices < 100.00): Node Address                     |
+--------------------------------------------------------------------+
```

### Our Implementation with google/btree

```go
import "github.com/google/btree"

// PriceLevel - groups orders at same price level
type PriceLevel struct {
    Price    Price
    Orders   []*Order // FIFO by insertion time
    Side     int8     // BUY or SELL
    Time     int64    // For tie-breaking at same price
}

// Less implements btree.Item interface
func (p PriceLevel) Less(than btree.Item) bool {
    other := than.(PriceLevel)
    if p.Side == constants.ORDER_SIDE_BUY {
        // Buy: higher prices first (Max-heap behavior)
        return p.Price > other.Price
    }
    // Sell: lower prices first (Min-heap behavior)
    return p.Price < other.Price
}

// OrderBook - B-tree based order book
type OrderBook struct {
    BuyTree  *btree.BTree  // PriceLevel with Side=BUY, sorted DESC by Price
    SellTree *btree.BTree  // PriceLevel with Side=SELL, sorted ASC by Price
    Orders   map[OrderID]*Order
}

func NewOrderBook() *OrderBook {
    return &OrderBook{
        BuyTree:  btree.New(3),  // Degree = 3
        SellTree: btree.New(3),
        Orders:   make(map[OrderID]*Order),
    }
}
```

### Order Operations

```go
func (ob *OrderBook) AddOrder(order *Order) {
    ob.Orders[order.ID] = order
    
    priceLevel := PriceLevel{
        Price: order.Price,
        Side:  order.Side,
        Time:  int64(order.CreatedAt),
    }
    
    tree := ob.BuyTree
    if order.Side == constants.ORDER_SIDE_SELL {
        tree = ob.SellTree
    }
    
    // Try to find existing price level
    item := tree.Get(priceLevel)
    if item != nil {
        existing := item.(PriceLevel)
        existing.Orders = append(existing.Orders, order)
        tree.ReplaceOrInsert(existing)
    } else {
        priceLevel.Orders = []*Order{order}
        tree.ReplaceOrInsert(priceLevel)
    }
}

func (ob *OrderBook) BestBid() *PriceLevel {
    if ob.BuyTree.Len() == 0 {
        return nil
    }
    return ob.BuyTree.Min().(PriceLevel)
}

func (ob *OrderBook) BestAsk() *PriceLevel {
    if ob.SellTree.Len() == 0 {
        return nil
    }
    return ob.SellTree.Min().(PriceLevel)
}
```

### Order Matching

```go
func (ob *OrderBook) MatchOrders() []*Trade {
    var trades []*Trade
    
    for {
        bestBuy := ob.BestBid()
        bestSell := ob.BestAsk()
        
        if bestBuy == nil || bestSell == nil {
            break
        }
        
        if bestBuy.Price < bestSell.Price {
            break // No cross
        }
        
        buyOrder := bestBuy.Orders[0]
        sellOrder := bestSell.Orders[0]
        
        // Execute trade
        tradeQty := min(buyOrder.Quantity, sellOrder.Quantity)
        trade := &Trade{
            ID:         TradeID(snowflake.Next()),
            TakerOrder: buyOrder,
            MakerOrder: sellOrder,
            Price:      sellOrder.Price, // Price of maker
            Quantity:   tradeQty,
        }
        trades = append(trades, trade)
        
        // Update quantities
        buyOrder.Quantity -= tradeQty
        sellOrder.Quantity -= tradeQty
        
        // Remove filled orders
        if buyOrder.Quantity == 0 {
            ob.RemoveOrder(buyOrder.ID)
        }
        if sellOrder.Quantity == 0 {
            ob.RemoveOrder(sellOrder.ID)
        }
    }
    
    return trades
}
```

### Complexity Analysis

| Operation | Array/List | Our B-Tree | Improvement |
|-----------|------------|------------|-------------|
| AddOrder | O(1) | O(log n) | - |
| RemoveOrder | O(n) | O(log n) | **n → log n** |
| BestBid/Ask | O(1) | O(1) | Same |
| Match | O(1) per trade | O(log n) per trade | **Same** |
| RangeQuery | O(n) | O(log n + m) | **Significant** |

---

## 3. Priority Queue for RO/COT Trimming (No Sorting!)

**Source:** https://raw.githubusercontent.com/Kautenja/limit-order-book/main/notes/lob.md

### Problem with Current Approach
```go
// OLD: Collect all, then sort - O(n log n)
orders := collectROOrders()
sort.Slice(orders, byPriceDistanceFromMarket)
trimOrders(orders)
```

### New Approach: Maintain Sorted Heaps
```go
// NEW: Keep orders sorted at all times - O(log n) insert
// SELL orders: Max-Heap by price (furthest = highest price)
// BUY orders: Min-Heap by price (furthest = lowest price)

type ReduceOnlyManager struct {
    sellHeap *PriorityQueue // Max-heap by price
    buyHeap  *PriorityQueue // Min-heap by price
    orders   map[OrderID]*ROOrderEntry
    marketPrice atomic.Int64
}

type ROOrderEntry struct {
    order      *Order
    side       int8
    price      Price
    quantity   Quantity
    heapIndex  int  // For O(1) removal
}

// When position updates, just pop from heap until total <= position
func (m *ReduceOnlyManager) trimForPosition(positionSize int64) {
    remaining := positionSize
    var toTrim []*ROOrderEntry
    
    // Determine which heap to use based on position side
    heap := m.buyHeap
    if positionSize > 0 {
        heap = m.sellHeap // Long position: trim SELL orders
    } else {
        heap = m.buyHeap // Short position: trim BUY orders
    }
    
    // Get total qty from heap
    totalQty := heap.totalQuantity()
    
    if totalQty <= remaining {
        return // Nothing to trim
    }
    
    // Pop from heap (furthest first by design!)
    for totalQty > remaining && !heap.isEmpty() {
        entry := heap.pop() // O(log n) - furthest order!
        totalQty -= entry.quantity
        toTrim = append(toTrim, entry)
    }
    
    // Trim the orders
    for _, entry := range toTrim {
        m.adjustOrderQuantity(entry, remaining)
    }
}
```

### Priority Queue Implementation (Lock-Free)

```go
// FastPriorityQueue - lock-free heap implementation
// Source: https://github.com/lemire/FastPriorityQueue.js

type FastPriorityQueue struct {
    data []*PriorityNode
    comparator func(a, b *PriorityNode) bool
}

func (q *FastPriorityQueue) Push(node *PriorityNode) {
    node.index = len(q.data)
    q.data = append(q.data, node)
    q.swim(node.index)
}

func (q *FastPriorityQueue) Pop() *PriorityNode {
    if len(q.data) == 0 {
        return nil
    }
    top := q.data[0]
    q.data[0] = q.data[len(q.data)-1]
    q.data[0].index = 0
    q.data = q.data[:len(q.data)-1]
    q.sink(0)
    return top
}

func (q *FastPriorityQueue) Peek() *PriorityNode {
    if len(q.data) == 0 {
        return nil
    }
    return q.data[0]
}

// Operations: O(log n) time, O(1) space
```

### Complexity Analysis

| Operation | Current (Sort) | New (Priority Queue) | Improvement |
|-----------|----------------|----------------------|-------------|
| Insert order | O(1) | O(log n) | - |
| Trim request | O(n log n) per trim | O(k log n) | **2-10x** |
| Get furthest | O(n) | O(1) | **n → 1** |
| Memory | O(n) | O(n) | Same |

**Note:** `k` = number of trimmed orders (typically small, often 1-2).
**Insert** is O(log n) because we maintain sorted order in the heap at all times.
**Trim** is O(k log n) because we just pop k items from the heap (no sorting needed).

---

## 4. Skip List for Order Management (Alternative)

**Source:** https://raw.githubusercontent.com/dairongpeng/algorithm-note/main/23-《进阶》有序表介绍及其原理.md

### Skip List Advantages
- O(log n) operations (like B-tree)
- Simpler implementation than B-tree
- Lock-free versions exist (used by Redis ZSET)
- Perfect for ordered price lookups

```go
// SkipList for order price management
type OrderSkipList struct {
    head   *SkipNode
    levels int
    size   int
}

type SkipNode struct {
    order   *Order
    forward []*SkipNode
    backward *SkipNode
}

// Insert: O(log n) average case
func (sl *OrderSkipList) Insert(order *Order) {
    // ... skip list insert logic
}

// Range query for orders in price range: O(log n + m)
func (sl *OrderSkipList) RangeQuery(minPrice, maxPrice Price) []*Order {
    // ... traverse levels, collect orders
}
```

### Redis ZSET Pattern
```go
// Redis uses skiplist + dict for ZSET
// We can use similar pattern:
// - SkipList: maintain sorted order by price
// - Hash table: O(1) order lookup by ID
```

---

## 5. Combined Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         API Layer                                    │
│  POST /api/v1/order  │  GET /api/v1/orders  │  WebSocket updates    │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      RingBuffer (Disruptor)                          │
│  Pre-allocated events: [Order][Trade][Position][PriceTick]          │
│  Single writer (API) → Single consumer (Engine)                     │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Engine (Single Thread)                        │
│                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │ Order Book       │  │ Position Manager │  │ RO/COT Manager   │  │
│  │                  │  │                  │  │                  │  │
│  │ • B-Tree         │  │ • Update size    │  │ • SellHeap (max) │  │
│  │ • Add/Remove     │  │ • Get position   │  │ • BuyHeap (min)  │  │
│  │ • Match orders   │  │ • PnL calc       │  │ • Trim (O log n) │  │
│  │ • Best bid/ask   │  │                  │  │                  │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘  │
│                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │ Trigger Monitor  │  │ Balance Manager  │  │ Event Publisher  │  │
│  │                  │  │                  │  │                  │  │
│  │ • Add trigger    │  │ • Reserve        │  │ • Order events   │  │
│  │ • Check triggers │  │ • Release        │  │ • Trade events   │  │
│  │ • Remove         │  │ • Execute trade  │  │ • Position evts  │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 6. Implementation Roadmap

### Phase 1: Ring Buffer + Event Loop (Week 1)

```go
// core/engine.go
type Engine struct {
    ringBuffer *RingBuffer
    orderBook  *OrderBook
    positions  *PositionManager
    roManager  *ROManager // New: priority queue based
    triggers   *TriggerMonitor
    balances   *BalanceManager
}

func NewEngine() *Engine {
    return &Engine{
        ringBuffer: NewRingBuffer(1024), // Power of 2
        orderBook:  NewOrderBook(),
        positions:  NewPositionManager(),
        roManager:  NewROManager(),
        triggers:   NewTriggerMonitor(),
        balances:   NewBalanceManager(),
    }
}

func (e *Engine) Start() {
    go e.runEventLoop()
}

func (e *Engine) PlaceOrder(input *OrderInput) error {
    event := e.ringBuffer.Next()
    event.Type = EVENT_ORDER
    event.Order = input
    e.ringBuffer.Publish(event)
    return nil
}
```

### Phase 2: Priority Queue RO Manager (Week 2)

```go
// core/ro_manager.go
type ROManager struct {
    buyHeap  *FastPriorityQueue // Min-heap by price
    sellHeap *FastPriorityQueue // Max-heap by price
    orders   map[OrderID]*ROEntry
    marketPrice atomic.Int64
}

type ROEntry struct {
    order     *Order
    quantity  Quantity
    price     Price
    side      int8
    heapNode  *PriorityNode
}

func (m *ROManager) AddOrder(order *Order) {
    entry := &ROEntry{
        order:    order,
        quantity: order.Quantity,
        price:    order.Price,
        side:     order.Side,
    }
    
    // Insert into appropriate heap
    node := &PriorityNode{Value: entry}
    if order.Side == ORDER_SIDE_SELL {
        m.sellHeap.Push(node)
    } else {
        m.buyHeap.Push(node)
    }
    
    m.orders[order.ID] = entry
}

func (m *ROManager) TrimForPosition(size int64) {
    // Get heap for position side
    var heap *FastPriorityQueue
    if size > 0 {
        heap = m.sellHeap // Long: trim SELL orders
    } else {
        heap = m.buyHeap // Short: trim BUY orders
    }
    
    // Pop until total <= position
    var toTrim []*ROEntry
    for !heap.IsEmpty() && heap.TotalQuantity() > abs(size) {
        node := heap.Pop()
        entry := node.Value.(*ROEntry)
        toTrim = append(toTrim, entry)
    }
    
    // Apply trims
    for _, entry := range toTrim {
        m.adjustOrder(entry, entry.quantity) // Full trim
    }
}
```

### Phase 3: B-Tree OrderBook (Week 3)

```go
// core/orderbook_btree.go
type BTreeOrderBook struct {
    buyTree  *btree.BTree  // google/btree
    sellTree *btree.BTree
    orders   map[OrderID]*Order
}

func (ob *BTreeOrderBook) AddOrder(order *Order) {
    // Implementation from article + our types
}

func (ob *BTreeOrderBook) MatchOrders() []*Trade {
    var trades []*Trade
    
    for {
        bestBuy := ob.BestBid()
        bestSell := ob.BestAsk()
        
        if bestBuy == nil || bestSell == nil || bestBuy.Price < bestSell.Price {
            break
        }
        
        // Match logic from article
        // ...
    }
    
    return trades
}
```

---

## 7. Performance Comparison

| Metric | Current | New Engine | Improvement |
|--------|---------|------------|-------------|
| PlaceOrder | 264ns | 100ns | **2.6x** |
| MatchOrder | 38ns | 20ns | **2x** |
| CancelOrder | 6ns | 5ns | **1.2x** |
| TrimRO (sort approach) | O(n log n) | - | - |
| TrimRO (heap approach) | - | O(k log n) | **2-10x** |
| TrimRO (heap insert) | O(1) | O(log n) | - |
| BestBidAsk | 7ns | 5ns | **1.4x** |
| **Snapshot overhead** | ~50ns per read | **0** | **Eliminated** |
| Race conditions | Possible | **Impossible** | ✅ |
| Memory allocs | Medium | **Zero** | ✅ |

**Note:** New engine metrics are TARGETS based on architectural advantages. TrimRO row split into:
- **Sort approach:** Current code collects + sorts each trim = O(n log n)
- **Heap approach (New):** Maintain sorted heap, trim = O(k log n) pop operations

**Snapshot Elimination:**
- Current: `Snapshot()` + `IsActive()` adds ~50ns overhead per trigger check
- New: Direct field access = 0 overhead (no lock acquisition, no copying)

---

## 8. Key Data Structures Summary

### FastPriorityQueue (Lock-Free Heap)
```go
type FastPriorityQueue struct {
    data       []*PriorityNode
    comparator func(a, b *PriorityNode) bool
}

func (q *FastPriorityQueue) Push(node *PriorityNode) // O(log n)
func (q *FastPriorityQueue) Pop() *PriorityNode      // O(log n)
func (q *FastPriorityQueue) Peek() *PriorityNode     // O(1)
func (q *FastPriorityQueue) Len() int                // O(1)
```

### RingBuffer (Disruptor Pattern)
```go
type RingBuffer struct {
    buffer []Event
    mask   uint64
    write  atomic.Uint64
    read   atomic.Uint64
}

func (rb *RingBuffer) Next() uint64      // O(1)
func (rb *RingBuffer) Publish(seq uint64) // O(1)
```

### BTreeOrderBook (From Article)
```go
type PriceLevel struct {
    Price  Price
    Orders []*Order
    Side   int8
}

func (p PriceLevel) Less(than btree.Item) bool {
    if p.Side == ORDER_SIDE_BUY {
        return p.Price > than.(PriceLevel).Price
    }
    return p.Price < than.(PriceLevel).Price
}
```

---

## 9. References

### Core Patterns
- **LMAX Disruptor:** https://lmax-exchange.github.io/disruptor/user-guide/index.html
- **FastPriorityQueue:** https://github.com/lemire/FastPriorityQueue.js
- **Redis ZSET (SkipList):** https://raw.githubusercontent.com/hujiese/C-background-development-interview-experience/main/01.计算机基础知识/04.数据库/Redis/设计与实现/README.md
- **Order Book B-Tree:** https://medium.com/@adityaraj_201551/market-depth-simplified-building-an-order-book-engine-in-go-9abb9bcaec9a
- **Order Book Trees:** https://raw.githubusercontent.com/Kautenja/limit-order-book/main/notes/lob.md

### Implementations
- **Go Heap:** https://pkg.go.dev/container/heap
- **Google BTree:** https://pkg.go.dev/github.com/google/btree
- **Concurrent SkipList:** https://raw.githubusercontent.com/oceanbase/miniob/main/docs/docs/db_course_lab/lab1.md

### Visualization
- **B-Tree Visualizer:** https://www.cs.usfca.edu/~galles/visualization/BTree.html

---

## 10. Action Items

### Rewrite Principles
- **Delete first, write later** - remove unused code immediately
- **Fewer files** - consolidate logic
- **Fewer lines** - every line must justify existence
- **English comments** - for every non-trivial edit
- **Use MCP tools** - search before implementing, research when stuck

### Immediate Tasks

#### Option B: Full Rewrite (Aggressive)
Delete old code entirely, implement new architecture:
1. RingBuffer + Event Loop (LMAX Disruptor)
2. PriorityQueue for RO trimming (O(k log n))
3. B-Tree OrderBook (from Medium article)
4. Remove all locks, snapshots, atomics
5. Simplify Order struct (remove mu, active, snapshots)

### Guidelines for New Code
- Comments in English explaining WHY
- Direct field access (no snapshots)
- No locks in hot path
- Zero allocations in critical path
- Consolidate into fewer packages
- **Search MCP tools before implementing new patterns**

### Research Workflow (When Stuck or Unclear)
1. Search for examples: `codesearch query="pattern name Go"`
2. Search the web: `websearch query="best practices 2024"`
3. Fetch relevant docs: `webfetch url="relevant article"`
4. Ask documentation: `context7_query-docs`
5. Apply findings, iterate

### Metrics to Track
- PlaceOrder latency (target: <100ns)
- Total lines of code (target: reduction)
- Number of files (target: consolidation)
- Race detector: 0 warnings
