package mm

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

const (
	defaultLevels        = 15
	defaultInterval      = 5 * time.Second
	defaultMinNotional   = 10000
	defaultMaxNotional   = 50000
	defaultCancelPercent = 0.2
	defaultSkipPercent   = 0.1
)

type Config struct {
	Levels        int
	Interval      time.Duration
	MinNotional   int64
	MaxNotional   int64
	CancelPercent float64
	SkipPercent   float64
	BotUserID     types.UserID
}

type MarketMaker struct {
	eng *engine.Engine
	reg *registry.Registry
	cfg Config
	val *rand.Rand

	orders map[marketKey]map[string]types.OrderID
}

type marketKey struct {
	symbol   string
	category int8
}

func New(eng *engine.Engine, reg *registry.Registry, cfg Config) *MarketMaker {
	if cfg.Levels <= 0 {
		cfg.Levels = defaultLevels
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.MinNotional <= 0 {
		cfg.MinNotional = defaultMinNotional
	}
	if cfg.MaxNotional <= 0 {
		cfg.MaxNotional = defaultMaxNotional
	}
	if cfg.CancelPercent <= 0 {
		cfg.CancelPercent = defaultCancelPercent
	}
	if cfg.SkipPercent < 0 {
		cfg.SkipPercent = defaultSkipPercent
	}
	if cfg.BotUserID == 0 {
		cfg.BotUserID = types.UserID(999999999)
	}

	return &MarketMaker{
		eng:    eng,
		reg:    reg,
		cfg:    cfg,
		val:    rand.New(rand.NewSource(time.Now().UnixNano())),
		orders: make(map[marketKey]map[string]types.OrderID),
	}
}

func (m *MarketMaker) Start(ctx context.Context) {
	if m == nil {
		return
	}
	go m.loop(ctx)
}

func (m *MarketMaker) loop(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh()
		}
	}
}

func (m *MarketMaker) refresh() {
	insts := m.reg.GetInstruments()
	for _, inst := range insts {
		if inst == nil {
			continue
		}
		priceTick, ok := m.reg.GetPrice(inst.Symbol)
		if !ok || math.Sign(priceTick.Price) <= 0 {
			continue
		}

		m.ensureBalances(inst)

		m.updateMarket(inst, constants.CATEGORY_SPOT, priceTick.Price)
		m.updateMarket(inst, constants.CATEGORY_LINEAR, priceTick.Price)
	}
}

func (m *MarketMaker) updateMarket(inst *types.Instrument, category int8, price types.Price) {
	key := marketKey{symbol: inst.Symbol, category: category}
	existing := m.orders[key]
	if existing == nil {
		existing = make(map[string]types.OrderID)
		m.orders[key] = existing
	}

	desired := make(map[string]types.PlaceOrderRequest, m.cfg.Levels*2)
	step := inst.TickSize
	for i := 1; i <= m.cfg.Levels; i++ {
		stepSize := types.Price(math.Mul(step, fixed.NewI(int64(i), 0)))
		up := types.Price(math.Add(price, stepSize))
		down := types.Price(math.Sub(price, stepSize))
		if math.Sign(up) > 0 {
			req := m.buildOrder(inst, category, constants.ORDER_SIDE_SELL, up)
			if req != nil {
				desired[levelKey(constants.ORDER_SIDE_SELL, up)] = *req
			}
		}
		if math.Sign(down) > 0 {
			req := m.buildOrder(inst, category, constants.ORDER_SIDE_BUY, down)
			if req != nil {
				desired[levelKey(constants.ORDER_SIDE_BUY, down)] = *req
			}
		}
	}

	// Cancel stale or randomly replaced orders
	for level, orderID := range existing {
		if _, ok := desired[level]; !ok || m.val.Float64() < m.cfg.CancelPercent {
			_ = m.eng.Cmd(&engine.CancelOrderCmd{UserID: m.cfg.BotUserID, OrderID: orderID})
			delete(existing, level)
		}
	}

	// Add missing or randomly skipped levels
	for level, req := range desired {
		if _, ok := existing[level]; ok {
			continue
		}
		if m.val.Float64() < m.cfg.SkipPercent {
			continue
		}
		res := m.eng.Cmd(&engine.PlaceOrderCmd{Req: &req})
		if res.Err != nil || res.Order == nil {
			log.Printf("mm: place failed symbol=%s category=%d price=%s err=%v", req.Symbol, req.Category, req.Price.String(), res.Err)
			continue
		}
		existing[level] = res.Order.ID
	}
}

func (m *MarketMaker) buildOrder(inst *types.Instrument, category int8, side int8, price types.Price) *types.PlaceOrderRequest {
	if math.Sign(price) <= 0 {
		return nil
	}
	minNotional := m.cfg.MinNotional
	maxNotional := m.cfg.MaxNotional
	if maxNotional < minNotional {
		maxNotional = minNotional
	}
	notional := minNotional
	if maxNotional > minNotional {
		notional = minNotional + m.val.Int63n(maxNotional-minNotional+1)
	}

	qty := types.Quantity(math.Div(types.Quantity(fixed.NewI(notional, 0)), price))
	qty = types.Quantity(math.RoundTo(qty, inst.LotSize))
	if math.Cmp(qty, inst.MinQty) < 0 {
		qty = inst.MinQty
	}
	if math.Cmp(qty, inst.MaxQty) > 0 {
		qty = inst.MaxQty
	}
	if math.Sign(qty) <= 0 {
		return nil
	}

	return &types.PlaceOrderRequest{
		UserID:   m.cfg.BotUserID,
		Symbol:   inst.Symbol,
		Category: category,
		Origin:   constants.ORDER_ORIGIN_SYSTEM,
		Side:     side,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price,
		Quantity: qty,
	}
}

func (m *MarketMaker) ensureBalances(inst *types.Instrument) {
	bot := m.cfg.BotUserID
	big := types.Quantity(fixed.NewI(1000000000, 0))
	if inst.BaseAsset != "" {
		m.eng.Portfolio().LoadBalance(&types.Balance{UserID: bot, Asset: inst.BaseAsset, Available: big})
	}
	if inst.QuoteAsset != "" {
		m.eng.Portfolio().LoadBalance(&types.Balance{UserID: bot, Asset: inst.QuoteAsset, Available: big})
	}
}

func levelKey(side int8, price types.Price) string {
	return string(rune('0'+side)) + ":" + price.String()
}
