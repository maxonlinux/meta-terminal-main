package oms

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
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

type Service struct {
	byUser      map[types.UserID]map[types.OrderID]*types.Order
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

func (s *Service) removeOrder(order *types.Order) {
	if order == nil {
		return
	}
	s.removeReduceOnly(order)
	s.removeConditional(order)
	userOrders := s.byUser[order.UserID]
	if userOrders != nil {
		delete(userOrders, order.ID)
		if len(userOrders) == 0 {
			delete(s.byUser, order.UserID)
		}
	}
	atomic.AddInt64(&s.count, -1)
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
	origin int8,
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
	if origin != constants.ORDER_ORIGIN_SYSTEM {
		origin = constants.ORDER_ORIGIN_USER
	}
	o.Origin = origin
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
		byUser:      make(map[types.UserID]map[types.OrderID]*types.Order),
		reduceonly:  NewReduceOnlyIndex(),
		conditional: NewConditionalIndex(),
	}
	return s
}

func (s *Service) indexUser(order *types.Order) {
	userOrders := s.byUser[order.UserID]
	if userOrders == nil {
		userOrders = make(map[types.OrderID]*types.Order)
		s.byUser[order.UserID] = userOrders
	}
	userOrders[order.ID] = order
}

func (s *Service) Load(order *types.Order) {
	if order == nil {
		return
	}
	s.indexUser(order)
	atomic.AddInt64(&s.count, 1)
	s.indexOrder(order)
}

func (s *Service) Build(
	userID types.UserID,
	symbol string,
	category int8,
	origin int8,
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
		origin,
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

func (s *Service) Add(order *types.Order) {
	if order == nil {
		return
	}
	s.indexUser(order)
	atomic.AddInt64(&s.count, 1)
	s.indexOrder(order)
}

func (s *Service) Create(
	userID types.UserID,
	symbol string,
	category int8,
	origin int8,
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
	order := newOrder(
		userID,
		symbol,
		category,
		origin,
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

	s.indexUser(order)
	atomic.AddInt64(&s.count, 1)
	s.indexOrder(order)

	return order
}

func (s *Service) GetUserOrder(userID types.UserID, id types.OrderID) (*types.Order, bool) {
	orders := s.byUser[userID]
	if orders == nil {
		return nil, false
	}
	order, ok := orders[id]
	return order, ok
}

func (s *Service) GetUserOrders(userID types.UserID) []*types.Order {
	orders := s.byUser[userID]
	if orders == nil {
		return nil
	}
	result := make([]*types.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, order)
	}
	return result
}

func (s *Service) Count() int {
	return int(atomic.LoadInt64(&s.count))
}

func (s *Service) Amend(userID types.UserID, id types.OrderID, newQty types.Quantity) error {
	order, ok := s.GetUserOrder(userID, id)
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
		s.reduceonly.adjustExposure(order.UserID, order.Symbol, order.Side, math.Neg(delta))
	}

	return nil
}

func (s *Service) Cancel(userID types.UserID, id types.OrderID) error {
	order, ok := s.GetUserOrder(userID, id)
	if !ok {
		return errors.New("order not found")
	}

	if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED && order.Status != constants.ORDER_STATUS_TRIGGERED {
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

	s.removeOrder(order)

	return nil
}

func (s *Service) CancelWithDetails(userID types.UserID, id types.OrderID) (*types.Order, error) {
	order, ok := s.GetUserOrder(userID, id)
	if !ok {
		return nil, errors.New("order not found")
	}

	if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED && order.Status != constants.ORDER_STATUS_TRIGGERED {
		return nil, errors.New("only active orders can be canceled")
	}

	if order.IsConditional {
		order.Status = constants.ORDER_STATUS_DEACTIVATED
	} else if !order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	order.UpdatedAt = utils.NowNano()
	s.removeOrder(order)
	return order, nil
}

func (s *Service) Fill(userID types.UserID, id types.OrderID, qty types.Quantity) error {
	order, ok := s.GetUserOrder(userID, id)
	if !ok {
		return errors.New("order not found")
	}

	if order.ReduceOnly {
		s.reduceonly.adjustExposure(order.UserID, order.Symbol, order.Side, math.Neg(qty))
	}

	order.Filled = math.Add(order.Filled, qty)
	order.UpdatedAt = utils.NowNano()

	if math.Cmp(order.Filled, order.Quantity) != 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		return nil
	}

	order.Status = constants.ORDER_STATUS_FILLED
	s.removeOrder(order)

	return nil
}

func (s *Service) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity, price types.Price) {
	s.reduceonly.OnPositionReduce(userID, symbol, positionSize, price)
}

func (s *Service) OnPriceTick(symbol string, price types.Price, callback func(*types.Order)) {
	s.conditional.CheckTriggers(symbol, price, callback)
}

func (s *Service) Iterate(fn func(*types.Order) bool) {
	for _, orders := range s.byUser {
		for _, order := range orders {
			if !fn(order) {
				return
			}
		}
	}
}
