package persistence

import "database/sql"

type statements struct {
	upsertOrder             *sql.Stmt
	upsertOpenOrder         *sql.Stmt
	updateOrderQty          *sql.Stmt
	updateOrderPriceQty     *sql.Stmt
	updateOpenOrderQty      *sql.Stmt
	updateOpenOrderPriceQty *sql.Stmt
	cancelOrder             *sql.Stmt
	deleteOpenOrder         *sql.Stmt
	markOrderTriggered      *sql.Stmt
	markOpenOrderTriggered  *sql.Stmt
	insertFill              *sql.Stmt
	updateOrderFilled       *sql.Stmt
	updateOpenOrderFilled   *sql.Stmt
	upsertBalance           *sql.Stmt
	upsertPosition          *sql.Stmt
	upsertFunding           *sql.Stmt
	updateFundingStatus     *sql.Stmt
	selectFundingUser       *sql.Stmt
	insertRPNL              *sql.Stmt
}

func prepareStatements(db *sql.DB) (*statements, error) {
	stmts := &statements{}
	// prepare binds statements and ensures partial cleanup on failure.
	prepare := func(dest **sql.Stmt, query string) error {
		stmt, prepErr := db.Prepare(query)
		if prepErr != nil {
			closeStatements(stmts)
			return prepErr
		}
		*dest = stmt
		return nil
	}
	if err := prepare(&stmts.upsertOrder, `
    insert into orders (id, user_id, symbol, category, origin, side, type, tif, status,
      price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, is_conditional, created_at, updated_at)
    values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    on conflict(id) do update set
      user_id=excluded.user_id,
      symbol=excluded.symbol,
      category=excluded.category,
      origin=excluded.origin,
      side=excluded.side,
      type=excluded.type,
      tif=excluded.tif,
      status=excluded.status,
      price=excluded.price,
      qty=excluded.qty,
      filled=excluded.filled,
      trigger_price=excluded.trigger_price,
      reduce_only=excluded.reduce_only,
      close_on_trigger=excluded.close_on_trigger,
      stop_order_type=excluded.stop_order_type,
      is_conditional=excluded.is_conditional,
      created_at=excluded.created_at,
      updated_at=excluded.updated_at
  `); err != nil {
		return nil, err
	}

	if err := prepare(&stmts.upsertOpenOrder, `
    insert into open_orders (id, user_id, symbol, category, origin, side, type, tif, status,
      price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, is_conditional, created_at, updated_at)
    values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    on conflict(id) do update set
      user_id=excluded.user_id,
      symbol=excluded.symbol,
      category=excluded.category,
      origin=excluded.origin,
      side=excluded.side,
      type=excluded.type,
      tif=excluded.tif,
      status=excluded.status,
      price=excluded.price,
      qty=excluded.qty,
      filled=excluded.filled,
      trigger_price=excluded.trigger_price,
      reduce_only=excluded.reduce_only,
      close_on_trigger=excluded.close_on_trigger,
      stop_order_type=excluded.stop_order_type,
      is_conditional=excluded.is_conditional,
      created_at=excluded.created_at,
      updated_at=excluded.updated_at
  `); err != nil {
		return nil, err
	}

	if err := prepare(&stmts.updateOrderQty, `update orders set qty = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.updateOrderPriceQty, `update orders set price = ?, qty = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.updateOpenOrderQty, `update open_orders set qty = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.updateOpenOrderPriceQty, `update open_orders set price = ?, qty = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.cancelOrder, `
    update orders
    set status = case
      when is_conditional = 1 then ?
      else ?
    end,
    updated_at = ?
    where id = ? and user_id = ?
  `); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.deleteOpenOrder, `delete from open_orders where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.markOrderTriggered, `update orders set status = ?, is_conditional = 0, trigger_price = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.markOpenOrderTriggered, `update open_orders set status = ?, is_conditional = 0, trigger_price = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.insertFill, `
    insert into fills (id, user_id, order_id, counterparty_order_id, symbol, category, side, role, price, qty, ts)
    values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.updateOrderFilled, `update orders set filled = ?, status = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.updateOpenOrderFilled, `update open_orders set filled = ?, status = ?, updated_at = ? where id = ? and user_id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.upsertBalance, `
    insert into balances (user_id, asset, available, locked, margin)
    values (?, ?, ?, ?, ?)
    on conflict(user_id, asset) do update set
      available=excluded.available,
      locked=excluded.locked,
      margin=excluded.margin
  `); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.upsertPosition, `
    insert into positions (user_id, symbol, size, entry_price, exit_price, mode, mm, im, liq_price, leverage, take_profit, stop_loss, tp_order_id, sl_order_id)
    values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    on conflict(user_id, symbol) do update set
      size=excluded.size,
      entry_price=excluded.entry_price,
      exit_price=excluded.exit_price,
      mode=excluded.mode,
      mm=excluded.mm,
      im=excluded.im,
      liq_price=excluded.liq_price,
      leverage=excluded.leverage,
      take_profit=excluded.take_profit,
      stop_loss=excluded.stop_loss,
      tp_order_id=excluded.tp_order_id,
      sl_order_id=excluded.sl_order_id
  `); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.upsertFunding, `
    insert into fundings (id, user_id, type, status, asset, amount, destination, created_by, message, created_at, updated_at)
    values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    on conflict(id) do update set
      user_id=excluded.user_id,
      type=excluded.type,
      status=excluded.status,
      asset=excluded.asset,
      amount=excluded.amount,
      destination=excluded.destination,
      created_by=excluded.created_by,
      message=excluded.message,
      created_at=excluded.created_at,
      updated_at=excluded.updated_at
  `); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.updateFundingStatus, `update fundings set status = ? where id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.selectFundingUser, `select user_id from fundings where id = ?`); err != nil {
		return nil, err
	}
	if err := prepare(&stmts.insertRPNL, `insert into rpnl_events (id, user_id, order_id, symbol, category, side, price, qty, realized, created_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	return stmts, nil
}

func closeStatements(stmts *statements) {
	if stmts == nil {
		return
	}
	closeStmt := func(stmt *sql.Stmt) {
		if stmt != nil {
			_ = stmt.Close()
		}
	}
	closeStmt(stmts.upsertOrder)
	closeStmt(stmts.upsertOpenOrder)
	closeStmt(stmts.updateOrderQty)
	closeStmt(stmts.updateOrderPriceQty)
	closeStmt(stmts.updateOpenOrderQty)
	closeStmt(stmts.updateOpenOrderPriceQty)
	closeStmt(stmts.cancelOrder)
	closeStmt(stmts.deleteOpenOrder)
	closeStmt(stmts.markOrderTriggered)
	closeStmt(stmts.markOpenOrderTriggered)
	closeStmt(stmts.insertFill)
	closeStmt(stmts.updateOrderFilled)
	closeStmt(stmts.updateOpenOrderFilled)
	closeStmt(stmts.upsertBalance)
	closeStmt(stmts.upsertPosition)
	closeStmt(stmts.upsertFunding)
	closeStmt(stmts.updateFundingStatus)
	closeStmt(stmts.selectFundingUser)
	closeStmt(stmts.insertRPNL)
}
