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
- Zero allocations in critical path
- Fewest lines of code possible
- Fewest files possible
- Maximum performance

1. The biggest problem of the app are reduce-only orders (ro) and that's why we decided to change the architecture
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
- User `sequential-thinking` in every response
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
codesearch query="Price Tree Go implementation"
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

## Bybit's Reduce-Only Trimming Rules

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
   - When position size decreases, all RO orders are re-evaluated reactively
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

3. **No margin required:**
   - Close on Trigger orders don't require margin for execution upon trigger
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

---


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

### Correct (Bybit-Compliant - Furthest-First):

```
1. Store by price distance from current market:
   - SELL: highest price first
   - BUY: lowest price first
2. Trim in sorted order (CORRECT!)
3. Result: Orders near market get priority
```


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


### LOW Priority

5. **Margin handling for CoT**
   - Bybit: CoT doesn't require margin
   - Current: Need to verify clearing service
   - Impact: Edge case for insufficient margin

---

## Reference Links

- Bybit Reduce-Only Orders: https://www.bybit.com/en/help-center/article/Reduce-Only-Order
- Bybit Close-On-Trigger: https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order
- Bybit Order Types: https://www.bybit.com/en/help-center/article/Types-of-Orders-Available-on-Bybit
- Bybit Getting Started: https://www.bybit.com/pl-PL/help-center/article/How-to-Get-Started-With-Perpetual-and-Futures-Trading/

---ß

**Implementation:**
- Added position mode constants in `internal/constants/constants.go`
- Added `PositionMode` field to Engine struct
- Added `SetPositionMode()` method for configuration
- Added validation in `handleOrder()` to reject reduce-only orders in Hedge Mode
- Default mode is One-Way (Bybit-compliant)

### TO BE DONE

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

4. **Position=0 CoT cancel:**
   - Create CoT order
   - Position closes before trigger
   - Expect: Order cancelled on trigger attempt

5. **Position Mode validation test:**
   - Try to place RO order in Hedge Mode
   - Expect: Error returned

---

# 🚀 NEW ENGINE ARCHITECTURE: Zero-Lock High-Performance Design

## Executive Summary
**WE HAVE FULL FREEDOM TO REWRITE:**
- Delete ANY code, ANY file, ANY package
- Rewrite architecture from scratch
- Change API contracts
- Consolidate files (fewer is better)
- Remove unused features
- Simplify wherever possible

**Code Style for New Implementation:**
- Comments in English for every non-trivial edit
- Consolidate related logic into fewer files
- Remove unused code immediately
- KISS - Keep It Simple, Stupid
- DRY - Don't Repeat Yourself

---

## 3. Priority Queue for RO Trimming (No Sorting!)

**Source:** https://raw.githubusercontent.com/Kautenja/limit-order-book/main/notes/lob.md

---

## 6. Implementation Roadmap


### Phase 1: Priority Queue RO Manager (Week 2) EXAMPLE ONLY DO NOT COPY!!!

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

## 10. Action Items

### Coding Principles
- **Fewer files** - consolidate logic
- **Fewer lines** - every line must justify existence
- **English comments** - for every non-trivial edit
- **Use MCP tools** - search before implementing, research when stuck

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
