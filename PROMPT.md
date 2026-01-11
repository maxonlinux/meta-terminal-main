Last updated: 2026-01-12

Project goals:
- Ultra fast in-proc trading engine (sub-millisecond), zero/near-zero allocations on hot paths.
- Strictly follow business rules in `AGENTS.md` (SPOT vs LINEAR isolation, order types, FOK behavior).

Decisions:
- In-proc engine only. No sharding.
- Persistence:
  - WAL + snapshot for state recovery and replay.
  - Outbox file for async history export.
  - DuckDB for historical data (closed orders, trades, PnL), fed by outbox batches.
- OCO/STOP orders are conditional; conditional = TriggerPrice > 0 (OCO differs only by StopOrderType).
- History loader runs as a separate process to isolate CPU/GC from the engine.
- NATS used later for mark-price feed (not part of core engine loop).
- Trigger side for CloseOnTrigger Stop/StopLoss is inverted (StopLoss for long triggers on price <= trigger).
- Snowflake IDs for all persisted/public IDs.
- Orderbook matches now produce Trade values (no per-trade heap alloc); OrderResult stores trade values with fixed buffer.
- Registry introduced for instruments + prices; instruments derived from price bands.
- Instrument bounds now include min/max price (band-based) and non-zero defaults; MaxQty defaults to MaxInt64 until per-symbol filters arrive.
- Separate binaries for `cmd/all`, `cmd/gateway`, `cmd/marketdata`, `cmd/risk`.
- Quote assets configurable via `QUOTE_ASSETS`.
- Centralized env config in `internal/config` (used by all binaries including history loader).
- OMS service split into focused files (order flow, triggers, events, orderbook access, positions).
- OMS unit tests now aligned by filename in `internal/oms` (validation/orderflow/positions/helpers); scenario/integration flows live in `tests/oms`.
- Gateway now exposes `Handler()` for in-process tests (no socket bind).
- Terminal order events (FILLED/CANCELED/PARTIALLY_FILLED_CANCELED/DEACTIVATED) are published to outbox and removed from in-memory orders.
- Added integration tests under `tests/` for gateway HTTP, messaging gob boundaries, persistence replay (snapshot+WAL), OMS invariants, and outbox terminal order flow.
- OMS NATS client moved under `internal/messaging` for clarity.
- DuckDB driver uses `github.com/marcboeker/go-duckdb/v2` (official v2 module path).
- Trigger monitor uses heap-based trigger queues; test-only reset lives in `_test.go`.
- OMS keeps `ordersByID` index to remove O(n) scans for linked orders; shared remove helper used across matching/trigger/positions/cancel.
- Risk uses symbol-indexed positions map and pooled liquidation buffers + OrderInput pooling; direct `OnPriceTick`/`OnPositionUpdate` added for in-proc path.

Next steps:
- Add tests for messaging boundaries and engine integration paths.
- Decide how instruments/price filters are distributed between services (shared store vs duplicated fetch).
