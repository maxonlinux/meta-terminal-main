# Trading Platform Low Level Design: Architecture & Flow

## Overview

Современная торговая платформа состоит из нескольких ключевых компонентов, которые взаимодействуют для обеспечения справедливого, консистентного и атомарного исполнения ордеров.

---

## 1. System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CLIENT LAYER                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Trading   │  │    Mobile   │  │    HFT      │  │   Broker    │        │
│  │      App    │  │      App    │  │    Bots     │  │   Systems   │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
└─────────┼────────────────┼────────────────┼────────────────┼────────────────┘
          │ FIX/Binary     │ FIX/Binary     │ FIX/Binary     │ FIX/Binary
          ▼                ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          GATEWAY LAYER                                       │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                        Client Gateway                                   │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │  │
│  │  │ Authentication│  │   Rate      │  │  Validation │  │  Message    │   │  │
│  │  │    & Auth    │  │   Limiting  │  │   & Normal  │  │  Parsing    │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                      │                                        │
│                                      ▼                                        │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                     Order Manager / Sequencer                          │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │  │
│  │  │   Sequence  │  │    Order    │  │    Cancel   │  │   Request   │   │  │
│  │  │   Number    │  │   Queueing  │  │   Handling  │  │  Batching   │   │  │
│  │  │  Generator  │  │             │  │             │  │             │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │  │
│  │         │                                                          │   │  │
│  │         │ Monotonically Increasing Sequence Number                  │   │  │
│  │         ▼                                                          ▼   │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                      │                                        │
└──────────────────────────────────────┼────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      RISK MANAGEMENT LAYER                                   │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                      Risk Management System (RMS)                      │  │
│  │                                                                        │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │  │
│  │  │   Balance   │  │    Margin   │  │ Position    │  │   Credit    │   │  │
│  │  │    Check    │  │  Calculation│  │    Limit    │  │    Limit    │   │  │
│  │  │             │  │             │  │   Check     │  │   Check     │   │  │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘   │  │
│  │         │                │                │                │          │  │
│  │         ▼                ▼                ▼                ▼          │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │              Pre-Trade Risk Validation Engine                    │  │  │
│  │  │                                                                │  │  │
│  │  │   Order + Account State ───► Risk Rules ───► Pass/Fail         │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                      │                                        │
│                                      ▼ (If Pass)                             │
└──────────────────────────────────────┼────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      MATCHING ENGINE LAYER                                   │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                    Matching Engine (Per-Symbol)                        │  │
│  │                                                                        │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │                    Order Book                                    │  │  │
│  │  │  ┌─────────────────────┐  ┌─────────────────────┐               │  │  │
│  │  │  │       BIDS          │  │        ASKS         │               │  │  │
│  │  │  │   (Buy Orders)      │  │    (Sell Orders)    │               │  │  │
│  │  │  │                     │  │                     │               │  │  │
│  │  │  │   Price DESC        │  │    Price ASC        │               │  │  │
│  │  │  │   Time ASC          │  │    Time ASC         │               │  │  │
│  │  │  └─────────────────────┘  └─────────────────────┘               │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  │                                                                        │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │              Matching Algorithm (CLOB)                           │  │  │
│  │  │                                                                │  │  │
│  │  │   Price-Time Priority:                                          │  │  │
│  │  │   1. Best Price First (Highest Bid / Lowest Ask)                │  │  │
│  │  │   2. Time Priority (FIFO for same price)                        │  │  │
│  │  │   3. Matching Criteria:                                         │  │  │
│  │  │      • Buy Price >= Sell Price                                  │  │  │
│  │  │      • Quantity Match                                           │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                      │                                        │
│                          Trade Events Generated                              │
│                                      ▼                                        │
└──────────────────────────────────────┼────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                     EVENT PROCESSING LAYER                                   │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                    Event Sequencer                                     │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │  │
│  │  │  Trade      │  │   Order     │  │   Balance   │  │   Market    │   │  │
│  │  │  Events     │  │   Events    │  │   Updates   │  │     Data    │   │  │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘   │  │
│  │         │                │                │                │          │  │
│  │         ▼                ▼                ▼                ▼          │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │              Kafka / Message Queue (Ordered Events)              │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                      │                                        │
│                    Multiple Consumers (Parallel Processing)                  │
│         ┌────────────────────────────┼────────────────────────────┐          │
│         ▼                            ▼                            ▼          │
│  ┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐     │
│  │   Balance &      │     │   Settlement &   │     │   Market Data    │     │
│  │   Position       │     │   Clearing       │     │   Publisher      │     │
│  │   Service        │     │   Service        │     │                  │     │
│  └──────────────────┘     └──────────────────┘     └──────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Order to Settlement Flow

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         COMPLETE ORDER LIFECYCLE                              │
└──────────────────────────────────────────────────────────────────────────────┘

  ┌─────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
  │  ORDER  │───►│   RISK   │───►│ SEQUENCER│───►│ MATCHING │───►│  TRADE   │
  │ CREATION│    │   CHECK  │    │          │    │  ENGINE  │    │  EVENT   │
  └─────────┘    └──────────┘    └──────────┘    └──────────┘    └──────────┘
       │              │               │               │               │
       │              │               │               │               │
       ▼              ▼               ▼               ▼               ▼
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                          │
  │  Step 1: ORDER CREATION (Client)                                         │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │  FIX Message: NewOrderSingle (35=D)                                │  │
  │  │  Fields:                                                           │  │
  │  │  • ClOrdID (11)      - Unique Order ID                            │  │
  │  │  • Symbol (55)       - Trading Pair                               │  │
  │  │  • Side (54)         - Buy/Sell                                   │  │
  │  │  • OrderQty (38)     - Quantity                                   │  │
  │  │  • OrdType (40)      - Limit/Market/IOC/FOK                       │  │
  │  │  • Price (44)        - Limit Price (if applicable)                │  │
  │  │  • TimeInForce (59)  - GTC/IOC/FOK                                │  │
  │  │  • Account (1)       - User Account ID                            │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  │  Step 2: PRE-TRADE RISK CHECK                                            │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │  For SPOT Trading:                                                 │  │
  │  │  • Check Available Balance                                         │  │
  │  │  • Required = OrderQty × Price (for BUY)                          │  │
  │  │  • Required = OrderQty (for SELL - asset availability)            │  │
  │  │  IF Available < Required: REJECT ORDER                            │  │
  │  │                                                                      │  │
  │  │  For LINEAR (Futures/Perpetuals):                                  │  │
  │  │  • Calculate Initial Margin                                        │  │
  │  │  • Margin = (OrderQty × Price) / Leverage                          │  │
  │  │  • Check if Equity > Initial Margin                                │  │
  │  │  IF Equity < Required Margin: REJECT ORDER                         │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  │  Step 3: EVENT SEQUENCING (Critical for Fairness)                        │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │  Assign Monotonically Increasing Sequence Number:                  │  │
  │  │                                                                      │  │
  │  │  Event = {                                                         │  │
  │  │    SequenceNumber: 1000001,                                        │  │
  │  │    OrderID: "ORDER_12345",                                         │  │
  │  │    Timestamp: 1705324800000000,                                    │  │
  │  │    ...                                                              │  │
  │  │  }                                                                 │  │
  │  │                                                                      │  │
  │  │  This ensures:                                                     │  │
  │  │  • Total Order of All Events                                       │  │
  │  │  • Deterministic Matching (Replay = Same Result)                   │  │
  │  │  • Price-Time Priority Enforcement                                 │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  │  Step 4: ORDER MATCHING (Matching Engine)                                │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │  For BUY Order:                                                    │  │
  │  │  1. Find Lowest Ask ≤ Buy Price                                    │  │
  │  │  2. Match with Oldest Order at that Price (FIFO)                   │  │
  │  │  3. Execute Trade at Ask Price (Price Improvement if Bid > Ask)   │  │
  │  │  4. Update Order Quantities                                        │  │
  │  │  5. If Quantity > 0, Add Rest to Order Book                        │  │
  │  │                                                                      │  │
  │  │  For SELL Order:                                                   │  │
  │  │  1. Find Highest Bid ≥ Sell Price                                  │  │
  │  │  2. Match with Oldest Order at that Price (FIFO)                   │  │
  │  │  3. Execute Trade at Bid Price                                     │  │
  │  │  4. Update Order Quantities                                        │  │
  │  │  5. If Quantity > 0, Add Rest to Order Book                        │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  │  Step 5: TRADE EVENT GENERATION                                          │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │  TradeEvent = {                                                    │  │
  │  │    TradeID: "TRADE_98765",                                         │  │
  │  │    OrderID: "ORDER_12345",                                         │  │
  │  │    CounterOrderID: "ORDER_67890",                                  │  │
  │  │    Symbol: "BTC/USDT",                                             │  │
  │  │    Side: "BUY",                                                    │  │
  │  │    ExecutedPrice: 43500.00,                                        │  │
  │  │    ExecutedQuantity: 0.5,                                          │  │
  │  │    ExecutedValue: 21750.00,                                        │  │
  │  │    Fee: 2.175,  // Maker/Taker Fee                                 │  │
  │  │    Timestamp: 1705324800001000,                                    │  │
  │  │    ...                                                              │  │
  │  │  }                                                                 │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
                                       │
                                       │ Trade Events Published to Kafka
                                       ▼
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                          │
  │  Step 6: BALANCE & POSITION UPDATE (Atomic Transaction)                  │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │                                                                      │  │
  │  │  DOUBLE-ENTRY BOOKKEEPING PRINCIPLE:                                │  │
  │  │  ┌───────────────────────────────────────────────────────────────┐  │  │
  │  │  │  Assets = Liabilities + Equity                                 │  │  │
  │  │  │  Debits = Credits                                              │  │  │
  │  │  └───────────────────────────────────────────────────────────────┘  │  │
  │  │                                                                      │  │
  │  │  For SPOT BUY Trade (User buys BTC with USDT):                     │  │
  │  │  ┌───────────────────────────────────────────────────────────────┐  │  │
  │  │  │  DEBIT:   Assets:BTC:User      +0.5 BTC                       │  │  │
  │  │  │  CREDIT:  Assets:USDT:User     -21750 USDT                    │  │  │
  │  │  │  CREDIT:  Liabilities:Fee:User -2.175 USDT                    │  │  │
  │  │  └───────────────────────────────────────────────────────────────┘  │  │
  │  │                                                                      │  │
  │  │  For SPOT SELL Trade (User sells BTC for USDT):                    │  │
  │  │  ┌───────────────────────────────────────────────────────────────┐  │  │
  │  │  │  DEBIT:   Assets:USDT:User     +21750 USDT                    │  │  │
  │  │  │  CREDIT:  Assets:BTC:User      -0.5 BTC                       │  │  │
  │  │  │  DEBIT:   Liabilities:Fee:User +2.175 USDT                    │  │  │
  │  │  └───────────────────────────────────────────────────────────────┘  │  │
  │  │                                                                      │  │
  │  │  For LINEAR BUY Position (User goes Long):                         │  │
  │  │  ┌───────────────────────────────────────────────────────────────┐  │  │
  │  │  │  DEBIT:   Positions:Long:BTC    +0.5 BTC                      │  │  │
  │  │  │  DEBIT:   Margin:Locked         -(21750 + 2.175) USDT         │  │  │
  │  │  │  CREDIT:  Assets:Available      -(21750 + 2.175) USDT         │  │  │
  │  │  └───────────────────────────────────────────────────────────────┘  │  │
  │  │                                                                      │  │
  │  │  ATOMIC TRANSACTION GUARANTEE:                                      │  │
  │  │  ┌───────────────────────────────────────────────────────────────┐  │  │
  │  │  │  BEGIN TRANSACTION                                             │  │  │
  │  │  │    1. Update User Balance (Debit/Credit)                      │  │  │
  │  │  │    2. Update Position (if derivatives)                        │  │  │
  │  │  │    3. Record Trade History                                     │  │  │
  │  │  │    4. Calculate PnL                                            │  │  │
  │  │  │    5. Update Account Equity                                    │  │  │
  │  │  │  COMMIT TRANSACTION                                            │  │  │
  │  │  │  IF ANY STEP FAILS: ROLLBACK ALL CHANGES                      │  │  │
  │  │  └───────────────────────────────────────────────────────────────┘  │  │
  │  │                                                                      │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  │  Step 7: SETTLEMENT & CLEARING                                            │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │                                                                      │  │
  │  │  IMMEDIATE SETTLEMENT (Centralized Exchange):                       │  │
  │  │  • Trades settle immediately upon execution                         │  │
  │  │  • Funds are instantly available for withdrawal/trading             │  │
  │  │                                                                      │  │
  │  │  DEFERRED SETTLEMENT (Some Futures/Options):                        │  │
  │  │  • Daily mark-to-market (PnL settled at end of day)                │  │
  │  │  • Variation margin posted intraday                                 │  │
  │  │  • Final settlement at contract expiry                              │  │
  │  │                                                                      │  │
  │  │  PnL CALCULATION:                                                   │  │
  │  │  ┌───────────────────────────────────────────────────────────────┐  │  │
  │  │  │  Realized PnL = (Exit Price - Entry Price) × Quantity          │  │  │
  │  │  │                                                                  │  │  │
  │  │  │  Unrealized PnL = (Current Price - Entry Price) × Quantity      │  │  │
  │  │  │                                                                  │  │  │
  │  │  │  Total Account Equity = Balance + Σ(Positions × Mark Price)     │  │  │
  │  │  └───────────────────────────────────────────────────────────────┘  │  │
  │  │                                                                      │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  │  Step 8: NOTIFICATION & MARKET DATA                                       │
  │  ┌────────────────────────────────────────────────────────────────────┐  │
  │  │  FIX Execution Report (35=8) sent to client:                       │  │
  │  │  • ExecType (150) = 0 (New) / 1 (Partial Fill) / 2 (Fill)         │  │
  │  │  • OrdStatus (39) = 0 (New) / 1 (Partial) / 2 (Filled)            │  │
  │  │  • CumQty (14) = Cumulative Filled Quantity                        │  │
  │  │  • AvgPx (6) = Average Execution Price                             │  │
  │  │  • Commission (12) = Trading Fee                                   │  │
  │  │                                                                      │  │
  │  │  Market Data Broadcast:                                             │  │
  │  │  • Trade Update (public trade data)                                │  │
  │  │  • Order Book Update (depth changes)                               │  │
  │  │  • Ticker Price Update (24h stats)                                 │  │
  │  │                                                                      │  │
  │  └────────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Consistency & Synchronization Mechanisms

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    CONSISTENCY GUARANTEE MECHANISMS                          │
└──────────────────────────────────────────────────────────────────────────────┘

  ┌─────────────────────────────────────────────────────────────────────────┐
  │                    SEQUENCER (Total Order Guarantee)                     │
  │                                                                         │
  │    Event A ─┐                                                           │
  │    Event B ─┼──► Sequencer ──► Sequenced Events ──► Matching Engine    │
  │    Event C ─┘      (Single                                    (Same Order)│
  │                    Writer)                                              │
  │                                                                         │
  │    Properties:                                                          │
  │    • Monotonically Increasing Sequence Numbers                          │
  │    • No Gaps (1, 2, 3, 4, ...)                                          │
  │    • Deterministic Ordering                                             │
  │    • Replay = Same Result                                               │
  └─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌─────────────────────────────────────────────────────────────────────────┐
  │                    KAFKA / MESSAGE QUEUE (Event Streaming)               │
  │                                                                         │
  │    Matching Engine ──► Trade Events ──► Kafka Topic                     │
  │                                      │                                  │
    Multiple Consumers:              ▼                                     │
    • Balance Service          ┌─────────────┐                             │
    • Settlement Service       │   Kafka     │                             │
    • Market Data Service      │   Cluster   │                             │
    • Audit Service            │  (Replicated│                             │
  │                              └─────────────┘                             │
  │                                                                         │
  │    Guarantees:                                                           │
  │    • Ordered Delivery (Partition-based)                                 │
  │    • At-Least-Once Delivery                                             │
  │    • Event Replay Capability                                            │
  │    • Replicated for Fault Tolerance                                      │
  └─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌─────────────────────────────────────────────────────────────────────────┐
  │                    DATABASE (ACID Transactions)                          │
  │                                                                         │
  │    Balance Service ──► PostgreSQL / FoundationDB                        │
  │                                                                         │
  │    ACID Properties:                                                      │
  │    ┌─────────────────────────────────────────────────────────────────┐  │
  │    │  ATOMICITY:                                                       │  │
  │    │  All balance changes succeed or fail together                    │  │
  │    │  No partial updates                                              │  │
  │    └─────────────────────────────────────────────────────────────────┘  │
  │    ┌─────────────────────────────────────────────────────────────────┐  │
  │    │  CONSISTENCY:                                                    │  │
  │    │  Database moves from one valid state to another                 │  │
  │    │  Invariants always maintained                                    │  │
  │    │  (e.g., Balance = Σ All Ledger Entries)                         │  │
  │    └─────────────────────────────────────────────────────────────────┘  │
  │    ┌─────────────────────────────────────────────────────────────────┐  │
  │    │  ISOLATION:                                                      │  │
  │    │  Concurrent updates don't interfere                             │  │
  │    │  Row-level locking or MVCC                                      │  │
  │    └─────────────────────────────────────────────────────────────────┘  │
  │    ┌─────────────────────────────────────────────────────────────────┐  │
  │    │  DURABILITY:                                                     │  │
  │    │  Committed transactions survive system failure                  │  │
  │    │  Write-ahead logging                                            │  │
  │    └─────────────────────────────────────────────────────────────────┘  │
  └─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌─────────────────────────────────────────────────────────────────────────┐
  │                    REPLICATION (High Availability)                       │
  │                                                                         │
  │    Primary Node ─┐                                                      │
  │    Replica 1 ───┼──► Consensus (Raft/Paxos) ──► Consistent State       │
  │    Replica 2 ───┘                            (All Nodes Agree)          │
  │                                                                         │
  │    If Primary Fails:                                                     │
  │    • Election of New Primary                                             │
  │    • Minimal Downtime                                                    │
  │    • No Data Loss                                                        │
  │    • Clients Failover to New Primary                                     │
  │                                                                         │
  └─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. FIX Protocol Message Flow

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         FIX PROTOCOL MESSAGE FLOW                             │
└──────────────────────────────────────────────────────────────────────────────┘

  CLIENT                                                           EXCHANGE
    │                                                                 │
    │  ┌───────────────────────────────────────────────────────────┐   │
    │  │  NewOrderSingle (35=D)                                    │   │
    │  │                                                           │   │
    │  │  8=FIX.4.4            BeginString                         │   │
    │  │  9=0123              BodyLength                           │   │
    │  │  35=D                 MsgType = New Order Single          │   │
    │  │  49=sender            SenderCompID                        │   │
    │  │  56=target            TargetCompID                        │   │
    │  │  11=ORD12345          ClOrdID (Client Order ID)           │   │
    │  │  55=BTC/USDT          Symbol                              │   │
    │  │  54=1                 Side (1=Buy, 2=Sell)                │   │
    │  │  38=0.5               OrderQty                            │   │
    │  │  40=2                 OrdType (2=Limit)                   │   │
    │  │  44=43500.00          Price                               │   │
    │  │  59=1                 TimeInForce (1=GTC)                │   │
    │  │  60=20240115...       TransactTime                        │   │
    │  │  10=000              Checksum                             │   │
    │  └───────────────────────────────────────────────────────────┘   │
    │  ───────────────────────────────────────────────────────────────► │
    │                                                                 │
    │                                                                 │
    │  ┌───────────────────────────────────────────────────────────┐   │
    │  │  ExecutionReport (35=8) - Order Acknowledged              │   │
    │  │                                                           │   │
    │  │  35=8                 MsgType = Execution Report          │   │
    │  │  11=ORD12345          ClOrdID                            │   │
    │  │  17=EXEC001           ExecID                             │   │
    │  │  150=0                ExecType (0=New)                   │   │
    │  │  39=0                 OrdStatus (0=New)                  │   │
    │  │  55=BTC/USDT          Symbol                             │   │
    │  │  54=1                 Side                               │   │
    │  │  38=0.5               OrderQty                           │   │
    │  │  44=43500.00          Price                              │   │
    │  │  150=0                ExecType                           │   │
    │  │  39=0                 OrdStatus                          │   │
    │  └───────────────────────────────────────────────────────────┘   │
    │  ◄─────────────────────────────────────────────────────────────── │
    │                                                                 │
    │  ... Order Working in Market ...                                  │
    │                                                                 │
    │  ┌───────────────────────────────────────────────────────────┐   │
    │  │  ExecutionReport (35=8) - Partial Fill                   │   │
    │  │                                                           │   │
    │  │  35=8                 MsgType                             │   │
    │  │  11=ORD12345          ClOrdID                            │   │
    │  │  17=EXEC002           ExecID                             │   │
    │  │  150=1                ExecType (1=Partial Fill)          │   │
    │  │  39=1                 OrdStatus (1=Partial Filled)       │   │
    │  │  14=0.25              CumQty (Cumulative Quantity)       │   │
    │  │  6=43500.00           AvgPx (Average Price)              │   │
    │  │  32=0.25              LastQty (Last Fill Quantity)       │   │
    │  │  31=43500.00          LastPx (Last Fill Price)           │   │
    │  │  12=0.087             Commission (Fee)                   │   │
    │  └───────────────────────────────────────────────────────────┘   │
    │  ◄─────────────────────────────────────────────────────────────── │
    │                                                                 │
    │  ┌───────────────────────────────────────────────────────────┐   │
    │  │  ExecutionReport (35=8) - Full Fill                      │   │
    │  │                                                           │   │
    │  │  35=8                 MsgType                             │   │
    │  │  11=ORD12345          ClOrdID                            │   │
    │  │  17=EXEC003           ExecID                             │   │
    │  │  150=2                ExecType (2=Fill)                  │   │
    │  │  39=2                 OrdStatus (2=Filled)               │   │
    │  │  14=0.5               CumQty                             │   │
    │  │  6=43500.00           AvgPx                              │   │
    │  │  32=0.25              LastQty                            │   │
    │  │  31=43500.00          LastPx                             │   │
    │  │  12=0.175             Commission                         │   │
    │  └───────────────────────────────────────────────────────────┘   │
    │  ◄─────────────────────────────────────────────────────────────── │
    │                                                                 │
    │  Alternative: Order Cancel Request (35=F)                        │
    │  ┌───────────────────────────────────────────────────────────┐   │
    │  │  35=F                  MsgType = Order Cancel Request     │   │
    │  │  11=ORD12345          OrigClOrdID                        │   │
    │  │  41=CANCEL001         ClOrdID (Cancel Request ID)        │   │
    │  │  55=BTC/USDT          Symbol                             │   │
    │  │  54=1                 Side                               │   │
    │  │  38=0.25              OrderQty                           │   │
    │  └───────────────────────────────────────────────────────────┘   │
    │  ───────────────────────────────────────────────────────────────► │
    │                                                                 │
    │  ┌───────────────────────────────────────────────────────────┐   │
    │  │  ExecutionReport (35=8) - Cancelled                      │   │
    │  │  150=4                ExecType (4=Cancelled)             │   │
    │  │  39=4                 OrdStatus (4=Cancelled)            │   │
    │  └───────────────────────────────────────────────────────────┘   │
    │  ◄─────────────────────────────────────────────────────────────── │
    │                                                                 │
    └─────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Order State Machine

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         ORDER STATE MACHINE                                   │
└──────────────────────────────────────────────────────────────────────────────┘

                            ┌─────────────────┐
                            │                 │
                            ▼                 │
                    ┌───────────────┐        │
                    │    PENDING    │        │ Client Submits Order
                    │  ACKNOWLEDGE  │        │
                    └───────┬───────┘        │
                            │                │
                            ▼                │
                    ┌───────────────┐        │
                    │   PENDING     │        │ Risk Check In Progress
                    │      RISK     │────────┤
                    └───────┬───────┘        │
                            │                │
                            ▼                │
              ┌─────────────┴─────────────┐   │
              │                           │   │
              ▼                           ▼   │
      ┌───────────────┐           ┌───────────────┐
      │     NEW       │           │   REJECTED    │──────────► Terminal
      │   (WORKING)   │           │               │           (End)
      └───────┬───────┘           └───────────────┘
              │
              │ Order Being Processed
              ▼
      ┌───────────────┐
      │    PARTIALLY  │
      │     FILLED    │
      └───────┬───────┘
              │
              │ Order Continues Matching
              ▼
      ┌───────────────┐
      │     FILLED    │──────────► Terminal
      │               │           (End)
      └───────────────┘


  Alternative Paths:

  ┌─────────────────────────────────────────────────────────────────────────┐
  │                                                                         │
  │  NEW ──────► CANCEL REQUEST ──────► PENDING CANCEL ──────► CANCELLED   │
  │                                                                         │
  │  NEW ──────► REPLACE REQUEST ──────► PENDING REPLACE ──────► NEW       │
  │              (Modify Order)                                              │
  │                                                                         │
  │  NEW ──────► EXPIRED (TimeInForce=IOC/FOK/GTD) ──────► EXPIRED         │
  │                                                                         │
  │  NEW ──────► SUSPENDED (Circuit Breaker) ──────► [Later] ─────► ...    │
  │                                                                         │
  └─────────────────────────────────────────────────────────────────────────┘


  FIX Protocol State Mapping:

  ┌─────────────────────────────────────────────────────────────────────────┐
  │  State              │ FIX ExecType(150) │ FIX OrdStatus(39)             │
  ├─────────────────────┼───────────────────┼───────────────────────────────┤
  │  PENDING            │ A (Pending New)   │ A (Pending New)               │
  │  NEW                │ 0 (New)           │ 0 (New)                       │
  │  PARTIALLY FILLED   │ 1 (Partial Fill)  │ 1 (Partial Filled)            │
  │  FILLED             │ 2 (Fill)          │ 2 (Filled)                    │
  │  CANCELLED          │ 4 (Cancelled)     │ 4 (Cancelled)                 │
  │  REJECTED           │ 8 (Rejected)      │ 8 (Rejected)                  │
  │  EXPIRED            │ C (Expired)       │ C (Expired)                   │
  │  PENDING CANCEL     │ 6 (Pending Cancel)│ 6 (Pending Cancel)            │
  │  PENDING REPLACE    │ 5 (Pending Replace)│ 5 (Pending Replace)          │
  └─────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Summary: Key Principles

**1. Determinism Through Sequencing**
- Все события проходят через единый sequencer
- Monotonically increasing sequence numbers гарантируют справедливость
- Replay всегда даёт тот же результат

**2. Atomic Balance Updates**
- Все изменения балансов происходят в рамках ACID транзакций
- Double-entry bookkeeping гарантирует сохранение баланса
- Нет частичных обновлений

**3. Separation of Concerns**
- Gateway: валидация и авторизация
- Risk Management: pre-trade checks
- Matching Engine: только matching (stateless)
- Balance Service: только учёт (stateful)
- Market Data Service: только публикация

**4. Event-Driven Architecture**
- Все события публикуются в Kafka
- Multiple consumers для разных задач
- Event replay для восстановления и аудита

**5. Replication & Fault Tolerance**
- Replicated state machines через Raft/Paxos
- Географическое распределение
- Automatic failover
