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
	return r.applyEvent(ev)
}

func (r *Replayer) applyEvent(ev events.Event) error {
	switch ev.Type {
	case events.OrderPlaced:
		order, err := events.DecodeOrderPlaced(ev.Data)
		if err != nil {
			return err
		}
		r.store.Load(order)
		if order.IsConditional {
			return nil
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			return nil
		}
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			_ = r.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
		}
	case events.OrderAmended:
		amend, err := events.DecodeOrderAmended(ev.Data)
		if err != nil {
			return err
		}
		order, ok := r.store.GetUserOrder(amend.UserID, amend.OrderID)
		if !ok {
			return nil
		}
		order.UpdatedAt = amend.Timestamp
		if order.IsConditional || order.Type != constants.ORDER_TYPE_LIMIT {
			order.Quantity = amend.NewQty
			return nil
		}
		oldRemaining := math.Sub(order.Quantity, order.Filled)
		order.Quantity = amend.NewQty
		newRemaining := math.Sub(order.Quantity, order.Filled)
		delta := math.Sub(newRemaining, oldRemaining)
		if math.Sign(delta) > 0 {
			_ = r.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, delta, order.Price)
		} else if math.Sign(delta) < 0 {
			r.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, math.Neg(delta), order.Price)
		}
	case events.OrderCanceled:
		cancel, err := events.DecodeOrderCanceled(ev.Data)
		if err != nil {
			return err
		}
		if order, ok := r.store.GetUserOrder(cancel.UserID, cancel.OrderID); ok {
			if order.IsConditional {
				order.UpdatedAt = cancel.Timestamp
				_ = r.store.Cancel(order.UserID, order.ID)
				return nil
			}
			remaining := math.Sub(order.Quantity, order.Filled)
			if math.Sign(remaining) > 0 {
				r.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
			}
			order.UpdatedAt = cancel.Timestamp
			_ = r.store.Cancel(order.UserID, order.ID)
		}
	case events.TradeExecuted:
		trade, err := events.DecodeTrade(ev.Data)
		if err != nil {
			return err
		}
		maker, _ := r.store.GetUserOrder(trade.MakerUserID, trade.MakerOrderID)
		taker, _ := r.store.GetUserOrder(trade.TakerUserID, trade.TakerOrderID)
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
			_ = r.clearing.Reserve(maker.UserID, maker.Symbol, maker.Category, maker.Side, trade.Quantity, trade.Price)
		}
		if taker.Type == constants.ORDER_TYPE_MARKET && taker.Price.Sign() == 0 {
			_ = r.clearing.Reserve(taker.UserID, taker.Symbol, taker.Category, taker.Side, trade.Quantity, trade.Price)
		}
		r.clearing.ExecuteTrade(&match)
		_ = r.store.Fill(maker.UserID, maker.ID, trade.Quantity)
		_ = r.store.Fill(taker.UserID, taker.ID, trade.Quantity)
	case events.LeverageSet:
		lev, err := events.DecodeLeverage(ev.Data)
		if err != nil {
			return err
		}
		_ = r.portfolio.SetLeverage(lev.UserID, lev.Symbol, lev.Leverage)
	case events.FundingCreated:
		req, err := events.DecodeFundingCreated(ev.Data)
		if err != nil {
			return err
		}
		r.portfolio.Fundings[req.ID] = req
		if req.Type == types.FundingTypeWithdrawal && req.Status == types.FundingStatusPending {
			_ = r.portfolio.Reserve(req.UserID, req.Asset, req.Amount)
		}
	case events.FundingApproved:
		evt, err := events.DecodeFundingStatus(ev.Data)
		if err != nil {
			return err
		}
		_, _ = r.portfolio.ApproveFunding(evt.FundingID)
	case events.FundingRejected:
		evt, err := events.DecodeFundingStatus(ev.Data)
		if err != nil {
			return err
		}
		_, _ = r.portfolio.RejectFunding(evt.FundingID)
	case events.OrderTriggered:
		evt, err := events.DecodeOrderTriggered(ev.Data)
		if err != nil {
			return err
		}
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
	}
	return nil
}
