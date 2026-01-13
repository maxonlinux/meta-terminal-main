package actor

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type MatchingActorState struct {
	OrderBooks  map[int8]map[string]*orderbook.OrderBook
	SymbolToCat map[string]int8
}

func NewMatchingActorState() *MatchingActorState {
	return &MatchingActorState{
		OrderBooks: map[int8]map[string]*orderbook.OrderBook{
			constants.CATEGORY_SPOT:   make(map[string]*orderbook.OrderBook),
			constants.CATEGORY_LINEAR: make(map[string]*orderbook.OrderBook),
		},
		SymbolToCat: make(map[string]int8),
	}
}

func (s *MatchingActorState) GetOrderBook(category int8, symbol string) *orderbook.OrderBook {
	catMap := s.OrderBooks[category]
	if catMap == nil {
		catMap = make(map[string]*orderbook.OrderBook)
		s.OrderBooks[category] = catMap
	}
	ob, ok := catMap[symbol]
	if !ok {
		ob = orderbook.NewWithIDGenerator(nil)
		catMap[symbol] = ob
		s.SymbolToCat[symbol] = category
	}
	return ob
}

func HandleMatchingMessage(state any, msg Message) {
	s := state.(*MatchingActorState)

	switch m := msg.(type) {
	case MsgMatchingRequest:
		handleMatchingRequest(s, m)
	case MsgPlaceOrder:
		handleMatchingPlaceOrder(s, m)
	}
}

func handleMatchingRequest(s *MatchingActorState, msg MsgMatchingRequest) {
	order := msg.Order
	ob := s.GetOrderBook(order.Category, order.Symbol)

	var limitPrice types.Price
	if order.Type == constants.ORDER_TYPE_LIMIT {
		limitPrice = order.Price
	}

	var trades []types.Trade
	matches, err := ob.MatchInto(order, limitPrice, nil)
	if err != nil {
		if msg.ResultChan != nil {
			msg.ResultChan <- &MatchingResult{Trades: nil, Error: err}
		}
		return
	}

	for i := range matches {
		trades = append(trades, matches[i].Trade)
	}

	if msg.ResultChan != nil {
		msg.ResultChan <- &MatchingResult{Trades: trades, Error: nil}
	}
}

func handleMatchingPlaceOrder(s *MatchingActorState, msg MsgPlaceOrder) {
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
	}

	if !order.IsConditional && order.Remaining() > 0 {
		ob := s.GetOrderBook(order.Category, order.Symbol)
		ob.Add(order)
	}

	if msg.Result != nil {
		msg.Result <- &types.OrderResult{
			Orders:    []*types.Order{order},
			Filled:    0,
			Remaining: order.Quantity,
			Status:    order.Status,
		}
	}
}
