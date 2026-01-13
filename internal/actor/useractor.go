package actor

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var ErrOrderNotFound = errors.New("order not found")

type UserActorState struct {
	UserID            types.UserID
	OrdersByID        map[types.OrderID]*types.Order
	OrdersByUser      map[types.UserID]map[types.OrderID]*types.Order
	Positions         map[string]int64
	ReduceOnlyByOrder map[types.OrderID]int64
	ReduceOnlyCommit  map[types.UserID]map[string]int64
	OrderLinkIds      map[types.OrderID]int64
	LinkedOrders      map[int64][]types.OrderID
	TriggerMonitor    *triggerState
}

type triggerState struct {
	orders map[types.OrderID]*types.Order
}

func NewUserActorState(userID types.UserID) *UserActorState {
	return &UserActorState{
		UserID:            userID,
		OrdersByID:        make(map[types.OrderID]*types.Order),
		OrdersByUser:      make(map[types.UserID]map[types.OrderID]*types.Order),
		Positions:         make(map[string]int64),
		ReduceOnlyByOrder: make(map[types.OrderID]int64),
		ReduceOnlyCommit:  make(map[types.UserID]map[string]int64),
		OrderLinkIds:      make(map[types.OrderID]int64),
		LinkedOrders:      make(map[int64][]types.OrderID),
		TriggerMonitor: &triggerState{
			orders: make(map[types.OrderID]*types.Order),
		},
	}
}

func HandleUserMessage(state any, msg Message) {
	s := state.(*MultiUserState)

	switch m := msg.(type) {
	case MsgPositionUpdate:
		handlePositionUpdateMulti(s, m)
	case MsgTradeExecuted:
		handleTradeExecutedMulti(s, m)
	case MsgPlaceOrder:
		handlePlaceOrderMulti(s, m)
	case MsgCancelOrder:
		handleCancelOrderMulti(s, m)
	case MsgTriggerOrder:
		handleTriggerOrderMulti(s, m)
	case MsgDeactivateOrder:
		handleDeactivateOrderMulti(s, m)
	case MsgOCOTriggered:
		handleOCOTriggeredMulti(s, m)
	case MsgGetOrder:
		handleGetOrderMulti(s, m)
	case MsgGetOrders:
		handleGetOrdersMulti(s, m)
	case MsgAddUserOrder:
		handleAddUserOrderMulti(s, m)
	}
}

func getOrCreateUserState(s *MultiUserState, userID types.UserID) *UserActorState {
	if s.Users == nil {
		s.Users = make(map[types.UserID]*UserActorState)
	}

	userState, ok := s.Users[userID]
	if !ok {
		userState = NewUserActorState(userID)
		s.Users[userID] = userState
	}
	return userState
}

func handlePositionUpdateMulti(s *MultiUserState, msg MsgPositionUpdate) {
	userState := getOrCreateUserState(s, msg.UserID)
	handlePositionUpdate(userState, msg)
}

func handleTradeExecutedMulti(s *MultiUserState, msg MsgTradeExecuted) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleTradeExecuted(userState, msg)
}

func handlePlaceOrderMulti(s *MultiUserState, msg MsgPlaceOrder) {
	userState := getOrCreateUserState(s, msg.UserID)
	handlePlaceOrder(userState, msg)
}

func handleCancelOrderMulti(s *MultiUserState, msg MsgCancelOrder) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleCancelOrder(userState, msg)
}

func handleTriggerOrderMulti(s *MultiUserState, msg MsgTriggerOrder) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleTriggerOrder(userState, msg)
}

func handleDeactivateOrderMulti(s *MultiUserState, msg MsgDeactivateOrder) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleDeactivateOrder(userState, msg)
}

func handleOCOTriggeredMulti(s *MultiUserState, msg MsgOCOTriggered) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleOCOTriggered(userState, msg)
}

func handleGetOrderMulti(s *MultiUserState, msg MsgGetOrder) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleGetOrder(userState, msg)
}

func handleGetOrdersMulti(s *MultiUserState, msg MsgGetOrders) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleGetOrders(userState, msg)
}

func handleAddUserOrderMulti(s *MultiUserState, msg MsgAddUserOrder) {
	userState := getOrCreateUserState(s, msg.UserID)
	handleAddUserOrder(userState, msg)
}

func handlePositionUpdate(s *UserActorState, msg MsgPositionUpdate) {
	allowed := absInt64(msg.NewSize)

	s.Positions[msg.Symbol] = msg.NewSize

	var reduceOnlyOrders []*types.Order
	var closeOnTriggerOrders []*types.Order

	for _, orders := range s.OrdersByUser {
		for _, order := range orders {
			if order.Symbol != msg.Symbol {
				continue
			}
			if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
				if order.ReduceOnly && order.Status == constants.ORDER_STATUS_NEW {
					reduceOnlyOrders = append(reduceOnlyOrders, order)
				}
				continue
			}
			if order.OrderLinkId <= 0 && !order.CloseOnTrigger {
				continue
			}
			if order.CloseOnTrigger && order.Quantity > 0 {
				closeOnTriggerOrders = append(closeOnTriggerOrders, order)
			}
		}
	}

	if len(reduceOnlyOrders) > 0 {
		adjustReduceOnlyOrders(s, reduceOnlyOrders, allowed)
	}

	if len(closeOnTriggerOrders) > 0 {
		adjustCloseOnTriggerOrders(s, closeOnTriggerOrders, allowed)
	}

	if msg.NewSize != 0 {
		return
	}

	for _, orders := range s.OrdersByUser {
		for _, order := range orders {
			if order.Symbol != msg.Symbol {
				continue
			}
			if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
				continue
			}
			if order.OrderLinkId <= 0 && !order.CloseOnTrigger {
				continue
			}
			delete(s.TriggerMonitor.orders, order.ID)
			order.Status = constants.ORDER_STATUS_DEACTIVATED
			order.UpdatedAt = types.NowNano()
			if order.OrderLinkId > 0 {
				delete(s.OrderLinkIds, order.ID)
				delete(s.LinkedOrders, order.OrderLinkId)
			}
		}
	}
}

func handleTradeExecuted(s *UserActorState, msg MsgTradeExecuted) {
	orderID := msg.Trade.TakerOrderID
	if msg.Trade.MakerOrderID == orderID {
		orderID = msg.Trade.MakerOrderID
	}

	order, ok := s.OrdersByID[orderID]
	if !ok {
		return
	}

	order.Filled += msg.Trade.Quantity
	if order.Filled >= order.Quantity {
		order.Status = constants.ORDER_STATUS_FILLED
		order.ClosedAt = types.NowNano()
	}

	side := msg.Side

	s.updatePositionFromTrade(s, msg.Symbol, side, int64(msg.Trade.Quantity))

	if order.Status == constants.ORDER_STATUS_FILLED {
		if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
			delete(s.ReduceOnlyByOrder, order.ID)
		}
		delete(s.OrdersByID, order.ID)
		if userOrders, ok := s.OrdersByUser[order.UserID]; ok {
			delete(userOrders, order.ID)
		}
	}
}

func handlePlaceOrder(s *UserActorState, msg MsgPlaceOrder) {
	order := &types.Order{
		ID:             types.OrderID(snowflake.Next()),
		UserID:         msg.Order.UserID,
		Symbol:         msg.Order.Symbol,
		Category:       msg.Order.Category,
		Side:           msg.Order.Side,
		Type:           msg.Order.Type,
		TIF:            msg.Order.TIF,
		Status:         constants.ORDER_STATUS_NEW,
		Price:          msg.Order.Price,
		Quantity:       msg.Order.Quantity,
		Filled:         0,
		TriggerPrice:   msg.Order.TriggerPrice,
		ReduceOnly:     msg.Order.ReduceOnly,
		CloseOnTrigger: msg.Order.CloseOnTrigger,
		StopOrderType:  msg.Order.StopOrderType,
		IsConditional:  msg.Order.IsConditional,
		OrderLinkId:    -1,
		CreatedAt:      types.NowNano(),
		UpdatedAt:      types.NowNano(),
	}

	if order.IsConditional {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		s.TriggerMonitor.orders[order.ID] = order
	}

	s.OrdersByID[order.ID] = order
	if _, ok := s.OrdersByUser[order.UserID]; !ok {
		s.OrdersByUser[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.OrdersByUser[order.UserID][order.ID] = order

	if msg.Result != nil {
		msg.Result <- &types.OrderResult{
			Orders:    []*types.Order{order},
			Filled:    0,
			Remaining: order.Quantity,
			Status:    order.Status,
		}
	}
}

func handleCancelOrder(s *UserActorState, msg MsgCancelOrder) {
	_, ok := s.OrdersByID[msg.OrderID]
	if !ok {
		if msg.Result != nil {
			msg.Result <- ErrOrderNotFound
		}
		return
	}

	order := s.OrdersByID[msg.OrderID]
	order.Status = constants.ORDER_STATUS_CANCELED
	order.ClosedAt = types.NowNano()
	order.UpdatedAt = types.NowNano()

	delete(s.OrdersByID, msg.OrderID)
	if userOrders, ok := s.OrdersByUser[order.UserID]; ok {
		delete(userOrders, msg.OrderID)
	}
	delete(s.TriggerMonitor.orders, msg.OrderID)

	if msg.Result != nil {
		msg.Result <- nil
	}
}

func handleTriggerOrder(s *UserActorState, msg MsgTriggerOrder) {
	order := msg.Order
	if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		return
	}

	order.Status = constants.ORDER_STATUS_NEW
	order.UpdatedAt = types.NowNano()
	delete(s.TriggerMonitor.orders, order.ID)
}

func handleDeactivateOrder(s *UserActorState, msg MsgDeactivateOrder) {
	order, ok := s.OrdersByID[msg.OrderID]
	if !ok {
		return
	}

	order.Status = constants.ORDER_STATUS_DEACTIVATED
	order.ClosedAt = types.NowNano()
	order.UpdatedAt = types.NowNano()

	delete(s.OrdersByID, msg.OrderID)
	if userOrders, ok := s.OrdersByUser[order.UserID]; ok {
		delete(userOrders, msg.OrderID)
	}
	delete(s.TriggerMonitor.orders, msg.OrderID)
}

func handleOCOTriggered(s *UserActorState, msg MsgOCOTriggered) {
	orders := s.LinkedOrders[int64(msg.TriggeredID)]
	for _, orderID := range orders {
		if orderID != msg.TriggeredID {
			if order, ok := s.OrdersByID[orderID]; ok {
				order.Status = constants.ORDER_STATUS_DEACTIVATED
				order.ClosedAt = types.NowNano()
				order.UpdatedAt = types.NowNano()
				delete(s.OrdersByID, orderID)
				if userOrders, ok := s.OrdersByUser[order.UserID]; ok {
					delete(userOrders, orderID)
				}
			}
		}
	}
	delete(s.LinkedOrders, int64(msg.TriggeredID))
}

func handleGetOrder(s *UserActorState, msg MsgGetOrder) {
	order, ok := s.OrdersByID[msg.OrderID]
	if msg.Result != nil {
		msg.Result <- order
	}
	_ = ok
}

func handleGetOrders(s *UserActorState, msg MsgGetOrders) {
	if msg.Result != nil {
		if userOrders, ok := s.OrdersByUser[msg.UserID]; ok {
			orders := make([]*types.Order, 0, len(userOrders))
			for _, order := range userOrders {
				orders = append(orders, order)
			}
			msg.Result <- orders
		} else {
			msg.Result <- nil
		}
	}
}

func handleAddUserOrder(s *UserActorState, msg MsgAddUserOrder) {
	s.OrdersByID[msg.Order.ID] = msg.Order
	if _, ok := s.OrdersByUser[msg.Order.UserID]; !ok {
		s.OrdersByUser[msg.Order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.OrdersByUser[msg.Order.UserID][msg.Order.ID] = msg.Order
}

func adjustReduceOnlyOrders(s *UserActorState, orders []*types.Order, allowed int64) {
	if allowed < 0 {
		allowed = 0
	}

	for _, order := range orders {
		if allowed == 0 {
			delete(s.ReduceOnlyByOrder, order.ID)
			continue
		}

		remaining := int64(order.Remaining())
		newRemaining := remaining
		if remaining > allowed {
			newRemaining = allowed
		}

		allowed -= newRemaining
		if newRemaining != remaining {
			delta := newRemaining - remaining
			order.Quantity += types.Quantity(delta)
			order.UpdatedAt = types.NowNano()
		}

		s.ReduceOnlyByOrder[order.ID] = newRemaining
	}
}

func adjustCloseOnTriggerOrders(s *UserActorState, orders []*types.Order, allowed int64) {
	if allowed < 0 {
		allowed = 0
	}

	for _, order := range orders {
		remaining := int64(order.Quantity)
		newRemaining := remaining
		if remaining > allowed {
			newRemaining = allowed
		}

		allowed -= newRemaining
		if newRemaining != remaining {
			order.Quantity = types.Quantity(newRemaining)
			order.UpdatedAt = types.NowNano()
		}
	}
}

func (s *UserActorState) updatePositionFromTrade(state *UserActorState, symbol string, side int8, qty int64) {
	sideSign := int64(1)
	if side == constants.ORDER_SIDE_SELL {
		sideSign = -1
	}

	currentSize := state.Positions[symbol]
	state.Positions[symbol] = currentSize + sideSign*qty
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
