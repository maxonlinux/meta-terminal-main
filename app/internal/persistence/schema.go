package persistence

import (
	"database/sql"
)

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
    create table if not exists orders (
      id integer primary key,
      user_id integer not null,
      symbol text not null,
      category integer not null,
      origin integer not null,
      side integer not null,
      type integer not null,
      tif integer not null,
      status integer not null,
      price text not null,
      qty text not null,
      filled text not null,
      trigger_price text not null,
      reduce_only integer not null,
      close_on_trigger integer not null,
      stop_order_type integer not null,
      trigger_direction integer not null default 0,
      is_conditional integer not null,
      created_at integer not null,
      updated_at integer not null
    );

    create index if not exists orders_user_idx on orders (user_id, updated_at);
    create index if not exists orders_symbol_idx on orders (symbol, category, updated_at);
    create index if not exists orders_user_status_idx on orders (user_id, status, updated_at);
    create index if not exists orders_symbol_status_idx on orders (symbol, category, status, updated_at);



    create table if not exists fills (
      id integer not null,
      user_id integer not null,
      order_id integer not null,
      counterparty_order_id integer not null,
      symbol text not null,
      category integer not null,
      order_type integer not null,
      side integer not null,
      role text not null,
      price text not null,
      qty text not null,
      ts integer not null,
      primary key (id, user_id, role)
    );

    create index if not exists fills_user_idx on fills (user_id, ts);
    create index if not exists fills_symbol_idx on fills (symbol, category, ts);

    create table if not exists balances (
      user_id integer not null,
      asset text not null,
      available text not null,
      locked text not null,
      margin text not null,
      primary key (user_id, asset)
    );

    create index if not exists balances_user_idx on balances (user_id);

    create table if not exists positions (
      user_id integer not null,
      symbol text not null,
      size text not null,
      entry_price text not null,
      exit_price text not null,
      mode integer not null,
      mm text not null,
      im text not null,
      liq_price text not null,
      leverage text not null,
      take_profit text not null,
      stop_loss text not null,
      tp_order_id integer not null,
      sl_order_id integer not null,
      primary key (user_id, symbol)
    );

    create index if not exists positions_user_idx on positions (user_id);

    create table if not exists fundings (
      id integer primary key,
      user_id integer not null,
      type text not null,
      status text not null,
      asset text not null,
      amount text not null,
      destination text not null,
      created_by text not null,
      message text not null,
      created_at integer not null,
      updated_at integer not null
    );

    create index if not exists fundings_user_idx on fundings (user_id, updated_at);

    create table if not exists rpnl_events (
      id integer primary key,
      user_id integer not null,
      order_id integer not null,
      symbol text not null,
      category integer not null,
      side integer not null,
      price text not null,
      qty text not null,
      realized text not null,
      created_at integer not null
    );

    create index if not exists rpnl_user_idx on rpnl_events (user_id, created_at);
    create index if not exists rpnl_symbol_idx on rpnl_events (symbol, category, created_at);
  `)
	if err != nil {
		return err
	}

	return nil
}
