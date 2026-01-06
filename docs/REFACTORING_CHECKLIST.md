# Trading Engine Refactoring Checklist

## Phase 2: Balance Management Refactoring

- [ ] Extract balance validation logic from `engine.go` (776 lines)
- [ ] Remove duplicate PlaceOrder/CancelOrder implementations
- [ ] Create unit tests for balance operations
- [ ] Integrate with engine via dependency injection

## Phase 3: OrderBook Refactoring

- [ ] Unify heap implementations in `state/heap.go` and `orderbook.go`
- [ ] Separate matching logic from storage logic
- [ ] Create comprehensive unit tests

## Phase 5: Event-Driven Architecture

- [ ] Implement `EventBus` interface
- [ ] Convert all operations to event sourcing pattern
- [ ] Update WAL to work with events instead of raw operations
- [ ] Add event replay functionality
- [ ] Create event store implementation

## Phase 6: Engine Decomposition

- [ ] Remove all duplicate implementations
- [ ] Integrate dependency injection container

## Phase 7: Performance Optimization

- [ ] Add nanosecond-level latency tracking
- [ ] Optimize memory allocations in matching engine
- [ ] Create performance benchmarks suite

## Phase 8: Testing Infrastructure

- [ ] Add performance regression tests
- [ ] Implement chaos testing for race conditions

## Phase 9: Code Quality & Documentation

- [ ] Add comprehensive error handling with proper types
- [ ] Add godoc comments to all exported functions
- [ ] Create architecture decision records (ADRs)
- [ ] Add performance profiling guides

## Phase 10: Final Integration & Validation

- [ ] Run full test suite with 95%+ coverage
- [ ] Validate performance benchmarks meet targets
- [ ] Run chaos testing for race conditions
- [ ] Validate zero-allocation design in hot paths

## Performance Targets

- [ ] Memory usage: < 300MB for 1000 markets × 100 users
- [ ] Allocations: <= 2 per operation in hot paths
- [ ] Zero memory leaks under sustained load

## Critical Files to Refactor

- [ ] `internal/engine/engine.go` (776 lines) - reduce size of code because we already have this logic elsewhere
- [ ] `internal/orderbook/orderbook.go` (394 lines) - impl
- [ ] `internal/snapshot/snapshot.go` (401 lines) - separate concerns
- [ ] `internal/api/server.go` (349 lines) - extract business logic
- [ ] `internal/state/state.go` (261 lines) - optimize storage layer
