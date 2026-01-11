package duckdb

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb/v2"

	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Store struct {
	db          *sql.DB
	insertOrder *sql.Stmt
	insertTrade *sql.Stmt
	insertPnL   *sql.Stmt
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	insertOrder, err := db.Prepare(`
		INSERT INTO closed_orders
		(order_id, user_id, symbol, category, side, type, status, price, quantity, filled, closed_at, order_link_id, reduce_only)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	insertTrade, err := db.Prepare(`
		INSERT INTO trades
		(trade_id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = insertOrder.Close()
		_ = db.Close()
		return nil, err
	}
	insertPnL, err := db.Prepare(`
		INSERT INTO pnl
		(user_id, symbol, category, realized, fee_paid, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = insertOrder.Close()
		_ = insertTrade.Close()
		_ = db.Close()
		return nil, err
	}
	return &Store{
		db:          db,
		insertOrder: insertOrder,
		insertTrade: insertTrade,
		insertPnL:   insertPnL,
	}, nil
}

func (s *Store) Close() error {
	if s.insertOrder != nil {
		_ = s.insertOrder.Close()
	}
	if s.insertTrade != nil {
		_ = s.insertTrade.Close()
	}
	if s.insertPnL != nil {
		_ = s.insertPnL.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) InsertBatch(records []history.Record) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	orderStmt := tx.Stmt(s.insertOrder)
	tradeStmt := tx.Stmt(s.insertTrade)
	pnlStmt := tx.Stmt(s.insertPnL)
	for i := range records {
		switch records[i].Kind {
		case history.KIND_ORDER_CLOSED:
			e := records[i].OrderClosed
			if e == nil {
				_ = tx.Rollback()
				return history.ErrInvalidOrderClosedRecord()
			}
			if _, err := orderStmt.Exec(
				e.OrderID, e.UserID, e.Symbol, e.Category, e.Side, e.Type, e.Status,
				e.Price, e.Quantity, e.Filled, e.ClosedAt, e.OrderLink, e.ReduceOnly,
			); err != nil {
				_ = tx.Rollback()
				return err
			}
		case history.KIND_TRADE:
			e := records[i].Trade
			if e == nil {
				_ = tx.Rollback()
				return history.ErrInvalidTradeRecord()
			}
			if _, err := tradeStmt.Exec(
				e.TradeID, e.Symbol, e.Category, e.TakerID, e.MakerID, e.TakerOrder, e.MakerOrder,
				e.Price, e.Quantity, e.ExecutedAt,
			); err != nil {
				_ = tx.Rollback()
				return err
			}
		case history.KIND_PNL:
			e := records[i].PnL
			if e == nil {
				_ = tx.Rollback()
				return history.ErrInvalidPnLRecord()
			}
			if _, err := pnlStmt.Exec(
				e.UserID, e.Symbol, e.Category, e.Realized, e.FeePaid, e.CreatedAt,
			); err != nil {
				_ = tx.Rollback()
				return err
			}
		default:
			_ = tx.Rollback()
			return history.ErrUnknownRecordKind()
		}
	}
	return tx.Commit()
}

func (s *Store) GetOrderHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.Order, error) {
	query := `
		SELECT order_id, user_id, symbol, category, side, type, status, price, quantity, filled, closed_at, order_link_id, reduce_only
		FROM closed_orders
		WHERE user_id = ?`
	args := []interface{}{int64(userID)}
	if symbol != "" {
		query += " AND symbol = ?"
		args = append(args, symbol)
	}
	query += " ORDER BY closed_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	orders := make([]*types.Order, 0)
	for rows.Next() {
		var order types.Order
		var orderID int64
		var user int64
		var price, qty, filled int64
		var closedAt int64
		var orderLinkID int64
		var reduceOnly bool
		if err := rows.Scan(&orderID, &user, &order.Symbol, &order.Category, &order.Side, &order.Type, &order.Status,
			&price, &qty, &filled, &closedAt, &orderLinkID, &reduceOnly); err != nil {
			return nil, err
		}
		order.ID = types.OrderID(orderID)
		order.UserID = types.UserID(user)
		order.Price = types.Price(price)
		order.Quantity = types.Quantity(qty)
		order.Filled = types.Quantity(filled)
		order.ReduceOnly = reduceOnly
		order.OrderLinkId = orderLinkID
		order.UpdatedAt = uint64(closedAt)
		order.ClosedAt = uint64(closedAt)
		orders = append(orders, &order)
	}
	return orders, rows.Err()
}

func (s *Store) GetTradeHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.Trade, error) {
	query := `
		SELECT trade_id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at
		FROM trades
		WHERE (taker_id = ? OR maker_id = ?)`
	args := []interface{}{int64(userID), int64(userID)}
	if symbol != "" {
		query += " AND symbol = ?"
		args = append(args, symbol)
	}
	query += " ORDER BY executed_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	trades := make([]*types.Trade, 0)
	for rows.Next() {
		var trade types.Trade
		var tradeID int64
		var takerID, makerID int64
		var takerOrderID, makerOrderID int64
		var price, qty int64
		var executedAt int64
		if err := rows.Scan(&tradeID, &trade.Symbol, &trade.Category, &takerID, &makerID, &takerOrderID, &makerOrderID, &price, &qty, &executedAt); err != nil {
			return nil, err
		}
		trade.ID = types.TradeID(tradeID)
		trade.TakerID = types.UserID(takerID)
		trade.MakerID = types.UserID(makerID)
		trade.TakerOrderID = types.OrderID(takerOrderID)
		trade.MakerOrderID = types.OrderID(makerOrderID)
		trade.Price = types.Price(price)
		trade.Quantity = types.Quantity(qty)
		trade.ExecutedAt = uint64(executedAt)
		trades = append(trades, &trade)
	}
	return trades, rows.Err()
}

func (s *Store) GetRPNLHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.RPNLEvent, error) {
	query := `
		SELECT user_id, symbol, category, realized, created_at
		FROM pnl
		WHERE user_id = ?`
	args := []interface{}{int64(userID)}
	if symbol != "" {
		query += " AND symbol = ?"
		args = append(args, symbol)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	events := make([]*types.RPNLEvent, 0)
	for rows.Next() {
		var event types.RPNLEvent
		var user int64
		var realized int64
		var createdAt int64
		if err := rows.Scan(&user, &event.Symbol, &event.Category, &realized, &createdAt); err != nil {
			return nil, err
		}
		event.UserID = types.UserID(user)
		event.RealizedPnl = realized
		event.ExecutedAt = uint64(createdAt)
		events = append(events, &event)
	}
	return events, rows.Err()
}

func migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS closed_orders (
			order_id BIGINT,
			user_id BIGINT,
			symbol TEXT,
			category TINYINT,
			side TINYINT,
			type TINYINT,
			status TINYINT,
			price BIGINT,
			quantity BIGINT,
			filled BIGINT,
			closed_at BIGINT,
			order_link_id BIGINT,
			reduce_only BOOLEAN
		)`,
		`CREATE TABLE IF NOT EXISTS trades (
			trade_id BIGINT,
			symbol TEXT,
			category TINYINT,
			taker_id BIGINT,
			maker_id BIGINT,
			taker_order_id BIGINT,
			maker_order_id BIGINT,
			price BIGINT,
			quantity BIGINT,
			executed_at BIGINT
		)`,
		`CREATE TABLE IF NOT EXISTS pnl (
			user_id BIGINT,
			symbol TEXT,
			category TINYINT,
			realized BIGINT,
			fee_paid BIGINT,
			created_at BIGINT
		)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
