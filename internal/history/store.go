package history

import (
	"database/sql"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Reader interface {
	GetOrders(userID types.UserID, symbol string, category int8, limit int) []types.Order
	GetTrades(userID types.UserID, symbol string, category int8, limit int) []types.Trade
}

type Store struct {
	db *sql.DB
}

func NewSQLite(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetOrders(userID types.UserID, symbol string, category int8, limit int) []types.Order {
	if s == nil || s.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
SELECT order_id, user_id, symbol, category, side, type, tif, status,
       price, qty, filled, trigger_price, reduce_only, close_on_trigger,
       stop_order_type, leverage, created_at, updated_at
FROM orders_history
WHERE user_id = ? AND symbol = ? AND category = ?
ORDER BY updated_at DESC
LIMIT ?`, int64(userID), symbol, int(category), limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]types.Order, 0, limit)
	for rows.Next() {
		var order types.Order
		var reduceOnly int
		var closeOnTrigger int
		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Symbol,
			&order.Category,
			&order.Side,
			&order.Type,
			&order.TIF,
			&order.Status,
			&order.Price,
			&order.Quantity,
			&order.Filled,
			&order.TriggerPrice,
			&reduceOnly,
			&closeOnTrigger,
			&order.StopOrderType,
			&order.Leverage,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return out
		}
		order.ReduceOnly = reduceOnly != 0
		order.CloseOnTrigger = closeOnTrigger != 0
		out = append(out, order)
	}
	return out
}

func (s *Store) GetTrades(userID types.UserID, symbol string, category int8, limit int) []types.Trade {
	if s == nil || s.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
SELECT trade_id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id,
       price, qty, executed_at
FROM trades_history
WHERE symbol = ? AND category = ? AND (taker_id = ? OR maker_id = ?)
ORDER BY executed_at DESC
LIMIT ?`, symbol, int(category), int64(userID), int64(userID), limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]types.Trade, 0, limit)
	for rows.Next() {
		var trade types.Trade
		if err := rows.Scan(
			&trade.ID,
			&trade.Symbol,
			&trade.Category,
			&trade.TakerID,
			&trade.MakerID,
			&trade.TakerOrderID,
			&trade.MakerOrderID,
			&trade.Price,
			&trade.Quantity,
			&trade.ExecutedAt,
		); err != nil {
			return out
		}
		out = append(out, trade)
	}
	return out
}
