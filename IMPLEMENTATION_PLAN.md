# DETAILED IMPLEMENTATION PLAN

## 1. RO/CoT SIDE VALIDATION

### Rule (Bybit Spec)
- LONG position (+100) → only SELL RO allowed
- SHORT position (-100) → only BUY RO allowed
- CloseOnTrigger: doesn't require position (can trigger to open)
- NO position (0) → RO rejected, but CoT allowed

### Implementation in handleOrder()

```go
if req.ReduceOnly {
    if pos.Size == 0 {
        return ErrNoPositionForRO
    }
    if pos.Size > 0 && req.Side != ORDER_SIDE_SELL {
        return ErrROSideMismatch
    }
    if pos.Size < 0 && req.Side != ORDER_SIDE_BUY {
        return ErrROSideMismatch
    }
}
```

### Edge Cases
| Scenario | Position | Order Side | Result |
|----------|----------|------------|--------|
| Normal LONG + SELL | +100 | SELL | ✅ OK |
| Normal SHORT + BUY | -100 | BUY | ✅ OK |
| LONG + BUY RO | +100 | BUY | ❌ REJECT |
| SHORT + SELL RO | -100 | SELL | ❌ REJECT |
| No position | 0 | SELL | ❌ REJECT |

---

## 2. TRIGGERPRICE DIRECTION VALIDATION

### Rule (Bybit Spec)
- BUY conditional: triggerPrice < currentPrice (wait for price DROP)
- SELL conditional: triggerPrice > currentPrice (wait for price RISE)

### Implementation

```go
if req.TriggerPrice > 0 {
    currentPrice := getLastPrice(req.Symbol, req.Category)
    if currentPrice == 0 {
        return ErrNoPriceForConditional // Can't validate without price
    }

    if req.Side == ORDER_SIDE_BUY && req.TriggerPrice >= currentPrice {
        return ErrInvalidTriggerForBuy // BUY trigger must be below current
    }
    if req.Side == ORDER_SIDE_SELL && req.TriggerPrice <= currentPrice {
        return ErrInvalidTriggerForSell // SELL trigger must be above current
    }
}
```

### Edge Cases
| Scenario | Current | Trigger | Side | Result |
|----------|---------|---------|------|--------|
| Normal BUY trigger | 50000 | 49000 | BUY | ✅ OK |
| Normal SELL trigger | 50000 | 51000 | SELL | ✅ OK |
| BUY trigger too high | 50000 | 50000 | BUY | ❌ REJECT |
| SELL trigger too low | 50000 | 50000 | SELL | ❌ REJECT |
| No price data | 0 | 49000 | BUY | ❌ REJECT |

---

## 3. POST-ONLY VALIDATION

### Rule (Bybit Spec)
- Post-Only orders: REJECT if they would match immediately
- Purpose: Always become maker, never taker

### Implementation

```go
if req.TIF == TIF_POST_ONLY {
    if req.Side == ORDER_SIDE_BUY {
        if bestAsk := ob.BestAsk(); bestAsk != nil && Price(req.Price) >= bestAsk.Price {
            return ErrPostOnlyWouldMatch
        }
    } else {
        if bestBid := ob.BestBid(); bestBid != nil && Price(req.Price) <= bestBid.Price {
            return ErrPostOnlyWouldMatch
        }
    }
}
```

### Edge Cases
| Scenario | Order | Best Bid | Best Ask | Result |
|----------|-------|----------|----------|--------|
| Normal BUY post-only | 50000 | 49900 | 50100 | ✅ OK |
| BUY would match | 50200 | 49900 | 50100 | ❌ REJECT |
| Normal SELL post-only | 50000 | 49900 | 50100 | ✅ OK |
| SELL would match | 49800 | 49900 | 50100 | ❌ REJECT |
| Empty book | 50000 | nil | nil | ✅ OK |

---

## 4. IOC/FOK EXECUTION

### Rule (Bybit Spec)
- **IOC (Immediate-Or-Cancel)**: Fill what you can immediately, cancel rest
- **FOK (Fill-Or-Kill)**: Either fill ENTIRE quantity immediately, or cancel all

### Implementation

```go
func (e *Engine) executeMarketOrder(order *Order) []*Trade {
    var trades []*Trade
    remaining := order.Quantity

    for remaining > 0 {
        // Get best opposite price level
        var bestLevel *PriceLevel
        if order.Side == ORDER_SIDE_BUY {
            bestLevel = ob.BestAsk()
        } else {
            bestLevel = ob.BestBid()
        }

        if bestLevel == nil {
            break // No liquidity
        }

        // Check price for LIMIT orders
        if order.Type == ORDER_TYPE_LIMIT {
            if order.Side == ORDER_SIDE_BUY && order.Price < bestLevel.Price {
                break // Price too low
            }
            if order.Side == ORDER_SIDE_SELL && order.Price > bestLevel.Price {
                break // Price too high
            }
        }

        // Execute trade
        fillQty := min(remaining, bestLevel.TotalQuantity())
        trade := e.createTrade(order, bestLevel, fillQty)
        trades = append(trades, trade)

        remaining -= fillQty

        // For IOC: continue until filled or no more liquidity
        // For FOK: if remaining > 0 after loop, cancel all trades
    }

    if order.TIF == TIF_FOK && remaining > 0 {
        // FOK failed - revert all trades
        e.revertTrades(trades)
        return nil // No trades, order cancelled
    }

    return trades
}
```

### Edge Cases
| Scenario | TIF | Filled | Remaining | Result |
|----------|-----|--------|-----------|--------|
| Full fill | IOC | 100 | 0 | ✅ Return trade, remaining=0 |
| Partial fill | IOC | 50 | 50 | ✅ Return trade, cancel rest |
| No liquidity | IOC | 0 | 100 | ✅ Return nil, cancel order |
| Full fill | FOK | 100 | 0 | ✅ Return trade |
| Partial fill | FOK | 50 | 50 | ❌ REVERT trades, return nil |

---

## 5. MATCHING LOGIC

### Rule
- Match BUY orders against lowest SELL orders
- Match SELL orders against highest BUY orders
- Price: Maker's price (passive order)
- Quantity: Min of both orders

### Implementation

```go
func (ob *OrderBook) MatchOrder(order *Order) []*Trade {
    var trades []*Trade

    for order.Quantity > 0 {
        // Get best opposite
        var opposite *Order
        if order.Side == ORDER_SIDE_BUY {
            opposite = ob.BestAskOrder() // Lowest SELL
        } else {
            opposite = ob.BestBidOrder() // Highest BUY
        }

        if opposite == nil {
            break // No match
        }

        // Check price crossing
        if order.Side == ORDER_SIDE_BUY && order.Price < opposite.Price {
            break // Price too low
        }
        if order.Side == ORDER_SIDE_SELL && order.Price > opposite.Price {
            break // Price too high
        }

        // Self-match prevention
        if order.UserID == opposite.UserID {
            return nil // Or handle differently
        }

        // Execute trade
        tradeQty := min(order.Quantity, opposite.Quantity)
        trade := &Trade{
            ID:         TradeID(Next()),
            Symbol:     order.Symbol,
            Price:      opposite.Price, // Maker's price
            Quantity:   tradeQty,
            TakerOrder: order,
            MakerOrder: opposite,
            Timestamp:  NowNano(),
        }

        // Update orders
        order.Quantity -= tradeQty
        opposite.Quantity -= tradeQty

        // Remove filled orders
        if opposite.Quantity == 0 {
            ob.RemoveOrder(opposite.ID)
        }

        trades = append(trades, trade)
    }

    // Handle TIF for remaining quantity
    if order.Quantity > 0 {
        switch order.TIF {
        case TIF_IOC:
            // Cancel remaining - but trades already made
            ob.RemoveOrder(order.ID)
        case TIF_FOK:
            // Cancel entire order + revert trades
            ob.RemoveOrder(order.ID)
            e.revertTrades(trades)
            return nil
        }
    }

    return trades
}
```

### Edge Cases
| Scenario | Order Qty | Opposite Qty | Result |
|----------|-----------|--------------|--------|
| Full match | 100 | 100 | Both filled |
| Partial BUY | 100 | 50 | BUY 50 remaining, SELL filled |
| Partial SELL | 50 | 100 | SELL filled, BUY 50 remaining |
| Price mismatch | 100 | 100 | No trade, both remain |
| Same user | 100 | 100 | Self-match - skip |

---

## 6. SELF-MATCH PREVENTION

### Rule
- Order should NOT match with own orders in the book
- Simply skip own orders when matching

### Implementation (in matching loop)

```go
func (ob *OrderBook) matchAgainstAsks(order *Order) []*Trade {
    for ob.asks.Len() > 0 && order.Quantity > 0 {
        ask := (*ob.asks)[0]
        if ask.Price > order.Price {
            break
        }
        if ask.Order.UserID == order.UserID {
            heap.Pop(ob.asks)
            delete(ob.orders, ask.Order.ID)
            continue
        }
        // ... execute trade
    }
}
```

### Edge Cases
| Scenario | Order User | Best Opposite | Result |
|----------|------------|---------------|--------|
| No own orders | User 1 | User 2 SELL @50000 | ✅ Match |
| Own order best | User 1 | User 1 SELL @50000 | ❌ Skip, find next |
| All own orders | User 1 | User 1 only | ❌ No match |
| Mixed | User 1 | User 2 @50000, User 1 @49900 | ✅ Match User 2 |

---

## 7. BALANCE INTEGRATION

### Rule
- **Reserve** on PlaceOrder: Available → Locked
- **Deduct** on Trade: Locked decreases
- **AddCredit** on Trade: Available increases (for maker)

### Implementation

```go
func (e *Engine) handleOrder(req *OrderRequest) {
    // ... validation ...

    // Calculate reserve amount
    reserveQty := Quantity(req.Quantity)
    var reserveValue int64
    if req.Category == CATEGORY_SPOT {
        if req.Side == ORDER_SIDE_BUY {
            reserveValue = int64(req.Quantity) * req.Price
        } else {
            reserveValue = int64(req.Quantity)
        }
    } else {
        leverage := e.getUserLeverage(UserID(req.UserID), req.Symbol)
        reserveValue = int64(req.Quantity) * req.Price / int64(leverage)
    }

    // Reserve balance
    err := e.balances.Reserve(UserID(req.UserID), req.Symbol, Price(reserveValue))
    if err != nil {
        return err
    }

    // ... place order ...
}

func (e *Engine) handleTrade(trade *Trade) {
    // Taker: Deduct locked
    e.balances.Deduct(trade.TakerOrder.UserID, trade.Symbol, Price(trade.Price*trade.Quantity))

    // Maker: Release locked + AddCredit available
    e.balances.Release(trade.MakerOrder.UserID, trade.Symbol, Price(trade.Price*trade.Quantity))
    e.balances.AddCredit(trade.MakerOrder.UserID, trade.Symbol, Price(trade.Price*trade.Quantity))
}
```

### Edge Cases
| Scenario | Balance Action |
|----------|----------------|
| Place BUY SPOT | Reserve: Available -= 50000, Locked += 50000 |
| Place SELL SPOT | Reserve: Available -= 1 BTC, Locked += 1 BTC |
| Trade fill | Deduct taker locked, Credit maker available |
| Order cancel | Release locked back to available |
| Insufficient balance | Reject order at Reserve |

---

## 8. ERRORS TO ADD

```go
// In constants.go
var (
    ErrNoPositionForRO        = errors.New("reduce-only order requires existing position")
    ErrROSideMismatch         = errors.New("reduce-only side doesn't match position direction")
    ErrInvalidTriggerForBuy   = errors.New("buy trigger must be below current price")
    ErrInvalidTriggerForSell  = errors.New("sell trigger must be above current price")
    ErrPostOnlyWouldMatch     = errors.New("post-only order would match immediately")
    ErrNoPriceForConditional  = errors.New("no price data for conditional order validation")
    ErrSelfMatch             = errors.New("order would match with own order")
    ErrInsufficientBalance   = errors.New("insufficient balance for reservation")
)
```

---

## EXECUTION ORDER (UPDATED)

| # | Task | Status | Notes |
|---|------|--------|-------|
| 1 | Add error constants | ✅ Done | `core/constants.go` |
| 2 | RO/CoT side validation | ✅ Done | Only `ReduceOnly` requires position |
| 3 | TriggerPrice direction validation | ✅ Done | In `handleOrder` |
| 4 | Post-Only validation | ✅ Done | Simplified, zero allocations |
| 5 | Matching logic | ✅ Done | `orderbook.go` - heap-based matching |
| 6 | Trade type refactor | ✅ Done | `*Trade` instead of `*TradeEvent` |
| 7 | Self-match prevention | ✅ Done | Skip own orders in matching loop |
| 8 | Balance integration | Pending | Reserve/Release/Deduct calls |
| 9 | IOC/FOK execution | Pending | Handle partial fills |
| 10 | Tests | Pending | Engine tests needed |

---

## FILES TO MODIFY

| File | Changes |
|------|---------|
| `core/constants.go` | Add error constants |
| `core/engine.go` | Add validations, matching, balance calls |
| `core/orderbook.go` | Add BestOppositeOrder, MatchOrder |
| `core/types.go` | May need new error type |
| `core/balance.go` | May need new methods |

