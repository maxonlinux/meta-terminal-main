# Trading Engine Architecture - Comprehensive Test Plan

## Architectural Rules Summary

### 1. Positions & ReduceOnly

- **Rule 1.1**: Positions only exist for LINEAR markets
- **Rule 1.2**: ReduceOnly only exists for LINEAR markets
- **Rule 1.3**: SPOT orders cannot use reduceOnly flag

### 2. Balance Management

- **Rule 2.1**: Balance lock/unlock happens at order price, not execution price
- **Rule 2.2**: Lock amount = price × qty for BUY, qty for SELL
- **Rule 2.3**: Balance lock/unlock on order creation (RESTING orders only)
- **Rule 2.4**: Balance lock/unlock on qty amendment (fills OR amend)
- **Rule 2.5**: Balance unlock BEFORE actual buy/sell operation
- **Rule 2.6**: Three buckets: AVAILABLE, LOCKED, MARGIN
- **Rule 2.7**: IOC/FOK/MARKET orders never rest, no balance lock

### 3. Order Lifecycle

- **Rule 3.1**: RESTING orders go to orderbook, wait for counterparty
- **Rule 3.2**: IOC orders: fill immediately or cancel (no rest)
- **Rule 3.3**: FOK orders: fill completely or cancel (no rest)
- **Rule 3.4**: MARKET orders: fill at best price, never rest
- **Rule 3.5**: POST_ONLY orders: if would cross, reject; else rest
- **Rule 3.6**: Price-time priority: FIFO at same price level

### 4. Position Management

- **Rule 4.1**: Positions tracked per symbol per user
- **Rule 4.2**: Long position: size > 0, Short position: size < 0
- **Rule 4.3**: ReduceOnly validation: order qty ≤ |position size|
- **Rule 4.3.2**: after every position change we should adjust reduceOnly orders to match the new position size (cancel/resize orders atomically and in the same transaction so no other action can trigger these orders before resizing them)
- **Rule 4.4**: On fill: newSize = currentSize ± filledQty
- **Rule 4.5**: Realized PnL calculated on position closes

### 5. Race Condition Protection

- **Rule 5.1**: UserQueue serialization for all user operations
- **Rule 5.2**: No locks needed due to serialization
- **Rule 5.3**: Order of operations matches queue order

### 6. Zero-Allocation Design

- **Rule 6.1**: Pre-allocated Int32Array/Float64Array memory pool
- **Rule 6.2**: Index-based references (no object pointers)
- **Rule 6.4**: O(log n) orderbook operations
- **Rule 6.5**: No GC pressure in hot paths

### 7. Symbol & Market Isolation

- **Rule 7.1**: SPOT and LINEAR treated as separate markets
- **Rule 7.2**: Same symbol (BTCUSDT) can exist in both categories
- **Rule 7.3**: Per-symbol and category orderbooks isolated

### 8. Validation & Rejection

- **Rule 8.1**: Pre-trade balance check before lock
- **Rule 8.2**: Pre-trade reduceOnly validation
- **Rule 8.3**: Rejection with specific error codes

---

## Test Coverage Matrix

| Rule | Test File             | Test Name                                               |
| ---- | --------------------- | ------------------------------------------------------- |
| 1.1  | positions.test.ts     | should_not_allow_positions_in_spot                      |
| 1.2  | reduceonly.test.ts    | should_reject_reduceonly_in_spot                        |
| 1.3  | reduceonly.test.ts    | should_not_allow_reduceonly_flag_for_spot_orders        |
| 2.1  | balance.test.ts       | should_lock_balance_at_order_price_not_execution_price  |
| 2.2  | balance.test.ts       | should_calculate_lock_amount_correctly_for_buy_and_sell |
| 2.3  | balance.test.ts       | should_lock_balance_only_for_resting_orders             |
| 2.4  | balance.test.ts       | should_unlock_balance_on_partial_fill                   |
| 2.4  | balance.test.ts       | should_adjust_locked_balance_on_amend                   |
| 2.5  | balance.test.ts       | should_unlock_before_fill_operation                     |
| 2.6  | balance.test.ts       | should_maintain_three_balance_buckets                   |
| 2.7  | balance.test.ts       | should_not_lock_balance_for_ioc_orders                  |
| 2.7  | balance.test.ts       | should_not_lock_balance_for_fok_orders                  |
| 2.7  | balance.test.ts       | should_not_lock_balance_for_market_orders               |
| 3.1  | orderbook.test.ts     | should_add_resting_order_to_orderbook                   |
| 3.2  | order_types.test.ts   | should_handle_ioc_fill_immediately                      |
| 3.2  | order_types.test.ts   | should_cancel_ioc_order_if_no_fill                      |
| 3.3  | order_types.test.ts   | should_handle_fok_fill_completely                       |
| 3.3  | order_types.test.ts   | should_cancel_fok_order_if_partial_fill                 |
| 3.4  | order_types.test.ts   | should_fill_market_order_at_best_price                  |
| 3.4  | order_types.test.ts   | should_not_place_market_order_in_book                   |
| 3.5  | order_types.test.ts   | should_reject_post_only_if_would_cross                  |
| 3.5  | order_types.test.ts   | should_place_post_only_if_no_cross                      |
| 3.6  | matching.test.ts      | should_fill_earlier_order_first_at_same_price           |
| 4.1  | positions.test.ts     | should_track_position_per_symbol_per_user               |
| 4.2  | positions.test.ts     | should_distinguish_long_and_short_positions             |
| 4.3  | reduceonly.test.ts    | should_validate_reduceonly_qty_against_position         |
| 4.4  | positions.test.ts     | should_update_position_size_on_fill                     |
| 4.5  | positions.test.ts     | should_calculate_realized_pnl_on_close                  |
| 5.1  | racecondition.test.ts | should_serialize_user_operations                        |
| 6.1  | memory.test.ts        | should_use_preallocated_memory_pool                     |
| 6.2  | memory.test.ts        | should_use_index_based_references                       |
| 6.4  | orderbook.test.ts     | should_maintain_log_n_complexity                        |
| 6.5  | memory.test.ts        | should_not_allocate_in_hot_paths                        |
| 7.1  | symbols.test.ts       | should_isolate_spot_and_linear_markets                  |
| 7.2  | symbols.test.ts       | should_allow_same_symbol_in_different_categories        |
| 7.3  | orderbook.test.ts     | should_have_isolated_orderbooks_per_symbol_and_category |
| 8.1  | validation.test.ts    | should_check_balance_before_lock                        |
| 8.2  | validation.test.ts    | should_validate_reduceonly_before_placing               |
| 8.3  | validation.test.ts    | should_return_specific_rejection_reasons                |

---

## Implementation Priority

### Phase 1: Core Balance Management

- Rule 2.1-2.7 (balance lock/unlock logic)
- Rule 3.1-3.5 (order lifecycle)

### Phase 2: Positions & ReduceOnly

- Rule 1.1-1.3 (category validation)
- Rule 4.1-4.5 (position management)
- Rule 3.6 (price-time priority)

### Phase 3: Race Conditions & Zero-Allocation

- Rule 5.1-5.3 (UserQueue serialization)
- Rule 6.1-6.5 (memory management)

### Phase 4: Validation & Isolation

- Rule 7.1-7.3 (market isolation)
- Rule 8.1-8.3 (validation)
