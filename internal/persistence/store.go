package persistence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/replay"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type Store struct {
	db        *sql.DB
	registry  *registry.Registry
	store     *oms.Service
	portfolio *portfolio.Service
	clearing  *clearing.Service
	replayer  *replay.Replayer
	stmts     *statements
	balances  map[types.UserID]struct{}
	positions map[positionKey]struct{}
}

type positionKey struct {
	userID types.UserID
	symbol string
}

type statements struct {
	upsertOrder            *sql.Stmt
	upsertOpenOrder        *sql.Stmt
	updateOrderQty         *sql.Stmt
	updateOpenOrderQty     *sql.Stmt
	cancelOrder            *sql.Stmt
	deleteOpenOrder        *sql.Stmt
	markOrderTriggered     *sql.Stmt
	markOpenOrderTriggered *sql.Stmt
	insertFill             *sql.Stmt
	updateOrderFilled      *sql.Stmt
	updateOpenOrderFilled  *sql.Stmt
	upsertBalance          *sql.Stmt
	upsertPosition         *sql.Stmt
	upsertFunding          *sql.Stmt
	updateFundingStatus    *sql.Stmt
	selectFundingUser      *sql.Stmt
}

type OrderRecord struct {
	ID             types.OrderID
	UserID         types.UserID
	Symbol         string
	Category       int8
	Origin         int8
	Side           int8
	Type           int8
	TIF            int8
	Status         int8
	Price          string
	Qty            string
	Filled         string
	TriggerPrice   string
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8
	IsConditional  bool
	CreatedAt      uint64
	UpdatedAt      uint64
}

type FillRecord struct {
	ID                  types.TradeID
	UserID              types.UserID
	OrderID             types.OrderID
	CounterpartyOrderID types.OrderID
	Symbol              string
	Category            int8
	Side                int8
	Role                string
	Price               string
	Qty                 string
	Timestamp           uint64
	OrderType           int8
}

type FundingRecord struct {
	ID          types.FundingID
	UserID      types.UserID
	Type        string
	Status      string
	Asset       string
	Amount      string
	Destination string
	CreatedBy   string
	Message     string
	CreatedAt   uint64
	UpdatedAt   uint64
}

func Open(dir string, reg *registry.Registry) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(dir, "trading.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open history db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping history db: %w", err)
	}

	if _, err := db.Exec("pragma journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable wal: %w", err)
	}
	if _, err := db.Exec("pragma synchronous=normal"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sync: %w", err)
	}
	if _, err := db.Exec("pragma temp_store=memory"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable temp store: %w", err)
	}

	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := refreshOpenOrders(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	stmts, err := prepareStatements(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	omsStore := oms.NewService()
	portfolioService := portfolio.New(nil, reg)
	clearingService := clearing.New(portfolioService, reg)
	replayer := replay.New(reg, omsStore, portfolioService, clearingService)
	persistenceStore := &Store{
		db:        db,
		registry:  reg,
		store:     omsStore,
		portfolio: portfolioService,
		clearing:  clearingService,
		replayer:  replayer,
		stmts:     stmts,
		balances:  make(map[types.UserID]struct{}, 1024),
		positions: make(map[positionKey]struct{}, 1024),
	}
	if err := persistenceStore.loadState(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return persistenceStore, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	closeStatements(s.stmts)
	return s.db.Close()
}

func (s *Store) LoadCore(store *oms.Service, portfolio *portfolio.Service) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.loadCore(store, portfolio)
}

func (s *Store) ListOrders(userID types.UserID, symbol string, category *int8, limit int, offset int) ([]OrderRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}

	query := `select id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, is_conditional, created_at, updated_at from orders where user_id = ?`
	args := []any{userID}
	if symbol != "" {
		query += " and symbol = ?"
		args = append(args, symbol)
	}
	if category != nil {
		query += " and category = ?"
		args = append(args, *category)
	}
	query += " order by updated_at desc"
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " offset ?"
		args = append(args, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	result := []OrderRecord{}
	for rows.Next() {
		var rec OrderRecord
		var reduceOnly int
		var closeOnTrigger int
		var isConditional int
		if err := rows.Scan(
			&rec.ID,
			&rec.UserID,
			&rec.Symbol,
			&rec.Category,
			&rec.Origin,
			&rec.Side,
			&rec.Type,
			&rec.TIF,
			&rec.Status,
			&rec.Price,
			&rec.Qty,
			&rec.Filled,
			&rec.TriggerPrice,
			&reduceOnly,
			&closeOnTrigger,
			&rec.StopOrderType,
			&isConditional,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rec.ReduceOnly = reduceOnly == 1
		rec.CloseOnTrigger = closeOnTrigger == 1
		rec.IsConditional = isConditional == 1
		result = append(result, rec)
	}
	return result, nil
}

func (s *Store) ListFills(userID types.UserID, symbol string, category *int8, limit int, offset int) ([]FillRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}

	query := `select fills.id, fills.user_id, fills.order_id, fills.counterparty_order_id, fills.symbol, fills.category, fills.side, fills.role, fills.price, fills.qty, fills.ts, orders.type from fills left join orders on fills.order_id = orders.id and fills.user_id = orders.user_id where fills.user_id = ?`
	args := []any{userID}
	if symbol != "" {
		query += " and symbol = ?"
		args = append(args, symbol)
	}
	if category != nil {
		query += " and category = ?"
		args = append(args, *category)
	}
	query += " order by ts desc"
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " offset ?"
		args = append(args, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	result := []FillRecord{}
	for rows.Next() {
		var rec FillRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.UserID,
			&rec.OrderID,
			&rec.CounterpartyOrderID,
			&rec.Symbol,
			&rec.Category,
			&rec.Side,
			&rec.Role,
			&rec.Price,
			&rec.Qty,
			&rec.Timestamp,
			&rec.OrderType,
		); err != nil {
			return nil, err
		}
		result = append(result, rec)
	}
	return result, nil
}

func (s *Store) ListFundings(userID types.UserID, limit int, offset int) ([]FundingRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := `select id, user_id, type, status, asset, amount, destination, created_by, message, created_at, updated_at from fundings where user_id = ? order by updated_at desc`
	args := []interface{}{userID}
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " offset ?"
		args = append(args, offset)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	res := make([]FundingRecord, 0)
	for rows.Next() {
		var r FundingRecord
		if err := rows.Scan(&r.ID, &r.UserID, &r.Type, &r.Status, &r.Asset, &r.Amount, &r.Destination, &r.CreatedBy, &r.Message, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

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
      is_conditional integer not null,
      created_at integer not null,
      updated_at integer not null
    );

    create index if not exists orders_user_idx on orders (user_id, updated_at);
    create index if not exists orders_symbol_idx on orders (symbol, category, updated_at);

    create table if not exists open_orders (
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
      is_conditional integer not null,
      created_at integer not null,
      updated_at integer not null
    );

    create index if not exists open_orders_user_idx on open_orders (user_id, updated_at);
    create index if not exists open_orders_symbol_idx on open_orders (symbol, category, updated_at);

    create table if not exists fills (
      id integer not null,
      user_id integer not null,
      order_id integer not null,
      counterparty_order_id integer not null,
      symbol text not null,
      category integer not null,
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
  `)
	if err != nil {
		return fmt.Errorf("init history schema: %w", err)
	}
	return nil
}

func (s *Store) Apply(eventsBatch []events.Event) error {
	if s == nil || s.db == nil || len(eventsBatch) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	if s.balances == nil {
		s.balances = make(map[types.UserID]struct{}, 1024)
	}
	if s.positions == nil {
		s.positions = make(map[positionKey]struct{}, 1024)
	}

	addBalance := func(userID types.UserID) {
		s.balances[userID] = struct{}{}
	}
	addPosition := func(userID types.UserID, symbol string) {
		s.positions[positionKey{userID: userID, symbol: symbol}] = struct{}{}
	}

	for i := range eventsBatch {
		event := eventsBatch[i]
		if s.replayer != nil {
			_ = s.replayer.ApplyEvent(event)
		}
		switch event.Type {
		case events.OrderPlaced:
			order, decErr := events.DecodeOrderPlaced(event.Data)
			if decErr != nil {
				return decErr
			}
			err = upsertOrder(tx, s.stmts, order)
			if err == nil {
				err = upsertOpenOrder(tx, s.stmts, order)
			}
			addBalance(order.UserID)
		case events.OrderAmended:
			amend, decErr := events.DecodeOrderAmended(event.Data)
			if decErr != nil {
				return decErr
			}
			err = updateOrderQty(tx, s.stmts, amend.UserID, amend.OrderID, amend.NewQty, amend.Timestamp)
			if err == nil {
				err = updateOpenOrderQty(tx, s.stmts, amend.UserID, amend.OrderID, amend.NewQty, amend.Timestamp)
			}
			addBalance(amend.UserID)
		case events.OrderCanceled:
			cancel, decErr := events.DecodeOrderCanceled(event.Data)
			if decErr != nil {
				return decErr
			}
			err = cancelOrder(tx, s.stmts, cancel.UserID, cancel.OrderID, cancel.Timestamp)
			if err == nil {
				err = deleteOpenOrder(tx, s.stmts, cancel.UserID, cancel.OrderID)
			}
			addBalance(cancel.UserID)
		case events.TradeExecuted:
			trade, decErr := events.DecodeTrade(event.Data)
			if decErr != nil {
				return decErr
			}
			err = applyTrade(tx, s.stmts, trade)
			if err == nil {
				if updErr := addFillToOpenOrder(tx, s.stmts, trade.MakerUserID, trade.MakerOrderID, trade.Quantity, trade.Timestamp); updErr != nil {
					err = updErr
				}
			}
			if err == nil {
				if updErr := addFillToOpenOrder(tx, s.stmts, trade.TakerUserID, trade.TakerOrderID, trade.Quantity, trade.Timestamp); updErr != nil {
					err = updErr
				}
			}
			addBalance(trade.MakerUserID)
			addBalance(trade.TakerUserID)
			if trade.Category == constants.CATEGORY_LINEAR {
				addPosition(trade.MakerUserID, trade.Symbol)
				addPosition(trade.TakerUserID, trade.Symbol)
			}
		case events.LeverageSet:
			lev, decErr := events.DecodeLeverage(event.Data)
			if decErr != nil {
				return decErr
			}
			addPosition(lev.UserID, lev.Symbol)
		case events.FundingCreated:
			req, decErr := events.DecodeFundingCreated(event.Data)
			if decErr != nil {
				return decErr
			}
			err = upsertFunding(tx, s.stmts, req)
			addBalance(req.UserID)
		case events.FundingApproved, events.FundingRejected:
			evt, decErr := events.DecodeFundingStatus(event.Data)
			if decErr != nil {
				return decErr
			}
			status := types.FundingStatusCanceled
			if event.Type == events.FundingApproved {
				status = types.FundingStatusCompleted
			}
			err = updateFundingStatus(tx, s.stmts, evt.FundingID, status)
			if userID, ok := selectFundingUser(tx, s.stmts, evt.FundingID); ok {
				addBalance(userID)
			} else {
				return fmt.Errorf("missing funding %d", evt.FundingID)
			}
		case events.OrderTriggered:
			evt, decErr := events.DecodeOrderTriggered(event.Data)
			if decErr != nil {
				return decErr
			}
			err = markOrderTriggered(tx, s.stmts, evt.UserID, evt.OrderID, evt.Timestamp)
			if err == nil {
				err = markOpenOrderTriggered(tx, s.stmts, evt.UserID, evt.OrderID, evt.Timestamp)
			}
			addBalance(evt.UserID)
		}
		if err != nil {
			return err
		}
	}

	for userID := range s.balances {
		balances := s.portfolio.GetBalances(userID)
		for i := range balances {
			if bal := balances[i]; bal != nil {
				err = upsertBalance(tx, s.stmts, bal)
				if err != nil {
					return err
				}
			}
		}
		delete(s.balances, userID)
	}

	for key := range s.positions {
		pos := s.portfolio.GetPosition(key.userID, key.symbol)
		if pos == nil {
			delete(s.positions, key)
			continue
		}
		err = upsertPosition(tx, s.stmts, pos)
		if err != nil {
			return err
		}
		delete(s.positions, key)
	}

	return err
}

func (s *Store) loadState() error {
	return s.loadCore(s.store, s.portfolio)
}

func (s *Store) loadCore(store *oms.Service, portfolio *portfolio.Service) error {
	if s == nil || s.db == nil {
		return nil
	}
	if store == nil || portfolio == nil {
		return nil
	}
	if err := loadBalances(s.db, portfolio); err != nil {
		return err
	}
	if err := loadPositions(s.db, portfolio); err != nil {
		return err
	}
	if err := loadOpenOrders(s.db, store); err != nil {
		return err
	}
	return nil
}

func upsertOrder(tx *sql.Tx, stmts *statements, order *types.Order) error {
	if order == nil {
		return nil
	}
	stmt := stmts.upsertOrder
	if stmt == nil {
		return fmt.Errorf("missing upsert order statement")
	}
	_, err := tx.Stmt(stmt).Exec(
		order.ID,
		order.UserID,
		order.Symbol,
		order.Category,
		order.Origin,
		order.Side,
		order.Type,
		order.TIF,
		order.Status,
		order.Price.String(),
		order.Quantity.String(),
		order.Filled.String(),
		order.TriggerPrice.String(),
		boolToInt(order.ReduceOnly),
		boolToInt(order.CloseOnTrigger),
		order.StopOrderType,
		boolToInt(order.IsConditional),
		order.CreatedAt,
		order.UpdatedAt,
	)
	return err
}

func upsertOpenOrder(tx *sql.Tx, stmts *statements, order *types.Order) error {
	if order == nil || !isOpenStatus(order.Status) {
		return nil
	}
	stmt := stmts.upsertOpenOrder
	if stmt == nil {
		return fmt.Errorf("missing upsert open order statement")
	}
	_, err := tx.Stmt(stmt).Exec(
		order.ID,
		order.UserID,
		order.Symbol,
		order.Category,
		order.Origin,
		order.Side,
		order.Type,
		order.TIF,
		order.Status,
		order.Price.String(),
		order.Quantity.String(),
		order.Filled.String(),
		order.TriggerPrice.String(),
		boolToInt(order.ReduceOnly),
		boolToInt(order.CloseOnTrigger),
		order.StopOrderType,
		boolToInt(order.IsConditional),
		order.CreatedAt,
		order.UpdatedAt,
	)
	return err
}

func updateOpenOrderQty(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, qty types.Quantity, ts uint64) error {
	stmt := stmts.updateOpenOrderQty
	if stmt == nil {
		return fmt.Errorf("missing update open order qty statement")
	}
	_, err := tx.Stmt(stmt).Exec(qty.String(), ts, orderID, userID)
	return err
}

func deleteOpenOrder(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID) error {
	stmt := stmts.deleteOpenOrder
	if stmt == nil {
		return fmt.Errorf("missing delete open order statement")
	}
	_, err := tx.Stmt(stmt).Exec(orderID, userID)
	return err
}

func markOpenOrderTriggered(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, ts uint64) error {
	stmt := stmts.markOpenOrderTriggered
	if stmt == nil {
		return fmt.Errorf("missing mark open order triggered statement")
	}
	_, err := tx.Stmt(stmt).Exec(constants.ORDER_STATUS_TRIGGERED, types.Price{}.String(), ts, orderID, userID)
	return err
}

func addFillToOpenOrder(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, qty types.Quantity, ts uint64) error {
	row := tx.QueryRow(`select filled, qty, status from open_orders where id = ? and user_id = ?`, orderID, userID)
	var filledStr, qtyStr string
	var status int8
	if err := row.Scan(&filledStr, &qtyStr, &status); err != nil {
		return err
	}
	if !isOpenStatus(status) {
		return fmt.Errorf("open order %d not active", orderID)
	}
	filled := parseFixed(filledStr)
	newFilled := math.Add(filled, qty)
	status = constants.ORDER_STATUS_PARTIALLY_FILLED
	if math.Cmp(newFilled, parseFixed(qtyStr)) >= 0 {
		status = constants.ORDER_STATUS_FILLED
	}
	if status == constants.ORDER_STATUS_FILLED {
		return deleteOpenOrder(tx, stmts, userID, orderID)
	}
	stmt := stmts.updateOpenOrderFilled
	if stmt == nil {
		return fmt.Errorf("missing update open order filled statement")
	}
	_, err := tx.Stmt(stmt).Exec(newFilled.String(), status, ts, orderID, userID)
	return err
}

func isOpenStatus(status int8) bool {
	return status == constants.ORDER_STATUS_NEW ||
		status == constants.ORDER_STATUS_PARTIALLY_FILLED ||
		status == constants.ORDER_STATUS_UNTRIGGERED ||
		status == constants.ORDER_STATUS_TRIGGERED
}

func updateOrderQty(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, qty types.Quantity, ts uint64) error {
	stmt := stmts.updateOrderQty
	if stmt == nil {
		return fmt.Errorf("missing update order qty statement")
	}
	_, err := tx.Stmt(stmt).Exec(qty.String(), ts, orderID, userID)
	return err
}

func cancelOrder(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, ts uint64) error {
	stmt := stmts.cancelOrder
	if stmt == nil {
		return fmt.Errorf("missing cancel order statement")
	}
	_, err := tx.Stmt(stmt).Exec(
		constants.ORDER_STATUS_DEACTIVATED,
		constants.ORDER_STATUS_CANCELED,
		ts,
		orderID,
		userID,
	)
	return err
}

func applyTrade(tx *sql.Tx, stmts *statements, trade events.TradeEvent) error {
	price := trade.Price.String()
	qty := trade.Quantity.String()
	makerSide := oppositeSide(trade.TakerSide)

	if err := insertFill(tx, stmts, trade.TradeID, trade.MakerUserID, trade.MakerOrderID, trade.TakerOrderID, trade.Symbol, trade.Category, makerSide, "MAKER", price, qty, trade.Timestamp); err != nil {
		return err
	}
	if err := insertFill(tx, stmts, trade.TradeID, trade.TakerUserID, trade.TakerOrderID, trade.MakerOrderID, trade.Symbol, trade.Category, trade.TakerSide, "TAKER", price, qty, trade.Timestamp); err != nil {
		return err
	}

	if err := addFillToOrder(tx, stmts, trade.MakerUserID, trade.MakerOrderID, trade.Quantity, trade.Timestamp); err != nil {
		return err
	}
	if err := addFillToOrder(tx, stmts, trade.TakerUserID, trade.TakerOrderID, trade.Quantity, trade.Timestamp); err != nil {
		return err
	}
	return nil
}

func insertFill(tx *sql.Tx, stmts *statements, id types.TradeID, userID types.UserID, orderID types.OrderID, counterparty types.OrderID, symbol string, category int8, side int8, role string, price string, qty string, ts uint64) error {
	stmt := stmts.insertFill
	if stmt == nil {
		return fmt.Errorf("missing insert fill statement")
	}
	_, err := tx.Stmt(stmt).Exec(id, userID, orderID, counterparty, symbol, category, side, role, price, qty, ts)
	return err
}

func addFillToOrder(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, qty types.Quantity, ts uint64) error {
	row := tx.QueryRow(`select filled, qty, status from orders where id = ? and user_id = ?`, orderID, userID)
	var filledStr, qtyStr string
	var status int8
	if err := row.Scan(&filledStr, &qtyStr, &status); err != nil {
		return err
	}
	if status == constants.ORDER_STATUS_CANCELED || status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED || status == constants.ORDER_STATUS_DEACTIVATED {
		return fmt.Errorf("order %d not active", orderID)
	}
	filled := parseFixed(filledStr)
	newFilled := math.Add(filled, qty)
	status = constants.ORDER_STATUS_PARTIALLY_FILLED
	if math.Cmp(newFilled, parseFixed(qtyStr)) >= 0 {
		status = constants.ORDER_STATUS_FILLED
	}
	stmt := stmts.updateOrderFilled
	if stmt == nil {
		return fmt.Errorf("missing update order filled statement")
	}
	_, err := tx.Stmt(stmt).Exec(newFilled.String(), status, ts, orderID, userID)
	return err
}

func markOrderTriggered(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, ts uint64) error {
	stmt := stmts.markOrderTriggered
	if stmt == nil {
		return fmt.Errorf("missing mark order triggered statement")
	}
	_, err := tx.Stmt(stmt).Exec(constants.ORDER_STATUS_TRIGGERED, types.Price{}.String(), ts, orderID, userID)
	return err
}

func upsertBalance(tx *sql.Tx, stmts *statements, bal *types.Balance) error {
	if bal == nil {
		return nil
	}
	stmt := stmts.upsertBalance
	if stmt == nil {
		return fmt.Errorf("missing upsert balance statement")
	}
	_, err := tx.Stmt(stmt).Exec(bal.UserID, bal.Asset, bal.Available.String(), bal.Locked.String(), bal.Margin.String())
	return err
}

func upsertPosition(tx *sql.Tx, stmts *statements, pos *types.Position) error {
	if pos == nil {
		return nil
	}
	stmt := stmts.upsertPosition
	if stmt == nil {
		return fmt.Errorf("missing upsert position statement")
	}
	_, err := tx.Stmt(stmt).Exec(
		pos.UserID,
		pos.Symbol,
		pos.Size.String(),
		pos.EntryPrice.String(),
		pos.ExitPrice.String(),
		pos.Mode,
		pos.MM.String(),
		pos.IM.String(),
		pos.LiqPrice.String(),
		pos.Leverage.String(),
		pos.TakeProfit.String(),
		pos.StopLoss.String(),
		pos.TPOrderID,
		pos.SLOrderID,
	)
	return err
}

func upsertFunding(tx *sql.Tx, stmts *statements, req *types.FundingRequest) error {
	if req == nil {
		return nil
	}
	stmt := stmts.upsertFunding
	if stmt == nil {
		return fmt.Errorf("missing upsert funding statement")
	}
	_, err := tx.Stmt(stmt).Exec(
		req.ID,
		req.UserID,
		req.Type,
		req.Status,
		req.Asset,
		req.Amount.String(),
		req.Destination,
		req.CreatedBy,
		req.Message,
		req.CreatedAt,
		req.UpdatedAt,
	)
	return err
}

func updateFundingStatus(tx *sql.Tx, stmts *statements, id types.FundingID, status types.FundingStatus) error {
	stmt := stmts.updateFundingStatus
	if stmt == nil {
		return fmt.Errorf("missing update funding status statement")
	}
	_, err := tx.Stmt(stmt).Exec(status, id)
	return err
}

func selectFundingUser(tx *sql.Tx, stmts *statements, id types.FundingID) (types.UserID, bool) {
	stmt := stmts.selectFundingUser
	if stmt == nil {
		return 0, false
	}
	row := tx.Stmt(stmt).QueryRow(id)
	var userID types.UserID
	if err := row.Scan(&userID); err != nil {
		return 0, false
	}
	return userID, true
}

func loadBalances(db *sql.DB, portfolio *portfolio.Service) error {
	rows, err := db.Query(`select user_id, asset, available, locked, margin from balances`)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var userID types.UserID
		var asset, available, locked, margin string
		if err := rows.Scan(&userID, &asset, &available, &locked, &margin); err != nil {
			return err
		}
		portfolio.LoadBalance(&types.Balance{
			UserID:    userID,
			Asset:     asset,
			Available: parseFixed(available),
			Locked:    parseFixed(locked),
			Margin:    parseFixed(margin),
		})
	}
	return nil
}

func loadPositions(db *sql.DB, portfolio *portfolio.Service) error {
	rows, err := db.Query(`select user_id, symbol, size, entry_price, exit_price, mode, mm, im, liq_price, leverage, take_profit, stop_loss, tp_order_id, sl_order_id from positions`)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var userID types.UserID
		var symbol string
		var size, entryPrice, exitPrice, mm, im, liqPrice, leverage, takeProfit, stopLoss string
		var mode types.PositionMode
		var tpOrderID types.OrderID
		var slOrderID types.OrderID
		if err := rows.Scan(&userID, &symbol, &size, &entryPrice, &exitPrice, &mode, &mm, &im, &liqPrice, &leverage, &takeProfit, &stopLoss, &tpOrderID, &slOrderID); err != nil {
			return err
		}
		portfolio.LoadPosition(&types.Position{
			UserID:     userID,
			Symbol:     symbol,
			Size:       parseFixed(size),
			EntryPrice: types.Price(parseFixed(entryPrice)),
			ExitPrice:  types.Price(parseFixed(exitPrice)),
			Mode:       mode,
			MM:         parseFixed(mm),
			IM:         parseFixed(im),
			LiqPrice:   types.Price(parseFixed(liqPrice)),
			Leverage:   types.Leverage(parseFixed(leverage)),
			TakeProfit: types.Price(parseFixed(takeProfit)),
			StopLoss:   types.Price(parseFixed(stopLoss)),
			TPOrderID:  tpOrderID,
			SLOrderID:  slOrderID,
		})
	}
	return nil
}

func loadOpenOrders(db *sql.DB, store *oms.Service) error {
	rows, err := db.Query(
		`select id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, is_conditional, created_at, updated_at
     from open_orders`,
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var order types.Order
		var price, qty, filled, triggerPrice string
		var reduceOnly, closeOnTrigger, isConditional int
		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Symbol,
			&order.Category,
			&order.Origin,
			&order.Side,
			&order.Type,
			&order.TIF,
			&order.Status,
			&price,
			&qty,
			&filled,
			&triggerPrice,
			&reduceOnly,
			&closeOnTrigger,
			&order.StopOrderType,
			&isConditional,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return err
		}
		order.Price = types.Price(parseFixed(price))
		order.Quantity = parseFixed(qty)
		order.Filled = parseFixed(filled)
		order.TriggerPrice = types.Price(parseFixed(triggerPrice))
		order.ReduceOnly = reduceOnly == 1
		order.CloseOnTrigger = closeOnTrigger == 1
		order.IsConditional = isConditional == 1
		store.Load(&order)
	}
	return nil
}

func refreshOpenOrders(db *sql.DB) error {
	_, err := db.Exec(`
    delete from open_orders;
    insert into open_orders (id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, is_conditional, created_at, updated_at)
    select id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, is_conditional, created_at, updated_at
    from orders
    where status in (?, ?, ?, ?);
  `,
		constants.ORDER_STATUS_NEW,
		constants.ORDER_STATUS_PARTIALLY_FILLED,
		constants.ORDER_STATUS_UNTRIGGERED,
		constants.ORDER_STATUS_TRIGGERED,
	)
	return err
}

func parseFixed(value string) types.Quantity {
	if value == "" {
		return types.Quantity{}
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return types.Quantity{}
	}
	return types.Quantity(fixed.NewF(parsed))
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func prepareStatements(db *sql.DB) (*statements, error) {
	stmts := &statements{}
	var err error
	stmts.upsertOrder, err = db.Prepare(`
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
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}

	stmts.upsertOpenOrder, err = db.Prepare(`
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
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}

	stmts.updateOrderQty, err = db.Prepare(`update orders set qty = ?, updated_at = ? where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.updateOpenOrderQty, err = db.Prepare(`update open_orders set qty = ?, updated_at = ? where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.cancelOrder, err = db.Prepare(`
    update orders
    set status = case
      when is_conditional = 1 then ?
      else ?
    end,
    updated_at = ?
    where id = ? and user_id = ?
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.deleteOpenOrder, err = db.Prepare(`delete from open_orders where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.markOrderTriggered, err = db.Prepare(`update orders set status = ?, is_conditional = 0, trigger_price = ?, updated_at = ? where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.markOpenOrderTriggered, err = db.Prepare(`update open_orders set status = ?, is_conditional = 0, trigger_price = ?, updated_at = ? where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.insertFill, err = db.Prepare(`
    insert into fills (id, user_id, order_id, counterparty_order_id, symbol, category, side, role, price, qty, ts)
    values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.updateOrderFilled, err = db.Prepare(`update orders set filled = ?, status = ?, updated_at = ? where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.updateOpenOrderFilled, err = db.Prepare(`update open_orders set filled = ?, status = ?, updated_at = ? where id = ? and user_id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.upsertBalance, err = db.Prepare(`
    insert into balances (user_id, asset, available, locked, margin)
    values (?, ?, ?, ?, ?)
    on conflict(user_id, asset) do update set
      available=excluded.available,
      locked=excluded.locked,
      margin=excluded.margin
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.upsertPosition, err = db.Prepare(`
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
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.upsertFunding, err = db.Prepare(`
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
  `)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.updateFundingStatus, err = db.Prepare(`update fundings set status = ? where id = ?`)
	if err != nil {
		closeStatements(stmts)
		return nil, err
	}
	stmts.selectFundingUser, err = db.Prepare(`select user_id from fundings where id = ?`)
	if err != nil {
		closeStatements(stmts)
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
	closeStmt(stmts.updateOpenOrderQty)
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
}

func oppositeSide(side int8) int8 {
	if side == constants.ORDER_SIDE_BUY {
		return constants.ORDER_SIDE_SELL
	}
	return constants.ORDER_SIDE_BUY
}
