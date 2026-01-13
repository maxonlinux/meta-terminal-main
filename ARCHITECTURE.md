# Architecture Overview

Goal: ultra-fast, in-proc, sub-millisecond latency trading engine with strict business rules from `BUSINESS_RULES.md`.

## Core Principles

- In-process engine loop, no sharding.
- Separate orderbooks for SPOT and LINEAR markets (per symbol, O(1) access).
- Shared balances (bucketed for SPOT and LINEAR flows).
- Zero/near-zero allocations on hot paths (pooling and preallocation).
- Deterministic IDs with Snowflake for all persisted/user-visible IDs.

## High-Level Components

```
cmd/
  engine/        In-proc engine runner
  gateway/       HTTP API (PlaceOrder, CancelOrder, etc.)
  marketdata/    Price feed adapter (NATS later)

internal/
  engine/        Single-thread event loop + queues
  oms/           Order validation, placement, cancellation
  orderbook/     Matching engine (SPOT/LINEAR isolation)
  triggers/      Trigger monitor (conditional/OCO)
  clearing/      Reserve/Release/ExecuteTrade
  portfolio/     Balances + positions
  persistence/   WAL + snapshot + outbox
  history/       DuckDB writer + schema
  types/         Core domain types
  constants/     Enums/constants
  snowflake/     ID generator
  pool/          Object pools
```

## Persistence Design

### State Recovery

- **WAL**: append-only event log of engine state transitions.
- **Snapshot**: periodic compacted state (orderbooks, balances, positions, triggers).
- **Replay**: load snapshot, then replay WAL tail to recover.

### History Storage (DuckDB)

- **Outbox file**: engine writes closed orders/trades/pnl events to a local append-only outbox file.
- **Background loader**: separate `history-loader` process batches outbox events into DuckDB tables.
- **Durability**: outbox is authoritative; DuckDB can be rebuilt by replaying outbox.

## Order Flow (Simplified)

1. Validate order (field + business rules).
2. Branch by type:
   - Conditional definition: any order with TriggerPrice > 0 (including STOP/TP/SL/OCO, differing by StopOrderType).
   - OCO: create 2 close-on-trigger orders, same OrderLinkId.
   - Conditional: store in trigger monitor (UNTRIGGERED).
   - Regular: reserve funds, then match.
3. Matching generates trades; clearing executes balance/position transitions.
4. Emit events into WAL and outbox (for history).

## FOK Rule

- FOK is **all-or-nothing**.
- Must pre-check full fill **before** reserve or trade.
- If insufficient liquidity: reject; do not persist in orderbook or WAL.
