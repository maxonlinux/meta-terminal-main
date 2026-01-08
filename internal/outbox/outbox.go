package outbox

import (
	"database/sql"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

const (
	recordTrade uint8 = 1
	recordOrder uint8 = 2
)

type tradeRow struct {
	id           types.TradeID
	symbol       string
	category     int8
	takerID      types.UserID
	makerID      types.UserID
	takerOrderID types.OrderID
	makerOrderID types.OrderID
	price        types.Price
	qty          types.Quantity
	executedAt   uint64
}

type orderRow struct {
	id             types.OrderID
	userID         types.UserID
	symbol         string
	category       int8
	side           int8
	orderType      int8
	tif            int8
	status         int8
	price          types.Price
	qty            types.Quantity
	filled         types.Quantity
	triggerPrice   types.Price
	reduceOnly     bool
	closeOnTrigger bool
	stopOrderType  int8
	leverage       int8
	createdAt      uint64
	updatedAt      uint64
}

type Record struct {
	kind  uint8
	trade tradeRow
	order orderRow
}

type Outbox struct {
	mu         sync.Mutex
	db         *sql.DB
	batchSize  int
	flushEvery time.Duration
	lastFlush  time.Time
	buffer     []Record
}

func NewSQLite(db *sql.DB, batchSize int, flushEvery time.Duration) *Outbox {
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushEvery <= 0 {
		flushEvery = 5 * time.Second
	}
	return &Outbox{
		db:         db,
		batchSize:  batchSize,
		flushEvery: flushEvery,
		lastFlush:  time.Now(),
		buffer:     make([]Record, 0, batchSize),
	}
}

func (o *Outbox) AppendTrade(trade *types.Trade) error {
	if o == nil || o.db == nil || trade == nil {
		return nil
	}
	rec := Record{kind: recordTrade}
	rec.trade = tradeRow{
		id:           trade.ID,
		symbol:       trade.Symbol,
		category:     trade.Category,
		takerID:      trade.TakerID,
		makerID:      trade.MakerID,
		takerOrderID: trade.TakerOrderID,
		makerOrderID: trade.MakerOrderID,
		price:        trade.Price,
		qty:          trade.Quantity,
		executedAt:   trade.ExecutedAt,
	}
	return o.append(rec)
}

func (o *Outbox) AppendOrder(order *types.Order) error {
	if o == nil || o.db == nil || order == nil {
		return nil
	}
	rec := Record{kind: recordOrder}
	rec.order = orderRow{
		id:             order.ID,
		userID:         order.UserID,
		symbol:         order.Symbol,
		category:       order.Category,
		side:           order.Side,
		orderType:      order.Type,
		tif:            order.TIF,
		status:         order.Status,
		price:          order.Price,
		qty:            order.Quantity,
		filled:         order.Filled,
		triggerPrice:   order.TriggerPrice,
		reduceOnly:     order.ReduceOnly,
		closeOnTrigger: order.CloseOnTrigger,
		stopOrderType:  order.StopOrderType,
		leverage:       order.Leverage,
		createdAt:      order.CreatedAt,
		updatedAt:      order.UpdatedAt,
	}
	return o.append(rec)
}

func (o *Outbox) append(rec Record) error {
	o.mu.Lock()
	o.buffer = append(o.buffer, rec)
	flush := len(o.buffer) >= o.batchSize || time.Since(o.lastFlush) >= o.flushEvery
	o.mu.Unlock()
	if flush {
		return o.Flush()
	}
	return nil
}

func (o *Outbox) Flush() error {
	if o == nil || o.db == nil {
		return nil
	}
	o.mu.Lock()
	if len(o.buffer) == 0 {
		o.lastFlush = time.Now()
		o.mu.Unlock()
		return nil
	}
	buf := make([]Record, len(o.buffer))
	copy(buf, o.buffer)
	o.buffer = o.buffer[:0]
	o.lastFlush = time.Now()
	o.mu.Unlock()

	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	for i := range buf {
		rec := buf[i]
		switch rec.kind {
		case recordTrade:
			_, err = tx.Exec(`
INSERT INTO trades_history (trade_id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, qty, executed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				int64(rec.trade.id), rec.trade.symbol, int(rec.trade.category), int64(rec.trade.takerID), int64(rec.trade.makerID),
				int64(rec.trade.takerOrderID), int64(rec.trade.makerOrderID), int64(rec.trade.price), int64(rec.trade.qty), rec.trade.executedAt)
		case recordOrder:
			reduceOnly := 0
			if rec.order.reduceOnly {
				reduceOnly = 1
			}
			closeOnTrigger := 0
			if rec.order.closeOnTrigger {
				closeOnTrigger = 1
			}
			_, err = tx.Exec(`
INSERT INTO orders_history (order_id, user_id, symbol, category, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, leverage, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				int64(rec.order.id), int64(rec.order.userID), rec.order.symbol, int(rec.order.category), int(rec.order.side),
				int(rec.order.orderType), int(rec.order.tif), int(rec.order.status), int64(rec.order.price), int64(rec.order.qty),
				int64(rec.order.filled), int64(rec.order.triggerPrice), reduceOnly, closeOnTrigger, int(rec.order.stopOrderType),
				int(rec.order.leverage), rec.order.createdAt, rec.order.updatedAt)
		default:
			continue
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (o *Outbox) Close() error {
	if o == nil || o.db == nil {
		return nil
	}
	return o.Flush()
}
