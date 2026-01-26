package oms

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

var orderPool = sync.Pool{
	New: func() interface{} {
		return &types.Order{}
	},
}

func getOrder() *types.Order {
	return orderPool.Get().(*types.Order)
}

func putOrder(o *types.Order) {
	o.ID = 0
	o.UserID = 0
	o.Symbol = ""
	o.Category = 0
	o.Side = 0
	o.Type = 0
	o.TIF = 0
	o.Status = 0
	o.Price = math.Zero
	o.Quantity = math.Zero
	o.Filled = math.Zero
	o.TriggerPrice = math.Zero
	o.ReduceOnly = false
	o.CloseOnTrigger = false
	o.StopOrderType = 0
	o.IsConditional = false
	o.CreatedAt = 0
	o.UpdatedAt = 0
	orderPool.Put(o)
}

type Service struct {
	all         map[types.OrderID]*types.Order
	count       int64
	reduceonly  *ReduceOnlyIndex
	conditional *ConditionalIndex
}

func (s *Service) removeReduceOnly(order *types.Order) {
	if order.ReduceOnly {
		s.reduceonly.Remove(order)
	}
}

func (s *Service) removeConditional(order *types.Order) {
	if order.IsConditional {
		s.conditional.Remove(order)
	}
}

func (s *Service) indexOrder(order *types.Order) {
	if order.ReduceOnly {
		s.reduceonly.Add(order)
	}
	if order.IsConditional {
		s.conditional.Add(order)
	}
}

func newOrder(
	userID types.UserID,
	symbol string,
	category int8,
	side int8,
	otype int8,
	tif int8,
	price types.Price,
	quantity types.Quantity,
	triggerPrice types.Price,
	reduceOnly bool,
	closeOnTrigger bool,
	stopOrderType int8,
	now uint64,
) *types.Order {
	o := getOrder()
	o.ID = types.OrderID(snowflake.Next())
	o.UserID = userID
	o.Symbol = symbol
	o.Category = category
	o.Side = side
	o.Type = otype
	o.TIF = tif
	o.Status = int8(constants.ORDER_STATUS_NEW)
	if !triggerPrice.IsZero() {
		o.Status = constants.ORDER_STATUS_UNTRIGGERED
	}
	o.Price = price
	o.Quantity = quantity
	o.Filled = math.Zero
	o.TriggerPrice = triggerPrice
	o.ReduceOnly = reduceOnly
	o.CloseOnTrigger = closeOnTrigger
	o.StopOrderType = stopOrderType
	o.IsConditional = !triggerPrice.IsZero()
	o.CreatedAt = now
	o.UpdatedAt = now
	return o
}

func NewService() *Service {
	s := &Service{
		all:         make(map[types.OrderID]*types.Order),
		reduceonly:  NewReduceOnlyIndex(),
		conditional: NewConditionalIndex(),
	}
	return s
}

func (s *Service) Load(order *types.Order) {
	if order == nil {
		return
	}
	s.all[order.ID] = order
	atomic.AddInt64(&s.count, 1)
	s.indexOrder(order)
}

func (s *Service) Build(
	userID types.UserID,
	symbol string,
	category int8,
	side int8,
	otype int8,
	tif int8,
	price types.Price,
	quantity types.Quantity,
	triggerPrice types.Price,
	reduceOnly bool,
	closeOnTrigger bool,
	stopOrderType int8,
) *types.Order {
	now := utils.NowNano()
	return newOrder(
		userID,
		symbol,
		category,
		side,
		otype,
		tif,
		price,
		quantity,
		triggerPrice,
		reduceOnly,
		closeOnTrigger,
		stopOrderType,
		now,
	)
}

func (s *Service) Add(order *types.Order, writer outbox.Writer) {
	if order == nil {
		return
	}
	s.all[order.ID] = order
	atomic.AddInt64(&s.count, 1)
	s.indexOrder(order)
	if writer != nil {
		_ = writer.SaveOrder(order)
	}
}

func (s *Service) Create(
	userID types.UserID,
	symbol string,
	category int8,
	side int8,
	otype int8,
	tif int8,
	price types.Price,
	quantity types.Quantity,
	triggerPrice types.Price,
	reduceOnly bool,
	closeOnTrigger bool,
	stopOrderType int8,
	writer outbox.Writer,
) *types.Order {
	now := utils.NowNano()
	order := newOrder(
		userID,
		symbol,
		category,
		side,
		otype,
		tif,
		price,
		quantity,
		triggerPrice,
		reduceOnly,
		closeOnTrigger,
		stopOrderType,
		now,
	)

	s.all[order.ID] = order
	atomic.AddInt64(&s.count, 1)
	s.indexOrder(order)

	if writer != nil {
		writer.SaveOrder(order)
	}

	return order
}

func (s *Service) Get(id types.OrderID) (*types.Order, bool) {
	order, ok := s.all[id]
	if !ok {
		return nil, false
	}
	return order, true
}

func (s *Service) Count() int {
	return int(atomic.LoadInt64(&s.count))
}

func (s *Service) Amend(id types.OrderID, newQty types.Quantity, writer outbox.Writer) error {
	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	if math.Cmp(newQty, order.Quantity) >= 0 {
		return errors.New("new quantity must be less than current")
	}

	remaining := math.Sub(order.Quantity, order.Filled)
	delta := math.Sub(remaining, newQty)

	order.Quantity = newQty
	order.UpdatedAt = utils.NowNano()

	if order.ReduceOnly {
		s.reduceonly.adjustExposure(order.UserID, order.Symbol, math.Neg(delta))
	}

	s.all[id] = order

	if writer != nil {
		writer.SaveOrder(order)
	}

	return nil
}

func (s *Service) Cancel(id types.OrderID, writer outbox.Writer) error {
	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		return errors.New("only active orders can be canceled")
	}

	if order.IsConditional {
		order.Status = constants.ORDER_STATUS_DEACTIVATED
	} else if !order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	now := utils.NowNano()
	order.UpdatedAt = now

	s.removeReduceOnly(order)
	s.removeConditional(order)

	s.all[id] = order

	if writer != nil {
		writer.SaveOrder(order)
	}

	return nil
}

func (s *Service) Fill(id types.OrderID, qty types.Quantity, writer outbox.Writer) error {
	order, ok := s.all[id]
	if !ok {
		return errors.New("order not found")
	}

	s.reduceonly.adjustExposure(order.UserID, order.Symbol, math.Neg(qty))

	order.Filled = math.Add(order.Filled, qty)
	order.UpdatedAt = utils.NowNano()

	if math.Cmp(order.Filled, order.Quantity) != 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		s.all[id] = order
		if writer != nil {
			writer.SaveOrder(order)
		}
		return nil
	}

	order.Status = constants.ORDER_STATUS_FILLED

	s.removeReduceOnly(order)

	s.all[id] = order
	if writer != nil {
		writer.SaveOrder(order)
	}

	return nil
}

func (s *Service) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity) {
	s.reduceonly.OnPositionReduce(userID, symbol, positionSize)
}

func (s *Service) OnPriceTick(symbol string, price types.Price, callback func(*types.Order)) {
	s.conditional.CheckTriggers(symbol, price, callback)
}

func (s *Service) Iterate(fn func(*types.Order) bool) {
	for _, order := range s.all {
		if !fn(order) {
			return
		}
	}
}
