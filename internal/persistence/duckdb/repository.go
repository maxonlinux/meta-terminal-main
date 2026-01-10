package duckdb

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"sync"

	_ "github.com/marcboeker/go-duckdb"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	Path string
}

type Repository struct {
	db *sql.DB
	mu sync.RWMutex
}

func New(cfg Config) (*Repository, error) {
	db, err := sql.Open("duckdb", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping duckdb: %w", err)
	}

	repo := &Repository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return repo, nil
}

func (r *Repository) initSchema() error {
	queries := []string{
		`CREATE SEQUENCE IF NOT EXISTS seq_orders_id`,
		`CREATE TABLE IF NOT EXISTS orders (
			id U BIGINT PRIMARY KEY DEFAULT nextval('seq_orders_id'),
			user_id U BIGINT NOT NULL,
			symbol VARCHAR NOT NULL,
			category TINYINT NOT NULL,
			side TINYINT NOT NULL,
			type TINYINT NOT NULL,
			tif TINYINT NOT NULL,
			status TINYINT NOT NULL,
			price BIGINT NOT NULL,
			quantity BIGINT NOT NULL,
			filled BIGINT NOT NULL,
			trigger_price BIGINT NOT NULL,
			reduce_only BOOLEAN NOT NULL,
			created_at U BIGINT NOT NULL,
			updated_at U BIGINT NOT NULL,
			closed_at U BIGINT,
			stop_order_type TINYINT DEFAULT 0,
			close_on_trigger BOOLEAN DEFAULT FALSE
		)`,

		`CREATE SEQUENCE IF NOT EXISTS seq_trades_id`,
		`CREATE TABLE IF NOT EXISTS trades (
			id U BIGINT PRIMARY KEY DEFAULT nextval('seq_trades_id'),
			symbol VARCHAR NOT NULL,
			category TINYINT NOT NULL,
			taker_id U BIGINT NOT NULL,
			maker_id U BIGINT NOT NULL,
			taker_order_id U BIGINT NOT NULL,
			maker_order_id U BIGINT NOT NULL,
			price BIGINT NOT NULL,
			quantity BIGINT NOT NULL,
			executed_at U BIGINT NOT NULL
		)`,

		`CREATE SEQUENCE IF NOT EXISTS seq_rpnl_id`,
		`CREATE TABLE IF NOT EXISTS rpnl (
			id U BIGINT PRIMARY KEY DEFAULT nextval('seq_rpnl_id'),
			user_id U BIGINT NOT NULL,
			symbol VARCHAR NOT NULL,
			category TINYINT NOT NULL,
			realized_pnl BIGINT NOT NULL,
			position_size BIGINT NOT NULL,
			position_side TINYINT NOT NULL,
			entry_price BIGINT NOT NULL,
			exit_price BIGINT NOT NULL,
			executed_at U BIGINT NOT NULL
		)`,

		`CREATE OR REPLACE VIEW positions AS
		SELECT 
			user_id,
			symbol,
			category,
			SUM(CASE WHEN side = 0 THEN quantity ELSE -quantity END) as size,
			CASE 
				WHEN SUM(CASE WHEN side = 0 THEN quantity ELSE -quantity END) > 0 THEN 0
				WHEN SUM(CASE WHEN side = 0 THEN quantity ELSE -quantity END) < 0 THEN 1
				ELSE -1
			END as side
		FROM trades
		GROUP BY user_id, symbol, category`,

		`CREATE INDEX IF NOT EXISTS idx_orders_user_symbol ON orders(user_id, symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_closed_at ON orders(closed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_executed_at ON trades(executed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_rpnl_user_symbol ON rpnl(user_id, symbol)`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return fmt.Errorf("exec query: %w", err)
		}
	}

	return nil
}

func (r *Repository) InsertOrder(order *types.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		INSERT INTO orders (user_id, symbol, category, side, type, tif, status, price, quantity, filled, trigger_price, reduce_only, created_at, updated_at, closed_at, stop_order_type, close_on_trigger)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id
	`

	return r.db.QueryRow(query,
		order.UserID,
		order.Symbol,
		order.Category,
		order.Side,
		order.Type,
		order.TIF,
		order.Status,
		order.Price,
		order.Quantity,
		order.Filled,
		order.TriggerPrice,
		order.ReduceOnly,
		order.CreatedAt,
		order.UpdatedAt,
		order.ClosedAt,
		order.StopOrderType,
		order.CloseOnTrigger,
	).Scan(&order.ID)
}

func (r *Repository) UpdateOrderStatus(orderID types.OrderID, status int8, filled int64, updatedAt uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `UPDATE orders SET status = $1, filled = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.Exec(query, status, filled, updatedAt, orderID)
	return err
}

func (r *Repository) InsertTrade(trade *types.Trade) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		INSERT INTO trades (symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	return r.db.QueryRow(query,
		trade.Symbol,
		trade.Category,
		trade.TakerID,
		trade.MakerID,
		trade.TakerOrderID,
		trade.MakerOrderID,
		trade.Price,
		trade.Quantity,
		trade.ExecutedAt,
	).Scan(&trade.ID)
}

func (r *Repository) InsertRPNL(rpnl *types.RPNLEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		INSERT INTO rpnl (user_id, symbol, category, realized_pnl, position_size, position_side, entry_price, exit_price, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	return r.db.QueryRow(query,
		rpnl.UserID,
		rpnl.Symbol,
		rpnl.Category,
		rpnl.RealizedPnl,
		rpnl.PositionSize,
		rpnl.PositionSide,
		rpnl.EntryPrice,
		rpnl.ExitPrice,
		rpnl.ExecutedAt,
	).Scan(&rpnl.ID)
}

func (r *Repository) GetOrders(userID types.UserID, symbol string, limit int) ([]*types.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var query string
	var args []interface{}

	if symbol != "" {
		query = `SELECT id, user_id, symbol, category, side, type, tif, status, price, quantity, filled, trigger_price, reduce_only, created_at, updated_at, closed_at, stop_order_type, close_on_trigger
		        FROM orders WHERE user_id = $1 AND symbol = $2 ORDER BY created_at DESC LIMIT $3`
		args = []interface{}{userID, symbol, limit}
	} else {
		query = `SELECT id, user_id, symbol, category, side, type, tif, status, price, quantity, filled, trigger_price, reduce_only, created_at, updated_at, closed_at, stop_order_type, close_on_trigger
		        FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
		args = []interface{}{userID, limit}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*types.Order
	for rows.Next() {
		o := &types.Order{}
		err := rows.Scan(
			&o.ID, &o.UserID, &o.Symbol, &o.Category, &o.Side, &o.Type, &o.TIF, &o.Status,
			&o.Price, &o.Quantity, &o.Filled, &o.TriggerPrice, &o.ReduceOnly,
			&o.CreatedAt, &o.UpdatedAt, &o.ClosedAt, &o.StopOrderType, &o.CloseOnTrigger,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	return orders, rows.Err()
}

func (r *Repository) GetTrades(userID types.UserID, symbol string, limit int) ([]*types.Trade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var query string
	var args []interface{}

	if symbol != "" {
		query = `SELECT id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at
		        FROM trades WHERE (taker_id = $1 OR maker_id = $1) AND symbol = $2 ORDER BY executed_at DESC LIMIT $3`
		args = []interface{}{userID, symbol, limit}
	} else {
		query = `SELECT id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at
		        FROM trades WHERE (taker_id = $1 OR maker_id = $1) ORDER BY executed_at DESC LIMIT $2`
		args = []interface{}{userID, limit}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*types.Trade
	for rows.Next() {
		t := &types.Trade{}
		err := rows.Scan(
			&t.ID, &t.Symbol, &t.Category, &t.TakerID, &t.MakerID, &t.TakerOrderID, &t.MakerOrderID,
			&t.Price, &t.Quantity, &t.ExecutedAt,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}

	return trades, rows.Err()
}

func (r *Repository) GetPositions(userID types.UserID, symbol string) ([]types.Position, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var query string
	var args []interface{}

	if symbol != "" {
		query = `SELECT symbol, size, side FROM positions WHERE user_id = $1 AND symbol = $2`
		args = []interface{}{userID, symbol}
	} else {
		query = `SELECT symbol, size, side FROM positions WHERE user_id = $1`
		args = []interface{}{userID}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []types.Position
	for rows.Next() {
		p := types.Position{}
		err := rows.Scan(&p.Symbol, &p.Size, &p.Side)
		if err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}

	return positions, rows.Err()
}

func (r *Repository) BatchInsertOrders(orders []*types.Order) error {
	if len(orders) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO orders (user_id, symbol, category, side, type, tif, status, price, quantity, filled, trigger_price, reduce_only, created_at, updated_at, closed_at, stop_order_type, close_on_trigger)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, order := range orders {
		_, err := stmt.Exec(
			order.UserID, order.Symbol, order.Category, order.Side, order.Type, order.TIF,
			order.Status, order.Price, order.Quantity, order.Filled, order.TriggerPrice,
			order.ReduceOnly, order.CreatedAt, order.UpdatedAt,
			order.ClosedAt, order.StopOrderType, order.CloseOnTrigger,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) BatchInsertTrades(trades []*types.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO trades (symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, trade := range trades {
		_, err := stmt.Exec(
			trade.Symbol, trade.Category, trade.TakerID, trade.MakerID,
			trade.TakerOrderID, trade.MakerOrderID, trade.Price, trade.Quantity,
			trade.ExecutedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) BatchInsertRPNL(rpnls []*types.RPNLEvent) error {
	if len(rpnls) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO rpnl (user_id, symbol, category, realized_pnl, position_size, position_side, entry_price, exit_price, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, rpnl := range rpnls {
		_, err := stmt.Exec(
			rpnl.UserID, rpnl.Symbol, rpnl.Category, rpnl.RealizedPnl,
			rpnl.PositionSize, rpnl.PositionSide, rpnl.EntryPrice, rpnl.ExitPrice, rpnl.ExecutedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) BatchInsertOutboxOrders(entries []*types.Order) error {
	if len(entries) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO orders (id, user_id, symbol, category, side, type, tif, status, price, quantity, filled, trigger_price, reduce_only, created_at, updated_at, closed_at, stop_order_type, close_on_trigger, is_conditional, order_link_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		_, err := stmt.Exec(
			e.ID, e.UserID, e.Symbol, e.Category, e.Side, e.Type, e.TIF,
			e.Status, e.Price, e.Quantity, e.Filled, e.TriggerPrice,
			e.ReduceOnly, e.CreatedAt, e.UpdatedAt, e.ClosedAt, e.StopOrderType, e.CloseOnTrigger, e.IsConditional, e.OrderLinkId,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) BatchInsertOutboxTrades(entries []*types.Trade) error {
	if len(entries) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO trades (id, symbol, category, taker_id, maker_id, taker_order_id, maker_order_id, price, quantity, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		_, err := stmt.Exec(
			e.ID, e.Symbol, e.Category, e.TakerID, e.MakerID,
			e.TakerOrderID, e.MakerOrderID, e.Price, e.Quantity, e.ExecutedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) BatchInsertOutboxRPNL(entries []*types.RPNLEvent) error {
	if len(entries) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO rpnl (id, user_id, symbol, category, realized_pnl, position_size, position_side, entry_price, exit_price, executed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		_, err := stmt.Exec(
			e.ID, e.UserID, e.Symbol, e.Category, e.RealizedPnl,
			e.PositionSize, e.PositionSide, e.EntryPrice, e.ExitPrice, e.ExecutedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func DecodeOutboxOrders(data []byte) ([]*types.Order, error) {
	var entries []*types.Order
	dec := gob.NewDecoder(bytes.NewReader(data))
	for {
		var e types.Order
		if err := dec.Decode(&e); err != nil {
			break
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

func DecodeOutboxTrades(data []byte) ([]*types.Trade, error) {
	var entries []*types.Trade
	dec := gob.NewDecoder(bytes.NewReader(data))
	for {
		var e types.Trade
		if err := dec.Decode(&e); err != nil {
			break
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

func DecodeOutboxRPNLs(data []byte) ([]*types.RPNLEvent, error) {
	var entries []*types.RPNLEvent
	dec := gob.NewDecoder(bytes.NewReader(data))
	for {
		var e types.RPNLEvent
		if err := dec.Decode(&e); err != nil {
			break
		}
		entries = append(entries, &e)
	}
	return entries, nil
}
