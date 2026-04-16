package persistence

import (
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// applyWriter executes one outbox batch against in-memory services and sqlite.
//
// Responsibilities are intentionally split by phase:
// 1) decode + replay in-memory state,
// 2) stage row mutations into accumulators,
// 3) flush staged writes at deterministic boundaries.
//
// This keeps restart replay and live background apply on the same path while
// keeping Store.Apply small and readable.
type applyWriter struct {
	s       *Store
	txStmts *txStatements
}

func (w *applyWriter) process(eventsBatch []events.Event) error {
	prevWasTrade := false
	for i := range eventsBatch {
		event := eventsBatch[i]
		isTrade := event.Type == events.TradeExecuted
		if isTrade {
			if !prevWasTrade {
				// Entering a trade streak: flush pending order mutations so order
				// rows are in a stable state before fill accumulation begins.
				if err := w.s.flushOrderMutations(w.txStmts); err != nil {
					return err
				}
			}
		} else if prevWasTrade {
			// Leaving a trade streak: flush trade side effects before processing
			// other event classes to preserve deterministic write ordering.
			if err := w.s.flushFillInserts(w.txStmts, true); err != nil {
				return err
			}
			if err := w.s.flushOrderProgressDeltas(w.txStmts); err != nil {
				return err
			}
		}
		if err := w.handleEvent(event); err != nil {
			return err
		}
		prevWasTrade = isTrade
	}
	return nil
}

func (w *applyWriter) finalize() error {
	if err := w.s.flushOrderProgressDeltas(w.txStmts); err != nil {
		return err
	}
	if err := w.s.flushFillInserts(w.txStmts, true); err != nil {
		return err
	}
	if err := w.s.flushOrderMutations(w.txStmts); err != nil {
		return err
	}
	if err := w.s.flushBalanceSnapshots(w.txStmts); err != nil {
		return err
	}
	return w.s.flushPositionSnapshots(w.txStmts)
}

func (w *applyWriter) handleEvent(event events.Event) error {
	s := w.s
	switch event.Type {
	case events.OrderPlaced:
		placed, err := events.DecodeOrderPlaced(event.Data)
		if err != nil {
			return err
		}
		if err := s.replayer.ApplyOrderPlaced(placed); err != nil {
			return err
		}
		if err := upsertOrder(w.txStmts, placed.Order); err != nil {
			return err
		}
		s.addBalance(placed.Order.UserID)
		return nil
	case events.OrderAmended:
		amend, err := events.DecodeOrderAmended(event.Data)
		if err != nil {
			return err
		}
		if err := s.replayer.ApplyOrderAmended(amend); err != nil {
			return err
		}
		s.scheduleOrderMutation(orderKey{userID: amend.UserID, orderID: amend.OrderID}, orderMutation{kind: orderMutationAmend, price: amend.NewPrice, qty: amend.NewQty, timestamp: amend.Timestamp})
		s.addBalance(amend.UserID)
		return nil
	case events.OrderCanceled:
		cancel, err := events.DecodeOrderCanceled(event.Data)
		if err != nil {
			return err
		}
		if err := s.replayer.ApplyOrderCanceled(cancel); err != nil {
			return err
		}
		s.scheduleOrderMutation(orderKey{userID: cancel.UserID, orderID: cancel.OrderID}, orderMutation{kind: orderMutationCancel, timestamp: cancel.Timestamp})
		s.addBalance(cancel.UserID)
		return nil
	case events.TradeExecuted:
		trade, instrumentPayload, err := events.DecodeTradeNoSymbolWithPayload(event.Data)
		if err != nil {
			return err
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
		inst, err := s.resolveTradeInstrument(trade.Symbol, instrumentPayload)
		if err != nil {
			return err
		}
		trade.Instrument = inst
		if err := s.replayer.ApplyTradeExecutedWithOrders(trade, makerOrder, takerOrder); err != nil {
			return err
		}
		s.stageOrderProgressDelta(makerOrder, trade.Quantity, trade.Timestamp)
		s.stageOrderProgressDelta(takerOrder, trade.Quantity, trade.Timestamp)
		price := trade.Price.String()
		qty := trade.Quantity.String()
		makerSide := oppositeSide(trade.TakerSide)
		s.appendTradeFills(trade, makerSide, price, qty)
		if err := s.flushFillInserts(w.txStmts, false); err != nil {
			return err
		}
		s.addBalance(trade.MakerUserID)
		s.addBalance(trade.TakerUserID)
		if trade.Category == constants.CATEGORY_LINEAR {
			s.addPosition(trade.MakerUserID, trade.Symbol)
			s.addPosition(trade.TakerUserID, trade.Symbol)
		}
		return nil
	case events.LeverageSet:
		lev, err := events.DecodeLeverage(event.Data)
		if err != nil {
			return err
		}
		if err := s.replayer.ApplyLeverageSet(lev); err != nil {
			return err
		}
		s.addPosition(lev.UserID, lev.Symbol)
		return nil
	case events.FundingCreated:
		req, err := events.DecodeFundingCreated(event.Data)
		if err != nil {
			return err
		}
		if err := s.replayer.ApplyFundingCreated(req); err != nil {
			return err
		}
		if err := upsertFunding(w.txStmts, req); err != nil {
			return err
		}
		s.addBalance(req.UserID)
		return nil
	case events.FundingApproved, events.FundingRejected:
		evt, err := events.DecodeFundingStatus(event.Data)
		if err != nil {
			return err
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
		if err := updateFundingStatus(w.txStmts, evt.FundingID, status); err != nil {
			return err
		}
		userID, err := selectFundingUser(w.txStmts, evt.FundingID)
		if err != nil {
			return err
		}
		s.addBalance(userID)
		return nil
	case events.OrderTriggered:
		evt, err := events.DecodeOrderTriggered(event.Data)
		if err != nil {
			return err
		}
		if err := s.replayer.ApplyOrderTriggered(evt); err != nil {
			return err
		}
		s.scheduleOrderMutation(orderKey{userID: evt.UserID, orderID: evt.OrderID}, orderMutation{kind: orderMutationTrigger, timestamp: evt.Timestamp})
		s.addBalance(evt.UserID)
		return nil
	case events.RPNLRecorded:
		evt, err := events.DecodeRPNL(event.Data)
		if err != nil {
			return err
		}
		if err := insertRPNL(w.txStmts, evt); err != nil {
			return err
		}
		s.addBalance(evt.UserID)
		return nil
	default:
		return s.replayer.ApplyEvent(event)
	}
}
