# AGENTS.md

### Test Commands
```bash
go test ./...                     # Run all tests
go test -v ./internal/orderbook/... # Verbose tests for orderbook package
go test -run TestOrderBook_Match -v  # Run single test with verbose output
go test -run TestOrderBook_AddRemove ./internal/orderbook # Run specific test in package
go test -bench=. -benchmem ./...  # Run all benchmarks with memory stats
go test -bench=BenchmarkOrderBook_Match -benchmem ./internal/orderbook # Specific benchmark
go test -race ./...               # Run with race detection
go test -cover ./...              # Run with coverage
```

### Lint/Static Analysis
```bash
go vet ./...                      # Static analysis
go mod tidy                        # Clean up dependencies
go fmt ./...                       # Format all Go code
go mod verify                      # Verify dependencies
```

## Code Style Guidelines

### Types and Domain Modeling
- Use domain-specific type aliases: `OrderID`, `TradeID`, `UserID`, `Price`, `Quantity`
- All IDs must use Snowflake generation for persisted/user-visible data
- Use math/big for financial values (Price, Quantity) to avoid floating-point errors
- Timestamps use `uint64` (nanoseconds since epoch) via `types.NowNano()`
- Constants live in `internal/constants/` with descriptive prefixes (CATEGORY_*, ORDER_*, SIDE_*)

### Import Organization
```go
// Standard library imports first
import (
    "context"
    "sync"
    "time"
)

// Internal imports after
import (
    "github.com/anomalyco/meta-terminal-go/internal/constants"
    "github.com/anomalyco/meta-terminal-go/internal/types"
    "github.com/anomalyco/meta-terminal-go/internal/pool"
)
```

### Error Handling
- Use sentinel errors with descriptive names:
```go
var (
    ErrReduceOnlySpot               = errors.New("reduceOnly not allowed for SPOT")
    ErrConditionalSpot              = errors.New("conditional orders not allowed for SPOT")
    ErrFOKInsufficientLiquidity     = errors.New("FOK: insufficient liquidity in orderbook")
)
```
- Validate inputs early and return errors immediately
- Wrap errors with context when necessary

### Performance Guidelines
- Use `sync.Pool` for hot-path object allocation
- Pre-allocate slices with known capacity
- Zero-allocation patterns in matching engine hot paths

### Naming Conventions
- Interface names: simple, descriptive (Portfolio, Clearing, etc)
- Struct methods: receivers should be value receivers for immutable operations, pointer receivers for mutable
- Private functions: camelCase, descriptive of business logic
- Constants: SCREAMING_SNAKE_CASE with prefixes

### Critical Business Rules
- **Market Isolation**: SPOT (Category=0) and LINEAR (Category=1) are completely separate
- **FOK Orders**: Must pre-check full liquidity BEFORE reserve or trade execution
- **TP/SL Orders**: Need to think about
- **ReduceOnly**: Only for LINEAR markets, must reduce existing position size
- **Trigger Validation**: BUY trigger < current price, SELL trigger > current price
- **Reserve Formulas**:
  - SPOT BUY: Qty × Price (quote currency)
  - SPOT SELL: Qty (base currency)
  - LINEAR: (Qty × Price) / Leverage

### Testing Patterns
- Unit tests alongside source files: `*_test.go`
- Benchmark tests for performance-critical paths
- Use `pool.Get*()` and `pool.Put*()` in tests for consistent object lifecycle
- Table-driven tests for validation scenarios

### Persistence and Messaging
- Use GOB encoding for NATS messaging
- WAL + Snapshot pattern for state recovery
- Outbox pattern for async history writing to DuckDB
- Events are immutable facts (OrderEvent, TradeEvent, RPNLEvent)

### Package Structure
- `cmd/`: Entry points and binaries
- `internal/`: Core business logic (no public exports)
- `tests/`: Integration, E2E, fuz and chaos tests
- Each internal package has single responsibility

### Code Quality
- DO NOT add any test/debug/bench/reset methods directly to the implementation code. You may only add those in _test or _bench files
- Add comprehensive comments on ALL new changes (not just complex logic)
- Prefer explicit error handling over silent failures
- Use constants over magic numbers/strings
- Always return errors from functions that can fail
