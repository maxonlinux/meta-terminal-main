package persistence

import (
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
	qty types.Quantity
	ts  uint64
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

func (s *Store) Apply(eventsBatch []events.Event) error {
	if len(eventsBatch) == 0 {
		return nil
	}
	s.balances = make(map[types.UserID]struct{})
	s.positions = make(map[positionKey]struct{})

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = s.loadState()
		}
	}()
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	addBalance := func(userID types.UserID) {
		s.balances[userID] = struct{}{}
	}
	addPosition := func(userID types.UserID, symbol string) {
		s.positions[positionKey{userID: userID, symbol: symbol}] = struct{}{}
	}

	pendingOrderFills := make(map[orderKey]orderFillAccum)
	flushOrderFills := func() error {
		if len(pendingOrderFills) == 0 {
			return nil
		}
		for key, accum := range pendingOrderFills {
			if err := addFillToOrder(tx, s.stmts, key.userID, key.orderID, accum.qty, accum.ts); err != nil {
				return err
			}
			delete(pendingOrderFills, key)
		}
		return nil
	}
	accumOrderFill := func(userID types.UserID, orderID types.OrderID, qty types.Quantity, ts uint64) {
		key := orderKey{userID: userID, orderID: orderID}
		accum := pendingOrderFills[key]
		accum.qty = math.Add(accum.qty, qty)
		if ts > accum.ts {
			accum.ts = ts
		}
		pendingOrderFills[key] = accum
	}

	for i := range eventsBatch {
		event := eventsBatch[i]
		if err := s.replayer.ApplyEvent(event); err != nil {
			return err
		}
		if event.Type != events.TradeExecuted {
			if err := flushOrderFills(); err != nil {
				return err
			}
		}
		switch event.Type {
		case events.OrderPlaced:
			placed, decErr := events.DecodeOrderPlaced(event.Data)
			if decErr != nil {
				return decErr
			}
			if placed.Instrument != nil && s.registry.GetInstrument(placed.Order.Symbol) == nil {
				s.registry.SetInstrument(placed.Order.Symbol, placed.Instrument)
			}
			err = upsertOrder(tx, s.stmts, placed.Order)
			addBalance(placed.Order.UserID)
		case events.OrderAmended:
			amend, decErr := events.DecodeOrderAmended(event.Data)
			if decErr != nil {
				return decErr
			}
			if math.Sign(amend.NewPrice) > 0 {
				err = updateOrderPriceQty(tx, s.stmts, amend.UserID, amend.OrderID, amend.NewPrice, amend.NewQty, amend.Timestamp)
			} else {
				err = updateOrderQty(tx, s.stmts, amend.UserID, amend.OrderID, amend.NewQty, amend.Timestamp)
			}
			addBalance(amend.UserID)
		case events.OrderCanceled:
			cancel, decErr := events.DecodeOrderCanceled(event.Data)
			if decErr != nil {
				return decErr
			}
			err = cancelOrder(tx, s.stmts, cancel.UserID, cancel.OrderID, cancel.Timestamp)
			addBalance(cancel.UserID)
		case events.TradeExecuted:
			trade, decErr := events.DecodeTrade(event.Data)
			if decErr != nil {
				return decErr
			}
			if trade.Instrument != nil && s.registry.GetInstrument(trade.Symbol) == nil {
				s.registry.SetInstrument(trade.Symbol, trade.Instrument)
			}
			price := trade.Price.String()
			qty := trade.Quantity.String()
			makerSide := oppositeSide(trade.TakerSide)
			if err = insertFill(tx, s.stmts, trade.TradeID, trade.MakerUserID, trade.MakerOrderID, trade.TakerOrderID, trade.Symbol, trade.Category, trade.MakerOrderType, makerSide, "MAKER", price, qty, trade.Timestamp); err != nil {
				return err
			}
			if err = insertFill(tx, s.stmts, trade.TradeID, trade.TakerUserID, trade.TakerOrderID, trade.MakerOrderID, trade.Symbol, trade.Category, trade.TakerOrderType, trade.TakerSide, "TAKER", price, qty, trade.Timestamp); err != nil {
				return err
			}
			accumOrderFill(trade.MakerUserID, trade.MakerOrderID, trade.Quantity, trade.Timestamp)
			accumOrderFill(trade.TakerUserID, trade.TakerOrderID, trade.Quantity, trade.Timestamp)
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
			userID, selErr := selectFundingUser(tx, s.stmts, evt.FundingID)
			if selErr != nil {
				return selErr
			}
			addBalance(userID)
		case events.OrderTriggered:
			evt, decErr := events.DecodeOrderTriggered(event.Data)
			if decErr != nil {
				return decErr
			}
			err = markOrderTriggered(tx, s.stmts, evt.UserID, evt.OrderID, evt.Timestamp)
			addBalance(evt.UserID)
		case events.RPNLRecorded:
			evt, decErr := events.DecodeRPNL(event.Data)
			if decErr != nil {
				return decErr
			}
			err = insertRPNL(tx, s.stmts, evt)
			addBalance(evt.UserID)
		}
		if err != nil {
			return err
		}
	}

	if err := flushOrderFills(); err != nil {
		return err
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

func upsertOrder(tx *sql.Tx, stmts *statements, order *types.Order) error {
	if order == nil {
		return fmt.Errorf("order is nil")
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
		order.TriggerDirection,
		boolToInt(order.IsConditional),
		order.CreatedAt,
		order.UpdatedAt,
	)
	return err
}

func updateOrderPriceQty(tx *sql.Tx, stmts *statements, userID types.UserID, orderID types.OrderID, price types.Price, qty types.Quantity, ts uint64) error {
	stmt := stmts.updateOrderPriceQty
	if stmt == nil {
		return fmt.Errorf("missing update order price qty statement")
	}
	_, err := tx.Stmt(stmt).Exec(price.String(), qty.String(), ts, orderID, userID)
	return err
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

	if err := insertFill(tx, stmts, trade.TradeID, trade.MakerUserID, trade.MakerOrderID, trade.TakerOrderID, trade.Symbol, trade.Category, trade.MakerOrderType, makerSide, "MAKER", price, qty, trade.Timestamp); err != nil {
		return err
	}
	if err := insertFill(tx, stmts, trade.TradeID, trade.TakerUserID, trade.TakerOrderID, trade.MakerOrderID, trade.Symbol, trade.Category, trade.TakerOrderType, trade.TakerSide, "TAKER", price, qty, trade.Timestamp); err != nil {
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

func insertFill(tx *sql.Tx, stmts *statements, id types.TradeID, userID types.UserID, orderID types.OrderID, counterparty types.OrderID, symbol string, category int8, orderType int8, side int8, role string, price string, qty string, ts uint64) error {
	stmt := stmts.insertFill
	if stmt == nil {
		return fmt.Errorf("missing insert fill statement")
	}
	_, err := tx.Stmt(stmt).Exec(id, userID, orderID, counterparty, symbol, category, orderType, side, role, price, qty, ts)
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

func insertRPNL(tx *sql.Tx, stmts *statements, ev events.RPNLEvent) error {
	// Persists realized PnL into history store.
	stmt := stmts.insertRPNL
	if stmt == nil {
		return fmt.Errorf("missing insert rpnl statement")
	}
	_, err := tx.Stmt(stmt).Exec(
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

func upsertBalance(tx *sql.Tx, stmts *statements, bal *types.Balance) error {
	if bal == nil {
		return fmt.Errorf("balance is nil")
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
		return fmt.Errorf("position is nil")
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
		return fmt.Errorf("funding request is nil")
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

func selectFundingUser(tx *sql.Tx, stmts *statements, id types.FundingID) (types.UserID, error) {
	stmt := stmts.selectFundingUser
	if stmt == nil {
		return 0, fmt.Errorf("missing select funding user statement")
	}
	row := tx.Stmt(stmt).QueryRow(id)
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
