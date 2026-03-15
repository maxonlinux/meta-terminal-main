package mm

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/logging"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

const (
	defaultLevels        = 15
	defaultInterval      = 5 * time.Second
	defaultMinNotional   = 300000
	defaultMaxNotional   = 400000
	defaultCancelPercent = 0.2
	defaultSkipPercent   = 0.1
	defaultMinBalance    = 10000000
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
	MaxBalance    int64
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
	if cfg.MaxBalance <= 0 {
		cfg.MaxBalance = cfg.MinBalance * 2
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
	existing, stale := m.rebuildExisting(key, inst, category, price)
	m.orders[key] = make(map[string]types.OrderID, len(existing))

	desired := make(map[string]types.PlaceOrderRequest, m.cfg.Levels*2)
	var bestBuy types.Price
	var bestSell types.Price
	hasBuy := false
	hasSell := false
	step := inst.TickSize
	for i := 1; i <= m.cfg.Levels; i++ {
		stepSize := types.Price(math.Mul(step, fixed.NewI(int64(i), 0)))
		up := types.Price(math.Add(price, stepSize))
		down := types.Price(math.Sub(price, stepSize))
		if math.Sign(up) > 0 {
			req := m.buildOrder(inst, category, constants.ORDER_SIDE_SELL, up)
			if req != nil {
				desired[levelKey(constants.ORDER_SIDE_SELL, i)] = *req
				if !hasSell || math.Cmp(req.Price, bestSell) < 0 {
					bestSell = req.Price
				}
				hasSell = true
			}
		}
		if math.Sign(down) > 0 {
			req := m.buildOrder(inst, category, constants.ORDER_SIDE_BUY, down)
			if req != nil {
				desired[levelKey(constants.ORDER_SIDE_BUY, i)] = *req
				if !hasBuy || math.Cmp(req.Price, bestBuy) > 0 {
					bestBuy = req.Price
				}
				hasBuy = true
			}
		}
	}

	if hasBuy && hasSell && math.Cmp(bestBuy, bestSell) >= 0 {
		filtered := make(map[string]types.PlaceOrderRequest, len(desired))
		for level, req := range desired {
			if req.Side == constants.ORDER_SIDE_BUY && math.Cmp(req.Price, bestSell) >= 0 {
				continue
			}
			if req.Side == constants.ORDER_SIDE_SELL && math.Cmp(req.Price, bestBuy) <= 0 {
				continue
			}
			filtered[level] = req
		}
		desired = filtered
	}

	// Cancel stale orders first
	for _, order := range stale {
		if order == nil {
			continue
		}
		_ = m.eng.Cmd(&engine.CancelOrderCmd{UserID: m.cfg.BotUserID, OrderID: order.ID})
	}

	// Cancel stale or randomly replaced orders
	for level, order := range existing {
		if order == nil {
			continue
		}
		if _, ok := desired[level]; !ok || m.val.Float64() < m.cfg.CancelPercent {
			_ = m.eng.Cmd(&engine.CancelOrderCmd{UserID: m.cfg.BotUserID, OrderID: order.ID})
			delete(existing, level)
			continue
		}
		m.orders[key][level] = order.ID
	}

	// Amend existing orders first to avoid self-matching on new placements.
	for level, req := range desired {
		order, ok := existing[level]
		if !ok {
			continue
		}
		if m.wouldSelfMatch(&req) {
			_ = m.eng.Cmd(&engine.CancelOrderCmd{UserID: m.cfg.BotUserID, OrderID: order.ID})
			delete(existing, level)
			continue
		}
		m.ensureBalanceForOrder(inst, &req)
		res := m.eng.Cmd(&engine.AmendOrderCmd{
			UserID:   m.cfg.BotUserID,
			OrderID:  order.ID,
			NewQty:   req.Quantity,
			NewPrice: req.Price,
		})
		if res.Err != nil && !errors.Is(res.Err, constants.ErrSelfMatch) {
			logging.Log().Error().Str("symbol", req.Symbol).Int8("category", req.Category).Str("price", req.Price.String()).Err(res.Err).Msg("mm: amend failed")
		}
	}

	// Place new orders after amendments are applied.
	for level, req := range desired {
		if _, ok := existing[level]; ok {
			continue
		}
		if m.val.Float64() < m.cfg.SkipPercent {
			continue
		}
		if m.wouldSelfMatch(&req) {
			continue
		}
		m.ensureBalanceForOrder(inst, &req)
		res := m.eng.Cmd(&engine.PlaceOrderCmd{Req: &req})
		if res.Err != nil || res.Order == nil {
			if res.Err != nil && !errors.Is(res.Err, constants.ErrSelfMatch) {
				logging.Log().Error().Str("symbol", req.Symbol).Int8("category", req.Category).Str("price", req.Price.String()).Err(res.Err).Msg("mm: place failed")
			}
			continue
		}
		existing[level] = res.Order
		m.orders[key][level] = res.Order.ID
	}
}

func (m *MarketMaker) rebuildExisting(key marketKey, inst *types.Instrument, category int8, mark types.Price) (map[string]*types.Order, []*types.Order) {
	store := m.eng.Store()
	if store == nil {
		return make(map[string]*types.Order), nil
	}
	orders := store.GetUserOrders(m.cfg.BotUserID)
	if len(orders) == 0 {
		return make(map[string]*types.Order), nil
	}
	result := make(map[string]*types.Order)
	stale := make([]*types.Order, 0)
	if existingLevels, ok := m.orders[key]; ok {
		for level, id := range existingLevels {
			order, ok := store.GetUserOrder(m.cfg.BotUserID, id)
			if !ok || order == nil {
				continue
			}
			if order.Symbol != inst.Symbol || order.Category != category {
				continue
			}
			if order.Origin != constants.ORDER_ORIGIN_SYSTEM {
				continue
			}
			switch order.Status {
			case constants.ORDER_STATUS_NEW, constants.ORDER_STATUS_PARTIALLY_FILLED:
			default:
				continue
			}
			result[level] = order
		}
	}
	levelByID := make(map[types.OrderID]string)
	for level, order := range result {
		levelByID[order.ID] = level
	}
	for _, order := range orders {
		if order == nil {
			continue
		}
		if order.Symbol != inst.Symbol || order.Category != category {
			continue
		}
		if order.Origin != constants.ORDER_ORIGIN_SYSTEM {
			continue
		}
		switch order.Status {
		case constants.ORDER_STATUS_NEW, constants.ORDER_STATUS_PARTIALLY_FILLED:
		default:
			continue
		}
		levelKeyValue, ok := levelByID[order.ID]
		if !ok {
			level, levelOk := m.orderLevel(inst, mark, order.Price, order.Side)
			if !levelOk {
				stale = append(stale, order)
				continue
			}
			levelKeyValue = levelKey(order.Side, level)
		}
		if prev, ok := result[levelKeyValue]; ok {
			if order.UpdatedAt > prev.UpdatedAt {
				stale = append(stale, prev)
				result[levelKeyValue] = order
			} else {
				stale = append(stale, order)
			}
			continue
		}
		result[levelKeyValue] = order
	}
	return result, stale
}

func (m *MarketMaker) orderLevel(inst *types.Instrument, mark types.Price, price types.Price, side int8) (int, bool) {
	if inst == nil || math.Sign(inst.TickSize) <= 0 {
		return 0, false
	}
	cmp := math.Cmp(price, mark)
	if side == constants.ORDER_SIDE_BUY && cmp >= 0 {
		return 0, false
	}
	if side == constants.ORDER_SIDE_SELL && cmp <= 0 {
		return 0, false
	}
	diff := math.AbsFixed(math.Sub(price, mark))
	if math.Sign(diff) <= 0 {
		return 0, false
	}
	levels := math.Div(diff, inst.TickSize)
	if math.Sign(levels) <= 0 {
		return 0, false
	}
	levelStr := levels.Round(0).String()
	level, err := strconv.Atoi(levelStr)
	if err != nil || level <= 0 || level > m.cfg.Levels {
		return 0, false
	}
	return level, true
}

func (m *MarketMaker) wouldSelfMatch(req *types.PlaceOrderRequest) bool {
	if req == nil {
		return false
	}
	book := m.eng.ReadBook(req.Category, req.Symbol)
	if book == nil {
		return false
	}
	taker := &types.Order{
		UserID:   req.UserID,
		Symbol:   req.Symbol,
		Category: req.Category,
		Side:     req.Side,
		Type:     req.Type,
		TIF:      req.TIF,
		Price:    req.Price,
		Quantity: req.Quantity,
	}
	var buf [8]types.Match
	matches := book.GetMatches(taker, req.Price, buf[:0])
	for i := range matches {
		maker := matches[i].MakerOrder
		if maker != nil && maker.UserID == req.UserID {
			return true
		}
	}
	return false
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
	qty = types.Quantity(math.RoundTo(qty, inst.StepSize))
	if math.Cmp(qty, inst.MinQty) < 0 {
		qty = inst.MinQty
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
	current := math.Zero
	if bal != nil {
		current = bal.Available
	}
	if math.Cmp(current, minBalance) >= 0 {
		return
	}
	targetBalance := m.cfg.MaxBalance
	delta := fixed.NewI(targetBalance, 0)
	delta = delta.Sub(current)
	if math.Sign(delta) <= 0 {
		return
	}
	res := m.eng.Cmd(&engine.CreateDepositCmd{
		UserID:      userID,
		Asset:       asset,
		Amount:      delta,
		Destination: "market-maker",
		CreatedBy:   types.FundingCreatedByPlatform,
		Message:     "mm seed balance",
	})
	if res.Err != nil || res.Funding == nil {
		logging.Log().Error().Err(res.Err).Str("asset", asset).Msg("mm: failed to create deposit")
		return
	}
	approve := m.eng.Cmd(&engine.ApproveFundingCmd{FundingID: res.Funding.ID})
	if approve.Err != nil {
		logging.Log().Error().Err(approve.Err).Str("asset", asset).Msg("mm: failed to approve deposit")
	}
}

func levelKey(side int8, level int) string {
	return string(rune('0'+side)) + ":" + fixed.NewI(int64(level), 0).String()
}
