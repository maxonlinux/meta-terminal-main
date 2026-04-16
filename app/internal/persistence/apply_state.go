package persistence

import (
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// resetApplyState clears per-batch scratch structures while retaining capacity.
// This keeps allocations flatter under sustained outbox throughput.
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
	s.fillHistoryRows = s.fillHistoryRows[:0]
	if s.orderProgressDeltas == nil {
		s.orderProgressDeltas = make(map[orderKey]orderProgressDelta)
	} else {
		clear(s.orderProgressDeltas)
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

// stageOrderProgressDelta accumulates updates for the orders table.
// This is distinct from fill history rows: it only tracks per-order aggregate
// progress (filled quantity and status) for the current Apply transaction.
func (s *Store) stageOrderProgressDelta(order *types.Order, qty types.Quantity, ts uint64) {
	key := orderKey{userID: order.UserID, orderID: order.ID}
	accum, ok := s.orderProgressDeltas[key]
	if !ok {
		accum.qty = order.Quantity
		accum.filled = order.Filled
	}
	accum.filled = math.Add(accum.filled, qty)
	if ts > accum.ts {
		accum.ts = ts
	}
	s.orderProgressDeltas[key] = accum
}

// flushOrderProgressDeltas materializes accumulated filled/status changes once per
// boundary instead of updating the same order row for every trade event.
func (s *Store) flushOrderProgressDeltas(stmts *txStatements) error {
	if len(s.orderProgressDeltas) == 0 {
		return nil
	}
	for key, accum := range s.orderProgressDeltas {
		if err := upsertOrderFill(stmts, key.userID, key.orderID, accum.filled, accum.qty, accum.ts); err != nil {
			return err
		}
	}
	clear(s.orderProgressDeltas)
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

// flushOrderMutations applies the coalesced amend/cancel/trigger set.
// Coalescing keeps order table write pressure bounded inside trade bursts.
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

func (s *Store) appendTradeFills(trade events.TradeEvent, makerSide int8, price string, qty string) {
	// fillHistoryRows maps 1:1 to rows in the fills table (trade history).
	// We append both maker and taker rows for every trade.
	s.fillHistoryRows = append(s.fillHistoryRows,
		fillInsertRow{
			id:           trade.TradeID,
			userID:       trade.MakerUserID,
			orderID:      trade.MakerOrderID,
			counterparty: trade.TakerOrderID,
			symbol:       trade.Symbol,
			category:     trade.Category,
			orderType:    trade.MakerOrderType,
			side:         makerSide,
			role:         "MAKER",
			price:        price,
			qty:          qty,
			ts:           trade.Timestamp,
		},
		fillInsertRow{
			id:           trade.TradeID,
			userID:       trade.TakerUserID,
			orderID:      trade.TakerOrderID,
			counterparty: trade.MakerOrderID,
			symbol:       trade.Symbol,
			category:     trade.Category,
			orderType:    trade.TakerOrderType,
			side:         trade.TakerSide,
			role:         "TAKER",
			price:        price,
			qty:          qty,
			ts:           trade.Timestamp,
		},
	)
}

func (s *Store) flushFillInserts(stmts *txStatements, all bool) error {
	if len(s.fillHistoryRows) == 0 {
		return nil
	}
	// Fast path during trade-heavy bursts: push fixed-size blocks through a
	// precompiled multi-row statement to reduce sqlite step/bind overhead.
	processed := 0
	if all {
		for processed+fillInsertBlockSize <= len(s.fillHistoryRows) {
			if err := insertFill8(stmts, s.fillHistoryRows[processed:processed+fillInsertBlockSize]); err != nil {
				return err
			}
			processed += fillInsertBlockSize
		}
		for processed < len(s.fillHistoryRows) {
			if err := insertFill(stmts, s.fillHistoryRows[processed]); err != nil {
				return err
			}
			processed++
		}
		s.fillHistoryRows = s.fillHistoryRows[:0]
		return nil
	}
	for processed+fillInsertBlockSize <= len(s.fillHistoryRows) {
		if err := insertFill8(stmts, s.fillHistoryRows[processed:processed+fillInsertBlockSize]); err != nil {
			return err
		}
		processed += fillInsertBlockSize
	}
	if processed > 0 {
		remaining := len(s.fillHistoryRows) - processed
		copy(s.fillHistoryRows, s.fillHistoryRows[processed:])
		s.fillHistoryRows = s.fillHistoryRows[:remaining]
	}
	return nil
}

func insertFill(stmts *txStatements, row fillInsertRow) error {
	stmt := stmts.insertFill
	if stmt == nil {
		return fmt.Errorf("missing insert fill statement")
	}
	var args [12]any
	setFillArgs(args[:], 0, row)
	_, err := stmt.Exec(args[:]...)
	return err
}

func insertFill8(stmts *txStatements, rows []fillInsertRow) error {
	if len(rows) != fillInsertBlockSize {
		return fmt.Errorf("expected %d fill rows, got %d", fillInsertBlockSize, len(rows))
	}
	stmt := stmts.insertFill8
	if stmt == nil {
		return fmt.Errorf("missing insert fill8 statement")
	}
	// Build args in a fixed array so we avoid allocating a new slice each call.
	var args [fillInsertBlockSize * 12]any
	for i := 0; i < fillInsertBlockSize; i++ {
		setFillArgs(args[:], i*12, rows[i])
	}
	_, err := stmt.Exec(args[:]...)
	return err
}

func setFillArgs(args []any, offset int, row fillInsertRow) {
	args[offset+0] = row.id
	args[offset+1] = row.userID
	args[offset+2] = row.orderID
	args[offset+3] = row.counterparty
	args[offset+4] = row.symbol
	args[offset+5] = row.category
	args[offset+6] = row.orderType
	args[offset+7] = row.side
	args[offset+8] = row.role
	args[offset+9] = row.price
	args[offset+10] = row.qty
	args[offset+11] = row.ts
}

// flushBalanceSnapshots persists portfolio balances only for users touched by
// the current apply batch.
func (s *Store) flushBalanceSnapshots(stmts *txStatements) error {
	for userID := range s.balances {
		balances := s.portfolio.GetBalances(userID)
		for i := range balances {
			if bal := balances[i]; bal != nil {
				if err := upsertBalance(stmts, bal); err != nil {
					return err
				}
			}
		}
	}
	clear(s.balances)
	return nil
}

// flushPositionSnapshots persists positions only for user/symbol keys touched
// by the current apply batch.
func (s *Store) flushPositionSnapshots(stmts *txStatements) error {
	for key := range s.positions {
		pos := s.portfolio.GetPosition(key.userID, key.symbol)
		if pos == nil {
			continue
		}
		if err := upsertPosition(stmts, pos); err != nil {
			return err
		}
	}
	clear(s.positions)
	return nil
}
