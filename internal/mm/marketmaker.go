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
	defaultMinBalance    = 500000
	defaultTickQueue     = 4096
)

type Config struct {
	Levels        int
	Interval      time.Duration
	MinNotional   int64
	MaxNotional   int64
	CancelPercent float64
	SkipPercent   float64
	BotUserID     types.UserID
	MinBalance    int64
}

type MarketMaker struct {
	eng *engine.Engine
	reg *registry.Registry
	cfg Config
	val *rand.Rand

	orders map[marketKey]map[string]types.OrderID
	ticks  chan priceTick
}

type marketKey struct {
	symbol   string
	category int8
}

type priceTick struct {
	symbol string
	price  types.Price
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
	if cfg.MinBalance <= 0 {
		cfg.MinBalance = defaultMinBalance
	}

	return &MarketMaker{
		eng:    eng,
		reg:    reg,
		cfg:    cfg,
		val:    rand.New(rand.NewSource(time.Now().UnixNano())),
		orders: make(map[marketKey]map[string]types.OrderID),
		ticks:  make(chan priceTick, defaultTickQueue),
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
		case tick := <-m.ticks:
			m.refreshSymbol(tick.symbol, tick.price)
		}
	}
}

// OnPriceTick enqueues a per-symbol refresh for market makers.
func (m *MarketMaker) OnPriceTick(symbol string, price types.Price) {
	if m == nil {
		return
	}
	select {
	case m.ticks <- priceTick{symbol: symbol, price: price}:
	default:
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
		// Top up after reserves to keep available balance above minimum.
		m.ensureBalances(inst)
	}
}

func (m *MarketMaker) refreshSymbol(symbol string, price types.Price) {
	if math.Sign(price) <= 0 {
		return
	}
	inst := m.reg.GetInstrument(symbol)
	if inst == nil {
		return
	}
	m.ensureBalances(inst)
	m.updateMarket(inst, constants.CATEGORY_SPOT, price)
	m.updateMarket(inst, constants.CATEGORY_LINEAR, price)
	// Top up after reserves to keep available balance above minimum.
	m.ensureBalances(inst)
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
				desired[levelKey(constants.ORDER_SIDE_SELL, i)] = *req
			}
		}
		if math.Sign(down) > 0 {
			req := m.buildOrder(inst, category, constants.ORDER_SIDE_BUY, down)
			if req != nil {
				desired[levelKey(constants.ORDER_SIDE_BUY, i)] = *req
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

	// Add or requote levels
	for level, req := range desired {
		orderID, ok := existing[level]
		if !ok {
			if m.val.Float64() < m.cfg.SkipPercent {
				continue
			}
			m.ensureBalanceForOrder(inst, &req)
			res := m.eng.Cmd(&engine.PlaceOrderCmd{Req: &req})
			if res.Err != nil || res.Order == nil {
				log.Printf("mm: place failed symbol=%s category=%d price=%s err=%v", req.Symbol, req.Category, req.Price.String(), res.Err)
				continue
			}
			existing[level] = res.Order.ID
			continue
		}

		// Amend existing order to new price/qty to avoid DB growth.
		m.ensureBalanceForOrder(inst, &req)
		res := m.eng.Cmd(&engine.AmendOrderCmd{
			UserID:   m.cfg.BotUserID,
			OrderID:  orderID,
			NewQty:   req.Quantity,
			NewPrice: req.Price,
		})
		if res.Err != nil {
			log.Printf("mm: amend failed symbol=%s category=%d price=%s err=%v", req.Symbol, req.Category, req.Price.String(), res.Err)
		}
	}
}

func (m *MarketMaker) buildOrder(inst *types.Instrument, category int8, side int8, price types.Price) *types.PlaceOrderRequest {
	if math.Sign(price) <= 0 {
		return nil
	}
	price = types.Price(math.RoundTo(price, inst.TickSize))
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
	minBal := types.Quantity(fixed.NewI(m.cfg.MinBalance, 0))
	if inst.BaseAsset != "" {
		m.ensureBalance(bot, inst.BaseAsset, minBal)
	}
	if inst.QuoteAsset != "" {
		m.ensureBalance(bot, inst.QuoteAsset, minBal)
	}
}

func (m *MarketMaker) ensureBalanceForOrder(inst *types.Instrument, req *types.PlaceOrderRequest) {
	if req == nil || inst == nil {
		return
	}
	bot := m.cfg.BotUserID
	minBal := types.Quantity(fixed.NewI(m.cfg.MinBalance, 0))
	if req.Category == constants.CATEGORY_LINEAR {
		if inst.QuoteAsset != "" {
			m.ensureBalance(bot, inst.QuoteAsset, minBal)
		}
		return
	}
	if req.Side == constants.ORDER_SIDE_BUY {
		if inst.QuoteAsset != "" {
			m.ensureBalance(bot, inst.QuoteAsset, minBal)
		}
		return
	}
	if inst.BaseAsset != "" {
		m.ensureBalance(bot, inst.BaseAsset, minBal)
	}
}

func (m *MarketMaker) ensureBalance(userID types.UserID, asset string, minBalance types.Quantity) {
	bal := m.eng.Portfolio().GetBalance(userID, asset)
	if bal == nil {
		m.eng.Portfolio().LoadBalance(&types.Balance{UserID: userID, Asset: asset, Available: minBalance})
		return
	}
	if math.Cmp(bal.Available, minBalance) < 0 {
		m.eng.Portfolio().LoadBalance(&types.Balance{
			UserID:    userID,
			Asset:     asset,
			Available: minBalance,
			Locked:    bal.Locked,
			Margin:    bal.Margin,
		})
	}
}

func levelKey(side int8, level int) string {
	return string(rune('0'+side)) + ":" + fixed.NewI(int64(level), 0).String()
}
