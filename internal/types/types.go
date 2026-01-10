package types

import "time"

type OrderID int64
type TradeID int64
type UserID uint64
type Price int64
type Quantity int64

func NowNano() uint64 { return uint64(time.Now().UnixNano()) }

// Order — торговая заявка в системе
//
// OrderLinkId — группа связанных ордеров (OCO, TP+SL)
//
//	> 0 = snowflake ID группы (все ордера в группе имеют одинаковый ID)
//	-1 = нет группы (обычные ордера, одиночные conditional)
//
// Пример: OCO создаёт 2 ордера с одинаковым OrderLinkId = snowflake.Next()
// При срабатывании одного — второй отменяется по этому ID
type Order struct {
	ID       OrderID
	UserID   UserID
	Symbol   string
	Category int8

	Side int8
	Type int8
	TIF  int8

	Status int8

	Price    Price
	Quantity Quantity
	Filled   Quantity

	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8

	// IsConditional — true если ордер conditional (TriggerPrice > 0)
	// Для удобства проверки типа ордера в коде
	IsConditional bool

	// OrderLinkId — группа OCO/TP/SL
	// > 0 = snowflake ID группы (OCO, связанные TP/SL)
	// -1 = нет группы (обычные ордера, одиночные conditional)
	OrderLinkId int64

	CreatedAt uint64
	UpdatedAt uint64
	ClosedAt  uint64
}

func (o *Order) Remaining() Quantity { return o.Quantity - o.Filled }

type Trade struct {
	ID       TradeID
	Symbol   string
	Category int8

	TakerID      UserID
	MakerID      UserID
	TakerOrderID OrderID
	MakerOrderID OrderID

	Price    Price
	Quantity Quantity

	ExecutedAt uint64
}

type Match struct {
	Trade *Trade
	Maker *Order
}

type OrderInput struct {
	UserID   UserID
	Symbol   string
	Category int8

	Side int8
	Type int8
	TIF  int8

	Quantity Quantity
	Price    Price

	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8

	// IsConditional — true если TriggerPrice > 0
	// Устанавливается автоматически при валидации для удобства
	IsConditional bool

	OCO *OCOInput `json:"oco,omitempty"`
}

// OCOInput — параметры OCO (One-Cancels-the-Other)
// Создаёт 2 связанных ордера: Take Profit + Stop Loss
//
// Правила OCO:
//  1. Quantity=0 → auto-use position size + reduceOnly=true
//  2. Оба ордера создаются как CloseOnTrigger=true
//  3. Оба ордера получают одинаковый OrderLinkId (snowflake ID)
//  4. При срабатывании TP → SL автоматически cancelled
//  5. При срабатывании SL → TP автоматически cancelled
//  6. Если позиция закрыта другим способом → оба cancelled
type OCOInput struct {
	Quantity   Quantity
	TakeProfit OCOChildOrder
	StopLoss   OCOChildOrder
}

type OCOChildOrder struct {
	TriggerPrice Price
	Price        Price
	ReduceOnly   bool
}

// OrderResult — результат PlaceOrder
// ВСЕГДА возвращается массив orders[] (1 элемент для single, 2+ для OCO/batch)
//
// OCO behavior:
//   - Создаётся 2 ордера: TP и SL
//   - Оба CloseOnTrigger = true
//   - Оба StopOrderType = TAKE_PROFIT / STOP_LOSS
//   - Оба OrderLinkId = ID группы (snowflake)
//   - При срабатывании одного — второй отменяется по OrderLinkId
//
// API Response Example:
//
//	Single order:
//	{
//	    "orders": [{
//	        "id": 12345,
//	        "symbol": "BTCUSDT",
//	        "side": "Buy",
//	        "status": "NEW"
//	    }],
//	    "filled": 0,
//	    "remaining": 1.0
//	}
//
//	OCO order:
//	{
//	    "orders": [
//	        {"id": 12346, "symbol": "BTCUSDT", "side": "Sell", "stopOrderType": 2, "orderLinkId": 99999},
//	        {"id": 12347, "symbol": "BTCUSDT", "side": "Sell", "stopOrderType": 3, "orderLinkId": 99999}
//	    ],
//	    "filled": 0,
//	    "remaining": 1.0
//	}
type OrderResult struct {
	Orders    []*Order // Все созданные ордера (1 = single, 2 = OCO)
	Trades    []*Trade // Сделки если были (для primary order)
	Filled    Quantity // Сумма filled для primary order
	Remaining Quantity // Сумма remaining для primary order
	Status    int8     // Статус primary order
}

type Position struct {
	Symbol     string
	Size       int64
	Side       int8
	EntryPrice int64
	Leverage   int8
}

type UserBalance struct {
	Asset     string
	Available int64
	Locked    int64
	Margin    int64
}

type Instrument struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	PricePrec  int8
	QtyPrec    int8
	MinQty     int64
	MaxQty     int64
	MinPrice   int64
	MaxPrice   int64
	TickSize   int64
	LotSize    int64
}

// Event types for persistence
type OrderEvent struct {
	OrderID      OrderID
	UserID       UserID
	Symbol       string
	Category     int8
	Side         int8
	Type         int8
	TIF          int8
	Status       int8
	Price        Price
	Quantity     Quantity
	Filled       Quantity
	TriggerPrice Price
	ReduceOnly   bool
	CreatedAt    uint64
	UpdatedAt    uint64
}

type TradeEvent struct {
	TradeID      TradeID
	Symbol       string
	Category     int8
	TakerID      UserID
	MakerID      UserID
	TakerOrderID OrderID
	MakerOrderID OrderID
	Price        Price
	Quantity     Quantity
	ExecutedAt   uint64
}

type RPNLEvent struct {
	ID           uint64
	UserID       UserID
	Symbol       string
	Category     int8
	RealizedPnl  int64
	PositionSize int64
	PositionSide int8
	EntryPrice   int64
	ExitPrice    int64
	ExecutedAt   uint64
}

type PositionReducedEvent struct {
	UserID       UserID
	Symbol       string
	Category     int8
	ClosedQty    int64
	ExitPrice    int64
	RPNL         int64
	PositionSize int64
	PositionSide int8
	ExecutedAt   uint64
}
