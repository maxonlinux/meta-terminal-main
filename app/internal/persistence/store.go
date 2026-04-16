package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/replay"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type Store struct {
	db               *sql.DB
	registry         *registry.Registry
	store            *oms.Service
	portfolio        *portfolio.Service
	clearing         *clearing.Service
	replayer         *replay.Replayer
	stmts            *statements
	balances         map[types.UserID]struct{}
	positions        map[positionKey]struct{}
	orderFills       map[orderKey]orderFillAccum
	orderMutations   map[orderKey]orderMutation
	tradeInstruments map[string]tradeInstrumentCacheEntry
}

type txStatements struct {
	upsertOrder         *sql.Stmt
	updateOrderQty      *sql.Stmt
	updateOrderPriceQty *sql.Stmt
	cancelOrder         *sql.Stmt
	markOrderTriggered  *sql.Stmt
	insertFill          *sql.Stmt
	updateOrderFilled   *sql.Stmt
	upsertBalance       *sql.Stmt
	upsertPosition      *sql.Stmt
	upsertFunding       *sql.Stmt
	updateFundingStatus *sql.Stmt
	selectFundingUser   *sql.Stmt
	insertRPNL          *sql.Stmt
}

func bindTxStatements(tx *sql.Tx, stmts *statements) *txStatements {
	if stmts == nil {
		return &txStatements{}
	}
	bind := func(stmt *sql.Stmt) *sql.Stmt {
		if tx == nil || stmt == nil {
			return nil
		}
		return tx.Stmt(stmt)
	}
	return &txStatements{
		upsertOrder:         bind(stmts.upsertOrder),
		updateOrderQty:      bind(stmts.updateOrderQty),
		updateOrderPriceQty: bind(stmts.updateOrderPriceQty),
		cancelOrder:         bind(stmts.cancelOrder),
		markOrderTriggered:  bind(stmts.markOrderTriggered),
		insertFill:          bind(stmts.insertFill),
		updateOrderFilled:   bind(stmts.updateOrderFilled),
		upsertBalance:       bind(stmts.upsertBalance),
		upsertPosition:      bind(stmts.upsertPosition),
		upsertFunding:       bind(stmts.upsertFunding),
		updateFundingStatus: bind(stmts.updateFundingStatus),
		selectFundingUser:   bind(stmts.selectFundingUser),
		insertRPNL:          bind(stmts.insertRPNL),
	}
}

// DB returns the underlying database handle for feature repositories.
func (s *Store) DB() *sql.DB {
	return s.db
}

// IntegrityCheck runs a guarded SQLite integrity check.
// Do not run integrity_check from the host against a hot database file; use this method
// or stop the container first to avoid false errors from concurrent writes/journal state.
// It acquires an immediate transaction to avoid running on a hot writer.
func (s *Store) IntegrityCheck(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", nil
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, "begin immediate"); err != nil {
		return "", err
	}
	defer func() {
		_, _ = conn.ExecContext(ctx, "rollback")
	}()
	row := conn.QueryRowContext(ctx, "pragma integrity_check")
	var res string
	if err := row.Scan(&res); err != nil {
		return "", err
	}
	return res, nil
}

// CleanupSystemOrders removes closed system-origin orders older than cutoff.
func (s *Store) CleanupSystemOrders(cutoff uint64) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	res, err := s.db.Exec(
		`delete from orders where origin = ? and status in (?, ?, ?, ?) and updated_at <= ?`,
		constants.ORDER_ORIGIN_SYSTEM,
		constants.ORDER_STATUS_FILLED,
		constants.ORDER_STATUS_CANCELED,
		constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED,
		constants.ORDER_STATUS_DEACTIVATED,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CleanupBotData removes all data for a specific bot user ID across all tables.
func (s *Store) CleanupBotData(botUserID types.UserID, cutoff uint64) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var totalCount int64

	tables := []string{
		"orders",
		"fills",
		"positions",
		"rpnl_events",
		"balances",
	}

	for _, table := range tables {
		var res sql.Result
		var err error

		switch table {
		case "orders":
			res, err = s.db.Exec(
				`delete from orders where user_id = ? and status in (?, ?, ?, ?) and updated_at <= ?`,
				botUserID,
				constants.ORDER_STATUS_FILLED,
				constants.ORDER_STATUS_CANCELED,
				constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED,
				constants.ORDER_STATUS_DEACTIVATED,
				cutoff,
			)
		case "fills":
			res, err = s.db.Exec(`delete from fills where user_id = ?`, botUserID)
		case "positions":
			res, err = s.db.Exec(`delete from positions where user_id = ?`, botUserID)
		case "rpnl_events":
			res, err = s.db.Exec(`delete from rpnl_events where user_id = ?`, botUserID)
		case "balances":
			res, err = s.db.Exec(`delete from balances where user_id = ?`, botUserID)
		}

		if err != nil {
			return totalCount, fmt.Errorf("cleanup %s: %w", table, err)
		}
		count, err := res.RowsAffected()
		if err != nil {
			return totalCount, err
		}
		totalCount += count
	}

	return totalCount, nil
}

type positionKey struct {
	userID types.UserID
	symbol string
}

type orderKey struct {
	userID  types.UserID
	orderID types.OrderID
}

type orderFillAccum struct {
	filled types.Quantity
	qty    types.Quantity
	ts     uint64
}

type orderMutationKind uint8

const (
	orderMutationAmend orderMutationKind = iota + 1
	orderMutationCancel
	orderMutationTrigger
)

type orderMutation struct {
	kind      orderMutationKind
	price     types.Price
	qty       types.Quantity
	timestamp uint64
}

type tradeInstrumentCacheEntry struct {
	payload []byte
	inst    *types.Instrument
}

type OrderRecord struct {
	ID               types.OrderID
	UserID           types.UserID
	Symbol           string
	Category         int8
	Origin           int8
	Side             int8
	Type             int8
	TIF              int8
	Status           int8
	Price            string
	Qty              string
	Filled           string
	TriggerPrice     string
	ReduceOnly       bool
	CloseOnTrigger   bool
	StopOrderType    int8
	TriggerDirection int8
	IsConditional    bool
	CreatedAt        uint64
	UpdatedAt        uint64
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

type RPNLRecord struct {
	// RPNLRecord stores a realized PnL entry in history.
	ID        int64
	UserID    types.UserID
	OrderID   types.OrderID
	Symbol    string
	Category  int8
	Side      int8
	Price     string
	Quantity  string
	Realized  string
	CreatedAt uint64
}

func Open(dir string, reg *registry.Registry) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(dir, "trading.db")
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open history db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping history db: %w", err)
	}
	if _, err := db.Exec("pragma busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := db.Exec("pragma journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable journal mode: %w", err)
	}
	if _, err := db.Exec("pragma synchronous=NORMAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sync: %w", err)
	}
	if _, err := db.Exec("pragma wal_autocheckpoint=2000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set wal autocheckpoint: %w", err)
	}
	if _, err := db.Exec("pragma cache_size=-131072"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set cache size: %w", err)
	}
	if _, err := db.Exec("pragma temp_store=memory"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable temp store: %w", err)
	}

	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	stmts, err := prepareStatements(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	omsStore := oms.NewService()
	portfolioService, err := portfolio.New(nil, reg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	clearingService, err := clearing.New(portfolioService, reg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
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
	closeStatements(s.stmts)
	return s.db.Close()
}

func (s *Store) LoadCore(store *oms.Service, portfolio *portfolio.Service) error {
	return s.loadCore(store, portfolio)
}

func (s *Store) ListOrders(userID types.UserID, symbol string, category *int8, limit int, offset int) ([]OrderRecord, error) {
	query := `select id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, trigger_direction, is_conditional, created_at, updated_at from orders where user_id = ?`
	args := []any{userID}
	if symbol != "" {
		query += " and symbol = ?"
		args = append(args, symbol)
	}
	if category != nil {
		query += " and category = ?"
		args = append(args, *category)
	}
	query += " and status in (?, ?, ?, ?, ?)"
	args = append(args,
		constants.ORDER_STATUS_FILLED,
		constants.ORDER_STATUS_CANCELED,
		constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED,
		constants.ORDER_STATUS_TRIGGERED,
		constants.ORDER_STATUS_DEACTIVATED,
	)
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
			&rec.TriggerDirection,
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
	query := `select id, user_id, order_id, counterparty_order_id, symbol, category, order_type, side, role, price, qty, ts from fills where user_id = ?`
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
			&rec.OrderType,
			&rec.Side,
			&rec.Role,
			&rec.Price,
			&rec.Qty,
			&rec.Timestamp,
		); err != nil {
			return nil, err
		}
		result = append(result, rec)
	}
	return result, nil
}

func (s *Store) ListRPNL(userID types.UserID, symbol string, category *int8, limit int, offset int) ([]RPNLRecord, error) {
	query := `select id, user_id, order_id, symbol, category, side, price, qty, realized, created_at from rpnl_events where user_id = ?`
	args := []any{userID}
	if symbol != "" {
		query += " and symbol = ?"
		args = append(args, symbol)
	}
	if category != nil {
		query += " and category = ?"
		args = append(args, *category)
	}
	query += " order by created_at desc"
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
	capHint := 0
	if limit > 0 {
		capHint = limit
	}
	res := make([]RPNLRecord, 0, capHint)
	for rows.Next() {
		var rec RPNLRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.UserID,
			&rec.OrderID,
			&rec.Symbol,
			&rec.Category,
			&rec.Side,
			&rec.Price,
			&rec.Quantity,
			&rec.Realized,
			&rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, rec)
	}
	return res, nil
}

func (s *Store) ListFundings(userID types.UserID, limit int, offset int) ([]FundingRecord, error) {
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

func (s *Store) GetPendingFundings() ([]FundingRecord, error) {
	query := `select id, user_id, type, status, asset, amount, destination, created_by, message, created_at, updated_at from fundings where status = 'PENDING'`
	rows, err := s.db.Query(query)
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

func (s *Store) ListFundingsAll(limit int, offset int, search string) ([]FundingRecord, error) {
	query := `select id, user_id, type, status, asset, amount, destination, created_by, message, created_at, updated_at from fundings`
	args := []interface{}{}
	if search != "" {
		query += " where (lower(destination) like ? or lower(message) like ? or cast(id as text) like ? or cast(user_id as text) like ?)"
		pattern := "%" + strings.ToLower(search) + "%"
		args = append(args, pattern, pattern, pattern, pattern)
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
	capHint := 0
	if limit > 0 {
		capHint = limit
	}
	res := make([]FundingRecord, 0, capHint)
	for rows.Next() {
		var r FundingRecord
		if err := rows.Scan(&r.ID, &r.UserID, &r.Type, &r.Status, &r.Asset, &r.Amount, &r.Destination, &r.CreatedBy, &r.Message, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

func (s *Store) CountPendingFundings() (int, error) {
	row := s.db.QueryRow("select count(1) from fundings where status = ?", string(types.FundingStatusPending))
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) resolveTradeInstrument(symbol string, payload []byte) (*types.Instrument, error) {
	if cached, ok := s.tradeInstruments[symbol]; ok && bytes.Equal(cached.payload, payload) {
		return cached.inst, nil
	}
	inst, err := events.DecodeInstrument(payload)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("missing trade instrument for %s", symbol)
	}
	s.tradeInstruments[symbol] = tradeInstrumentCacheEntry{payload: payload, inst: inst}
	return inst, nil
}

func (s *Store) resetApplyState() {
	if s.balances == nil {
		s.balances = make(map[types.UserID]struct{})
	} else {
		clear(s.balances)
	}
	if s.positions == nil {
		s.positions = make(map[positionKey]struct{})
	} else {
		clear(s.positions)
	}
	if s.orderFills == nil {
		s.orderFills = make(map[orderKey]orderFillAccum)
	} else {
		clear(s.orderFills)
	}
	if s.orderMutations == nil {
		s.orderMutations = make(map[orderKey]orderMutation)
	} else {
		clear(s.orderMutations)
	}
	if s.tradeInstruments == nil {
		s.tradeInstruments = make(map[string]tradeInstrumentCacheEntry)
	} else {
		clear(s.tradeInstruments)
	}
}

func (s *Store) addBalance(userID types.UserID) {
	s.balances[userID] = struct{}{}
}

func (s *Store) addPosition(userID types.UserID, symbol string) {
	s.positions[positionKey{userID: userID, symbol: symbol}] = struct{}{}
}

func (s *Store) accumOrderFill(order *types.Order, qty types.Quantity, ts uint64) {
	key := orderKey{userID: order.UserID, orderID: order.ID}
	accum, ok := s.orderFills[key]
	if !ok {
		accum.qty = order.Quantity
		accum.filled = order.Filled
	}
	accum.filled = math.Add(accum.filled, qty)
	if ts > accum.ts {
		accum.ts = ts
	}
	s.orderFills[key] = accum
}

func (s *Store) flushOrderFills(stmts *txStatements) error {
	if len(s.orderFills) == 0 {
		return nil
	}
	for key, accum := range s.orderFills {
		if err := upsertOrderFill(stmts, key.userID, key.orderID, accum.filled, accum.qty, accum.ts); err != nil {
			return err
		}
	}
	clear(s.orderFills)
	return nil
}

func (s *Store) scheduleOrderMutation(key orderKey, mutation orderMutation) {
	current, ok := s.orderMutations[key]
	if !ok {
		s.orderMutations[key] = mutation
		return
	}
	if mutation.kind == orderMutationAmend && current.kind == orderMutationAmend {
		if math.Sign(mutation.price) > 0 {
			current.price = mutation.price
		}
		current.qty = mutation.qty
		if mutation.timestamp > current.timestamp {
			current.timestamp = mutation.timestamp
		}
		s.orderMutations[key] = current
		return
	}
	s.orderMutations[key] = mutation
}

func (s *Store) flushOrderMutations(stmts *txStatements) error {
	if len(s.orderMutations) == 0 {
		return nil
	}
	for key, mutation := range s.orderMutations {
		switch mutation.kind {
		case orderMutationAmend:
			if math.Sign(mutation.price) > 0 {
				if err := updateOrderPriceQty(stmts, key.userID, key.orderID, mutation.price, mutation.qty, mutation.timestamp); err != nil {
					return err
				}
			} else {
				if err := updateOrderQty(stmts, key.userID, key.orderID, mutation.qty, mutation.timestamp); err != nil {
					return err
				}
			}
		case orderMutationCancel:
			if err := cancelOrder(stmts, key.userID, key.orderID, mutation.timestamp); err != nil {
				return err
			}
		case orderMutationTrigger:
			if err := markOrderTriggered(stmts, key.userID, key.orderID, mutation.timestamp); err != nil {
				return err
			}
		}
	}
	clear(s.orderMutations)
	return nil
}

func (s *Store) Apply(eventsBatch []events.Event) error {
	if len(eventsBatch) == 0 {
		return nil
	}
	s.resetApplyState()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	txStmts := bindTxStatements(tx, s.stmts)
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			_ = s.loadState()
		} else {
			_ = tx.Commit()
		}
	}()

	for i := range eventsBatch {
		event := eventsBatch[i]
		if event.Type == events.TradeExecuted {
			if err := s.flushOrderMutations(txStmts); err != nil {
				return err
			}
		}
		if event.Type != events.TradeExecuted {
			if err := s.flushOrderFills(txStmts); err != nil {
				return err
			}
		}
		switch event.Type {
		case events.OrderPlaced:
			placed, decErr := events.DecodeOrderPlaced(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyOrderPlaced(placed); err != nil {
				return err
			}
			err = upsertOrder(txStmts, placed.Order)
			s.addBalance(placed.Order.UserID)
		case events.OrderAmended:
			amend, decErr := events.DecodeOrderAmended(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyOrderAmended(amend); err != nil {
				return err
			}
			s.scheduleOrderMutation(orderKey{userID: amend.UserID, orderID: amend.OrderID}, orderMutation{kind: orderMutationAmend, price: amend.NewPrice, qty: amend.NewQty, timestamp: amend.Timestamp})
			s.addBalance(amend.UserID)
		case events.OrderCanceled:
			cancel, decErr := events.DecodeOrderCanceled(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyOrderCanceled(cancel); err != nil {
				return err
			}
			s.scheduleOrderMutation(orderKey{userID: cancel.UserID, orderID: cancel.OrderID}, orderMutation{kind: orderMutationCancel, timestamp: cancel.Timestamp})
			s.addBalance(cancel.UserID)
		case events.TradeExecuted:
			trade, instrumentPayload, decErr := events.DecodeTradeNoSymbolWithPayload(event.Data)
			if decErr != nil {
				return decErr
			}
			makerOrder, ok := s.store.GetUserOrder(trade.MakerUserID, trade.MakerOrderID)
			if !ok || makerOrder == nil {
				return fmt.Errorf("maker order %d for user %d not found", trade.MakerOrderID, trade.MakerUserID)
			}
			takerOrder, ok := s.store.GetUserOrder(trade.TakerUserID, trade.TakerOrderID)
			if !ok || takerOrder == nil {
				return fmt.Errorf("taker order %d for user %d not found", trade.TakerOrderID, trade.TakerUserID)
			}
			if makerOrder.Symbol != takerOrder.Symbol {
				return fmt.Errorf("trade %d symbol mismatch between maker and taker orders", trade.TradeID)
			}
			trade.Symbol = makerOrder.Symbol
			inst, instErr := s.resolveTradeInstrument(trade.Symbol, instrumentPayload)
			if instErr != nil {
				return instErr
			}
			trade.Instrument = inst
			s.accumOrderFill(makerOrder, trade.Quantity, trade.Timestamp)
			s.accumOrderFill(takerOrder, trade.Quantity, trade.Timestamp)
			if err := s.replayer.ApplyTradeExecutedWithOrders(trade, makerOrder, takerOrder); err != nil {
				return err
			}
			price := trade.Price.String()
			qty := trade.Quantity.String()
			makerSide := oppositeSide(trade.TakerSide)
			if err = insertFill(txStmts, trade.TradeID, trade.MakerUserID, trade.MakerOrderID, trade.TakerOrderID, trade.Symbol, trade.Category, trade.MakerOrderType, makerSide, "MAKER", price, qty, trade.Timestamp); err != nil {
				return err
			}
			if err = insertFill(txStmts, trade.TradeID, trade.TakerUserID, trade.TakerOrderID, trade.MakerOrderID, trade.Symbol, trade.Category, trade.TakerOrderType, trade.TakerSide, "TAKER", price, qty, trade.Timestamp); err != nil {
				return err
			}
			s.addBalance(trade.MakerUserID)
			s.addBalance(trade.TakerUserID)
			if trade.Category == constants.CATEGORY_LINEAR {
				s.addPosition(trade.MakerUserID, trade.Symbol)
				s.addPosition(trade.TakerUserID, trade.Symbol)
			}
		case events.LeverageSet:
			lev, decErr := events.DecodeLeverage(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyLeverageSet(lev); err != nil {
				return err
			}
			s.addPosition(lev.UserID, lev.Symbol)
		case events.FundingCreated:
			req, decErr := events.DecodeFundingCreated(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyFundingCreated(req); err != nil {
				return err
			}
			err = upsertFunding(txStmts, req)
			s.addBalance(req.UserID)
		case events.FundingApproved, events.FundingRejected:
			evt, decErr := events.DecodeFundingStatus(event.Data)
			if decErr != nil {
				return decErr
			}
			if event.Type == events.FundingApproved {
				if err := s.replayer.ApplyFundingApproved(evt); err != nil {
					return err
				}
			} else {
				if err := s.replayer.ApplyFundingRejected(evt); err != nil {
					return err
				}
			}
			status := types.FundingStatusCanceled
			if event.Type == events.FundingApproved {
				status = types.FundingStatusCompleted
			}
			err = updateFundingStatus(txStmts, evt.FundingID, status)
			userID, selErr := selectFundingUser(txStmts, evt.FundingID)
			if selErr != nil {
				return selErr
			}
			s.addBalance(userID)
		case events.OrderTriggered:
			evt, decErr := events.DecodeOrderTriggered(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyOrderTriggered(evt); err != nil {
				return err
			}
			s.scheduleOrderMutation(orderKey{userID: evt.UserID, orderID: evt.OrderID}, orderMutation{kind: orderMutationTrigger, timestamp: evt.Timestamp})
			s.addBalance(evt.UserID)
		case events.RPNLRecorded:
			evt, decErr := events.DecodeRPNL(event.Data)
			if decErr != nil {
				return decErr
			}
			if err := s.replayer.ApplyEvent(event); err != nil {
				return err
			}
			err = insertRPNL(txStmts, evt)
			s.addBalance(evt.UserID)
		default:
			if err := s.replayer.ApplyEvent(event); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}

	if err := s.flushOrderFills(txStmts); err != nil {
		return err
	}
	if err := s.flushOrderMutations(txStmts); err != nil {
		return err
	}

	for userID := range s.balances {
		balances := s.portfolio.GetBalances(userID)
		for i := range balances {
			if bal := balances[i]; bal != nil {
				err = upsertBalance(txStmts, bal)
				if err != nil {
					return err
				}
			}
		}
	}
	clear(s.balances)

	for key := range s.positions {
		pos := s.portfolio.GetPosition(key.userID, key.symbol)
		if pos == nil {
			continue
		}
		err = upsertPosition(txStmts, pos)
		if err != nil {
			return err
		}
	}
	clear(s.positions)

	return err
}

func (s *Store) loadState() error {
	return s.loadCore(s.store, s.portfolio)
}

func (s *Store) loadCore(store *oms.Service, portfolio *portfolio.Service) error {
	if store != nil {
		store.Reset()
	}
	if portfolio != nil {
		portfolio.Reset()
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
	if err := loadPendingFundings(s.db, portfolio); err != nil {
		return err
	}
	return nil
}

func loadPendingFundings(db *sql.DB, portfolio *portfolio.Service) error {
	query := `select id, user_id, type, status, asset, amount, destination, created_by, message, created_at, updated_at from fundings where status = 'PENDING'`
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var r struct {
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
		if err := rows.Scan(&r.ID, &r.UserID, &r.Type, &r.Status, &r.Asset, &r.Amount, &r.Destination, &r.CreatedBy, &r.Message, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return err
		}
		amount, err := fixed.Parse(r.Amount)
		if err != nil {
			return fmt.Errorf("parse funding amount: %w", err)
		}
		if portfolio.Fundings == nil {
			portfolio.Fundings = make(map[types.FundingID]*types.FundingRequest)
		}
		portfolio.Fundings[r.ID] = &types.FundingRequest{
			ID:          r.ID,
			UserID:      r.UserID,
			Type:        types.FundingType(r.Type),
			Status:      types.FundingStatus(r.Status),
			Asset:       r.Asset,
			Amount:      types.Quantity(amount),
			Destination: r.Destination,
			CreatedBy:   types.FundingCreatedBy(r.CreatedBy),
			Message:     r.Message,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		}
	}
	return nil
}

func upsertOrder(stmts *txStatements, order *types.Order) error {
	if order == nil {
		return fmt.Errorf("order is nil")
	}
	stmt := stmts.upsertOrder
	if stmt == nil {
		return fmt.Errorf("missing upsert order statement")
	}
	_, err := stmt.Exec(
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
		order.TriggerDirection,
		boolToInt(order.IsConditional),
		order.CreatedAt,
		order.UpdatedAt,
	)
	return err
}

func updateOrderPriceQty(stmts *txStatements, userID types.UserID, orderID types.OrderID, price types.Price, qty types.Quantity, ts uint64) error {
	stmt := stmts.updateOrderPriceQty
	if stmt == nil {
		return fmt.Errorf("missing update order price qty statement")
	}
	_, err := stmt.Exec(price.String(), qty.String(), ts, orderID, userID)
	return err
}
func updateOrderQty(stmts *txStatements, userID types.UserID, orderID types.OrderID, qty types.Quantity, ts uint64) error {
	stmt := stmts.updateOrderQty
	if stmt == nil {
		return fmt.Errorf("missing update order qty statement")
	}
	_, err := stmt.Exec(qty.String(), ts, orderID, userID)
	return err
}

func cancelOrder(stmts *txStatements, userID types.UserID, orderID types.OrderID, ts uint64) error {
	stmt := stmts.cancelOrder
	if stmt == nil {
		return fmt.Errorf("missing cancel order statement")
	}
	_, err := stmt.Exec(
		constants.ORDER_STATUS_DEACTIVATED,
		constants.ORDER_STATUS_CANCELED,
		ts,
		orderID,
		userID,
	)
	return err
}

func insertFill(stmts *txStatements, id types.TradeID, userID types.UserID, orderID types.OrderID, counterparty types.OrderID, symbol string, category int8, orderType int8, side int8, role string, price string, qty string, ts uint64) error {
	stmt := stmts.insertFill
	if stmt == nil {
		return fmt.Errorf("missing insert fill statement")
	}
	_, err := stmt.Exec(id, userID, orderID, counterparty, symbol, category, orderType, side, role, price, qty, ts)
	return err
}

func upsertOrderFill(stmts *txStatements, userID types.UserID, orderID types.OrderID, filled types.Quantity, qty types.Quantity, ts uint64) error {
	status := constants.ORDER_STATUS_PARTIALLY_FILLED
	if math.Cmp(filled, qty) >= 0 {
		status = constants.ORDER_STATUS_FILLED
	}
	stmt := stmts.updateOrderFilled
	if stmt == nil {
		return fmt.Errorf("missing update order filled statement")
	}
	_, err := stmt.Exec(filled.String(), status, ts, orderID, userID)
	return err
}

func markOrderTriggered(stmts *txStatements, userID types.UserID, orderID types.OrderID, ts uint64) error {
	stmt := stmts.markOrderTriggered
	if stmt == nil {
		return fmt.Errorf("missing mark order triggered statement")
	}
	_, err := stmt.Exec(constants.ORDER_STATUS_TRIGGERED, types.Price{}.String(), ts, orderID, userID)
	return err
}

func insertRPNL(stmts *txStatements, ev events.RPNLEvent) error {
	// Persists realized PnL into history store.
	stmt := stmts.insertRPNL
	if stmt == nil {
		return fmt.Errorf("missing insert rpnl statement")
	}
	_, err := stmt.Exec(
		snowflake.Next(),
		uint64(ev.UserID),
		uint64(ev.OrderID),
		ev.Symbol,
		ev.Category,
		ev.Side,
		ev.Price.String(),
		ev.Quantity.String(),
		ev.Realized.String(),
		ev.Timestamp,
	)
	return err
}

func upsertBalance(stmts *txStatements, bal *types.Balance) error {
	if bal == nil {
		return fmt.Errorf("balance is nil")
	}
	stmt := stmts.upsertBalance
	if stmt == nil {
		return fmt.Errorf("missing upsert balance statement")
	}
	_, err := stmt.Exec(bal.UserID, bal.Asset, bal.Available.String(), bal.Locked.String(), bal.Margin.String())
	return err
}

func upsertPosition(stmts *txStatements, pos *types.Position) error {
	if pos == nil {
		return fmt.Errorf("position is nil")
	}
	stmt := stmts.upsertPosition
	if stmt == nil {
		return fmt.Errorf("missing upsert position statement")
	}
	_, err := stmt.Exec(
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

func upsertFunding(stmts *txStatements, req *types.FundingRequest) error {
	if req == nil {
		return fmt.Errorf("funding request is nil")
	}
	stmt := stmts.upsertFunding
	if stmt == nil {
		return fmt.Errorf("missing upsert funding statement")
	}
	_, err := stmt.Exec(
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

func updateFundingStatus(stmts *txStatements, id types.FundingID, status types.FundingStatus) error {
	stmt := stmts.updateFundingStatus
	if stmt == nil {
		return fmt.Errorf("missing update funding status statement")
	}
	_, err := stmt.Exec(status, id)
	return err
}

func selectFundingUser(stmts *txStatements, id types.FundingID) (types.UserID, error) {
	stmt := stmts.selectFundingUser
	if stmt == nil {
		return 0, fmt.Errorf("missing select funding user statement")
	}
	row := stmt.QueryRow(id)
	var userID types.UserID
	if err := row.Scan(&userID); err != nil {
		return 0, err
	}
	return userID, nil
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
		`select id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, trigger_direction, is_conditional, created_at, updated_at
     from orders
     where status in (?, ?, ?, ?)`,
		constants.ORDER_STATUS_NEW,
		constants.ORDER_STATUS_PARTIALLY_FILLED,
		constants.ORDER_STATUS_UNTRIGGERED,
		constants.ORDER_STATUS_TRIGGERED,
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		order := types.Order{}
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
			&order.TriggerDirection,
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

func parseFixed(value string) types.Quantity {
	if value == "" {
		return types.Quantity{}
	}
	// Parse fixed-point values directly from strings to avoid float rounding.
	parsed, err := fixed.Parse(value)
	if err != nil {
		return types.Quantity{}
	}
	return types.Quantity(parsed)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func oppositeSide(side int8) int8 {
	if side == constants.ORDER_SIDE_BUY {
		return constants.ORDER_SIDE_SELL
	}
	return constants.ORDER_SIDE_BUY
}
