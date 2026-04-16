package replay

import (
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type Replayer struct {
	store     *oms.Service
	portfolio *portfolio.Service
	clearing  *clearing.Service
	registry  *registry.Registry
}

func New(reg *registry.Registry, store *oms.Service, portfolio *portfolio.Service, clearing *clearing.Service) *Replayer {
	return &Replayer{
		store:     store,
		portfolio: portfolio,
		clearing:  clearing,
		registry:  reg,
	}
}

func (r *Replayer) ApplyEvent(ev events.Event) error {
	switch ev.Type {
	case events.OrderPlaced:
		placed, err := events.DecodeOrderPlaced(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyOrderPlaced(placed)
	case events.OrderAmended:
		amend, err := events.DecodeOrderAmended(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyOrderAmended(amend)
	case events.OrderCanceled:
		cancel, err := events.DecodeOrderCanceled(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyOrderCanceled(cancel)
	case events.TradeExecuted:
		trade, err := events.DecodeTrade(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyTradeExecuted(trade)
	case events.LeverageSet:
		lev, err := events.DecodeLeverage(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyLeverageSet(lev)
	case events.FundingCreated:
		req, err := events.DecodeFundingCreated(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyFundingCreated(req)
	case events.FundingApproved, events.FundingRejected:
		evt, err := events.DecodeFundingStatus(ev.Data)
		if err != nil {
			return err
		}
		if ev.Type == events.FundingApproved {
			return r.ApplyFundingApproved(evt)
		}
		return r.ApplyFundingRejected(evt)
	case events.OrderTriggered:
		evt, err := events.DecodeOrderTriggered(ev.Data)
		if err != nil {
			return err
		}
		return r.ApplyOrderTriggered(evt)
	}
	return nil
}

func (r *Replayer) ApplyOrderPlaced(placed events.OrderPlacedEvent) error {
	if placed.Instrument != nil {
		r.registry.SetInstrument(placed.Order.Symbol, placed.Instrument)
	}
	order := placed.Order
	r.store.Load(order)
	if order.IsConditional {
		return nil
	}
	if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
		return nil
	}
	remaining := math.Sub(order.Quantity, order.Filled)
	if math.Sign(remaining) > 0 {
		if err := r.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price); err != nil {
			return err
		}
	}
	return nil
}

func (r *Replayer) ApplyOrderAmended(amend events.OrderAmendedEvent) error {
	order, ok := r.store.GetUserOrder(amend.UserID, amend.OrderID)
	if !ok {
		return nil
	}
	order.UpdatedAt = amend.Timestamp
	if math.Sign(amend.NewPrice) > 0 {
		order.Price = amend.NewPrice
	}
	if order.IsConditional || order.Type != constants.ORDER_TYPE_LIMIT {
		order.Quantity = amend.NewQty
		return nil
	}
	oldRemaining := math.Sub(order.Quantity, order.Filled)
	order.Quantity = amend.NewQty
	newRemaining := math.Sub(order.Quantity, order.Filled)
	delta := math.Sub(newRemaining, oldRemaining)
	if math.Sign(delta) > 0 {
		if err := r.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, delta, order.Price); err != nil {
			return err
		}
	} else if math.Sign(delta) < 0 {
		if err := r.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, math.Neg(delta), order.Price); err != nil {
			return err
		}
	}
	return nil
}

func (r *Replayer) ApplyOrderCanceled(cancel events.OrderCanceledEvent) error {
	if order, ok := r.store.GetUserOrder(cancel.UserID, cancel.OrderID); ok {
		if order.IsConditional {
			order.UpdatedAt = cancel.Timestamp
			if err := r.store.Cancel(order.UserID, order.ID); err != nil {
				return err
			}
			return nil
		}
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			if err := r.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price); err != nil {
				return err
			}
		}
		order.UpdatedAt = cancel.Timestamp
		if err := r.store.Cancel(order.UserID, order.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Replayer) ApplyTradeExecuted(trade events.TradeEvent) error {
	maker, _ := r.store.GetUserOrder(trade.MakerUserID, trade.MakerOrderID)
	taker, _ := r.store.GetUserOrder(trade.TakerUserID, trade.TakerOrderID)
	return r.ApplyTradeExecutedWithOrders(trade, maker, taker)
}

func (r *Replayer) ApplyTradeExecutedWithOrders(trade events.TradeEvent, maker *types.Order, taker *types.Order) error {
	if trade.Instrument != nil {
		r.registry.SetInstrument(trade.Symbol, trade.Instrument)
	}
	if maker == nil || taker == nil {
		return fmt.Errorf("missing orders for trade %d", trade.TradeID)
	}
	match := types.Match{
		ID:         trade.TradeID,
		Symbol:     trade.Symbol,
		Category:   trade.Category,
		Price:      trade.Price,
		Quantity:   trade.Quantity,
		TakerOrder: taker,
		MakerOrder: maker,
		Timestamp:  trade.Timestamp,
	}
	if maker.Type == constants.ORDER_TYPE_MARKET && maker.Price.Sign() == 0 {
		if err := r.clearing.Reserve(maker.UserID, maker.Symbol, maker.Category, maker.Side, trade.Quantity, trade.Price); err != nil {
			return err
		}
	}
	if taker.Type == constants.ORDER_TYPE_MARKET && taker.Price.Sign() == 0 {
		if err := r.clearing.Reserve(taker.UserID, taker.Symbol, taker.Category, taker.Side, trade.Quantity, trade.Price); err != nil {
			return err
		}
	}
	if err := r.clearing.ExecuteTrade(&match); err != nil {
		return err
	}
	if err := r.store.Fill(maker.UserID, maker.ID, trade.Quantity); err != nil {
		return err
	}
	if err := r.store.Fill(taker.UserID, taker.ID, trade.Quantity); err != nil {
		return err
	}
	return nil
}

func (r *Replayer) ApplyLeverageSet(lev events.LeverageEvent) error {
	_ = r.portfolio.SetLeverage(lev.UserID, lev.Symbol, lev.Leverage)
	return nil
}

func (r *Replayer) ApplyFundingCreated(req *types.FundingRequest) error {
	r.portfolio.Fundings[req.ID] = req
	if req.Type == types.FundingTypeWithdrawal && req.Status == types.FundingStatusPending {
		_ = r.portfolio.Reserve(req.UserID, req.Asset, req.Amount)
	}
	return nil
}

func (r *Replayer) ApplyFundingApproved(evt events.FundingStatusEvent) error {
	_, _ = r.portfolio.ApproveFunding(evt.FundingID)
	return nil
}

func (r *Replayer) ApplyFundingRejected(evt events.FundingStatusEvent) error {
	_, _ = r.portfolio.RejectFunding(evt.FundingID)
	return nil
}

func (r *Replayer) ApplyOrderTriggered(evt events.OrderTriggeredEvent) error {
	if order, ok := r.store.GetUserOrder(evt.UserID, evt.OrderID); ok {
		order.Status = constants.ORDER_STATUS_TRIGGERED
		order.UpdatedAt = evt.Timestamp
		order.IsConditional = false
		order.TriggerPrice = types.Price{}
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			_ = r.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
		}
	}
	return nil
}
