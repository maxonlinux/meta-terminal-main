package duckdb

import (
	"context"
	"database/sql"

	_ "github.com/marcboeker/go-duckdb"

	"github.com/anomalyco/meta-terminal-go/internal/history"
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
