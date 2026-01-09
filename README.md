# META-TERMINAL-GO

High-performance trading engine microservices with NATS messaging.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         GATEWAY (HTTP+WS)                        │
│   POST /auth/login | POST /orders/place | WS /market/:symbol    │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                              NATS                               │
│   orders.BTCUSDT.PLACED  │  prices.BTCUSDT  │  clearing.*       │
└────────────────────────────┬────────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────────┐
│      OMS      │   │   MARKETDATA  │   │      RISK         │
│  (Matching)   │   │  (Price Feed) │   │  (Validation)     │
└───────┬───────┘   └───────┬───────┘   └─────────┬─────────┘
        │                   │                       │
        ▼                   │                       ▼
┌───────────────┐           │             ┌───────────────────┐
│  ORDERBOOK    │           │             │    PORTFOLIO      │
│  (38ns/match) │           │             │  (Balances/Pos)   │
└───────────────┘           │             └───────────────────┘
        │                   │
        ▼                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                          CLEARING                               │
│                    (Trade Settlement)                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       PERSISTENCE (WAL)                         │
│                  tidwall/wal - async, crash-safe                │
└─────────────────────────────────────────────────────────────────┘
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| gateway | 8080 | HTTP API + WebSocket |
| oms | - | Order Matching Engine |
| portfolio | - | Balances & Positions |
| marketdata | - | Price Feed Handler |
| risk | - | Pre-trade Validation |
| clearing | - | Trade Settlement |

## Quick Start

### 1. Start NATS

```bash
docker run -d --name nats -p 4222:4222 nats:latest
```

### 2. Run All Services

```bash
# Terminal 1: Run all services
go run ./cmd/all -mode all

# Terminal 2: Run benchmarks
go test -bench="Benchmark(OrderBook|Pool)" -benchmem ./...
```

### 3. Test Gateway API

```bash
# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"pass123"}'

# Place Order
curl -X POST http://localhost:8080/orders/place \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol":"BTCUSDT",
    "category":1,
    "side":0,
    "type":0,
    "tif":0,
    "qty":100,
    "price":50000
  }'

# WebSocket for real-time updates
wss://localhost:8080/ws?token=<token>
```

## Configuration

```bash
export NATS_URL=nats://localhost:4222
export JWT_SECRET=your-secret-key
export STREAM_PREFIX=meta
```

## Performance Targets vs Actual

| Operation | Target | Actual | Status |
|-----------|--------|--------|--------|
| MatchOrder | < 200μs | **38.5ns** | ✓ EXCELLENT |
| PlaceOrder | < 500μs | **264ns** | ✓ EXCELLENT |
| BestBidAsk | < 10μs | **7.7ns** | ✓ EXCELLENT |
| CancelOrder | < 100μs | **6.3ns** | ✓ EXCELLENT |
| ConcurrentMatch | < 200μs | **116ns** | ✓ EXCELLENT |
| Pool GetOrder | < 10μs | **7.3ns** | ✓ EXCELLENT |
| WAL Save | < 100μs | **668ns** | ✓ EXCELLENT |
| WAL Load | < 50μs | **128ns** | ⚠ OPTIMIZE |
| WAL SaveTx | < 100μs | **437ns** | ✓ EXCELLENT |

## Benchmarks

Run comprehensive benchmarks:

```bash
# All benchmarks
go test -bench="Benchmark(OrderBook|Pool|WAL)" -benchmem ./...

# OrderBook only
go test -bench="BenchmarkOrderBook" -benchmem ./internal/orderbook/

# Pool operations
go test -bench="BenchmarkPool" -benchmem ./services/oms/

# WAL persistence
go test -bench="BenchmarkWAL" -benchmem ./internal/persistence/

# Run with 1 second per benchmark
go test -bench="Benchmark(OrderBook|Pool|WAL)" -benchmem -benchtime=1s ./internal/orderbook/ ./services/oms/ ./internal/persistence/
```

## NATS Subjects

```
orders.{symbol}.PLACED    # New order placed
orders.{symbol}.FILLED    # Order filled
orders.{symbol}.CANCELLED # Order cancelled
prices.{symbol}           # Price tick
clearing.{symbol}.TRADE   # Trade execution
```

## Project Structure

```
meta-terminal-go/
├── cmd/
│   ├── all/main.go         # Run all services
│   ├── gateway/main.go     # HTTP+WS API
│   └── bench/main.go       # Benchmarks
├── internal/
│   ├── orderbook/          # Matching engine
│   ├── pool/               # Object pooling
│   ├── persistence/        # WAL storage (tidwall/wal)
│   ├── messaging/          # NATS client
│   ├── types/              # Shared types
│   ├── constants/          # Constants
│   └── id/                 # ID generation
├── services/
│   ├── gateway/            # HTTP+WebSocket
│   ├── oms/                # Order Management + Matching
│   ├── marketdata/         # Price feeds
│   ├── portfolio/          # Balances & Positions
│   ├── risk/               # Pre-trade validation
│   └── clearing/           # Trade settlement
└── README.md
```

## Key Design Decisions

1. **Symbol Sharding**: Each OMS instance handles one symbol (ready for horizontal scaling)
2. **Async WAL**: tidwall/wal with background queue - non-blocking writes
3. **Object Pooling**: Zero-allocation hot paths
4. **NATS Messaging**: Pub/sub for inter-service communication
5. **JWT Auth**: Simple token-based authentication

## License

MIT
