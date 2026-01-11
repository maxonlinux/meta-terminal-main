Last updated: 2025-01-11

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

Next steps:
- Add tests for messaging boundaries and engine integration paths.
