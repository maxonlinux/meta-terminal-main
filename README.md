# Meta-Terminal-Go

Zero-allocation sub-millisecond trading engine written in Go.

## Features

- **Spot Market** - Direct asset-to-asset trading
- **Linear Market** - Margin-based trading with leverage
- **Conditional Orders** - Stop, TP, SL orders
- **Position Management** - Long/Short with risk calculations
- **WAL + Snapshots** - Atomic consistency with durability
- **Zero-Allocation Design** - Object pooling for minimal GC

## Architecture

See [AGENTS.md](AGENTS.md) for detailed architecture documentation.

## Building

```bash
go build -o meta-terminal-go ./cmd/main.go
```

## Running

```bash
./meta-terminal-go
```

## Configuration

Configure via environment variables or `.env` file:

```env
WAL_PATH=wal
SNAPSHOT_PATH=snapshots
OUTBOX_PATH=outbox
```

## Performance Targets

| Operation | Target Latency |
|-----------|----------------|
| PlaceOrder | < 500μs |
| MatchOrder | < 200μs |
| TradeExec | < 300μs |
| PriceTick | < 100μs |

## Project Structure

```
meta-terminal-go/
├── cmd/main.go              # Entry point
├── types/                   # Domain types
├── internal/                # Core implementation
│   ├── balance/            # Balance operations
│   ├── position/           # Position management
│   ├── orderbook/          # Order book
│   ├── matching/           # Matching engine
│   ├── trigger/            # Conditional orders
│   ├── spot/               # Spot market
│   ├── linear/             # Linear market
│   ├── state/              # Global state
│   ├── pool/               # Object pooling
│   ├── persistence/        # WAL, Snapshot, Outbox
│   ├── price/              # Price feed
│   └── api/                # HTTP API
└── config/                 # Configuration
```
