package storage

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func OpenSQLite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA temp_store=MEMORY;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func EnsureSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS orders_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	order_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	symbol TEXT NOT NULL,
	category INTEGER NOT NULL,
	side INTEGER NOT NULL,
	type INTEGER NOT NULL,
	tif INTEGER NOT NULL,
	status INTEGER NOT NULL,
	price INTEGER NOT NULL,
	qty INTEGER NOT NULL,
	filled INTEGER NOT NULL,
	trigger_price INTEGER NOT NULL,
	reduce_only INTEGER NOT NULL,
	close_on_trigger INTEGER NOT NULL,
	stop_order_type INTEGER NOT NULL,
	leverage INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS orders_history_user_symbol ON orders_history(user_id, symbol, category, updated_at);

CREATE TABLE IF NOT EXISTS trades_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	trade_id INTEGER NOT NULL,
	symbol TEXT NOT NULL,
	category INTEGER NOT NULL,
	taker_id INTEGER NOT NULL,
	maker_id INTEGER NOT NULL,
	taker_order_id INTEGER NOT NULL,
	maker_order_id INTEGER NOT NULL,
	price INTEGER NOT NULL,
	qty INTEGER NOT NULL,
	executed_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS trades_history_symbol ON trades_history(symbol, category, executed_at);
CREATE INDEX IF NOT EXISTS trades_history_taker ON trades_history(taker_id, executed_at);
CREATE INDEX IF NOT EXISTS trades_history_maker ON trades_history(maker_id, executed_at);
`)
	return err
}
