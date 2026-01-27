package events

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type Type byte

const (
	OrderPlaced     Type = 1
	OrderAmended    Type = 2
	OrderCanceled   Type = 3
	TradeExecuted   Type = 4
	LeverageSet     Type = 5
	FundingCreated  Type = 6
	FundingApproved Type = 7
	FundingRejected Type = 8
	OrderTriggered  Type = 9
)

type Event struct {
	Type Type
	Data []byte
}

type OrderAmendedEvent struct {
	UserID  types.UserID
	OrderID types.OrderID
	NewQty  types.Quantity
}

type OrderCanceledEvent struct {
	UserID  types.UserID
	OrderID types.OrderID
}

type TradeEvent struct {
	TradeID      types.TradeID
	MakerUserID  types.UserID
	TakerUserID  types.UserID
	MakerOrderID types.OrderID
	TakerOrderID types.OrderID
	Symbol       string
	Category     int8
	Price        types.Price
	Quantity     types.Quantity
	TakerSide    int8
	Timestamp    uint64
}

type LeverageEvent struct {
	UserID   types.UserID
	Symbol   string
	Leverage types.Leverage
}

type FundingEvent struct {
	Request types.FundingRequest
}

type FundingStatusEvent struct {
	FundingID types.FundingID
}

type OrderTriggeredEvent struct {
	UserID    types.UserID
	OrderID   types.OrderID
	Timestamp uint64
}

func EncodeOrderPlaced(order *types.Order) Event {
	return Event{Type: OrderPlaced, Data: persistence.EncodeOrder(order)}
}

func DecodeOrderPlaced(data []byte) (*types.Order, error) {
	return persistence.DecodeOrder(data)
}

func EncodeOrderAmended(ev OrderAmendedEvent) Event {
	buf := bytes.NewBuffer(make([]byte, 0, 40))
	_ = binary.Write(buf, binary.BigEndian, ev.UserID)
	_ = binary.Write(buf, binary.BigEndian, ev.OrderID)
	qty, _ := ev.NewQty.MarshalBinary()
	_ = binary.Write(buf, binary.BigEndian, uint32(len(qty)))
	_, _ = buf.Write(qty)
	return Event{Type: OrderAmended, Data: buf.Bytes()}
}

func DecodeOrderAmended(data []byte) (OrderAmendedEvent, error) {
	buf := bytes.NewReader(data)
	var userID types.UserID
	var id types.OrderID
	if err := binary.Read(buf, binary.BigEndian, &userID); err != nil {
		return OrderAmendedEvent{}, err
	}
	if err := binary.Read(buf, binary.BigEndian, &id); err != nil {
		return OrderAmendedEvent{}, err
	}
	var qtyLen uint32
	if err := binary.Read(buf, binary.BigEndian, &qtyLen); err != nil {
		return OrderAmendedEvent{}, err
	}
	qtyBytes := make([]byte, qtyLen)
	if _, err := buf.Read(qtyBytes); err != nil {
		return OrderAmendedEvent{}, err
	}
	var qty types.Quantity
	if err := qty.UnmarshalBinary(qtyBytes); err != nil {
		return OrderAmendedEvent{}, err
	}
	return OrderAmendedEvent{UserID: userID, OrderID: id, NewQty: qty}, nil
}

func EncodeOrderCanceled(ev OrderCanceledEvent) Event {
	buf := bytes.NewBuffer(make([]byte, 0, 24))
	_ = binary.Write(buf, binary.BigEndian, ev.UserID)
	_ = binary.Write(buf, binary.BigEndian, ev.OrderID)
	return Event{Type: OrderCanceled, Data: buf.Bytes()}
}

func DecodeOrderCanceled(data []byte) (OrderCanceledEvent, error) {
	buf := bytes.NewReader(data)
	var userID types.UserID
	var id types.OrderID
	if err := binary.Read(buf, binary.BigEndian, &userID); err != nil {
		return OrderCanceledEvent{}, err
	}
	if err := binary.Read(buf, binary.BigEndian, &id); err != nil {
		return OrderCanceledEvent{}, err
	}
	return OrderCanceledEvent{UserID: userID, OrderID: id}, nil
}

func EncodeTrade(ev TradeEvent) Event {
	buf := bytes.NewBuffer(make([]byte, 0, 80))
	_ = binary.Write(buf, binary.BigEndian, ev.TradeID)
	_ = binary.Write(buf, binary.BigEndian, ev.MakerUserID)
	_ = binary.Write(buf, binary.BigEndian, ev.TakerUserID)
	_ = binary.Write(buf, binary.BigEndian, ev.MakerOrderID)
	_ = binary.Write(buf, binary.BigEndian, ev.TakerOrderID)
	buf.WriteByte(byte(ev.Category))
	buf.WriteByte(byte(ev.TakerSide))
	_ = binary.Write(buf, binary.BigEndian, ev.Timestamp)
	writeString(buf, ev.Symbol)
	priceBytes, _ := ev.Price.MarshalBinary()
	qtyBytes, _ := ev.Quantity.MarshalBinary()
	writeBytes(buf, priceBytes)
	writeBytes(buf, qtyBytes)
	return Event{Type: TradeExecuted, Data: buf.Bytes()}
}

func DecodeTrade(data []byte) (TradeEvent, error) {
	buf := bytes.NewReader(data)
	var ev TradeEvent
	if err := binary.Read(buf, binary.BigEndian, &ev.TradeID); err != nil {
		return ev, err
	}
	if err := binary.Read(buf, binary.BigEndian, &ev.MakerUserID); err != nil {
		return ev, err
	}
	if err := binary.Read(buf, binary.BigEndian, &ev.TakerUserID); err != nil {
		return ev, err
	}
	if err := binary.Read(buf, binary.BigEndian, &ev.MakerOrderID); err != nil {
		return ev, err
	}
	if err := binary.Read(buf, binary.BigEndian, &ev.TakerOrderID); err != nil {
		return ev, err
	}
	category, err := buf.ReadByte()
	if err != nil {
		return ev, err
	}
	TakerSide, err := buf.ReadByte()
	if err != nil {
		return ev, err
	}
	ev.Category = int8(category)
	ev.TakerSide = int8(TakerSide)
	if err := binary.Read(buf, binary.BigEndian, &ev.Timestamp); err != nil {
		return ev, err
	}
	symbol, err := readString(buf)
	if err != nil {
		return ev, err
	}
	ev.Symbol = symbol
	priceBytes, err := readBytes(buf)
	if err != nil {
		return ev, err
	}
	qtyBytes, err := readBytes(buf)
	if err != nil {
		return ev, err
	}
	if err := ev.Price.UnmarshalBinary(priceBytes); err != nil {
		return ev, err
	}
	if err := ev.Quantity.UnmarshalBinary(qtyBytes); err != nil {
		return ev, err
	}
	return ev, nil
}

func EncodeLeverage(ev LeverageEvent) Event {
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	_ = binary.Write(buf, binary.BigEndian, ev.UserID)
	writeString(buf, ev.Symbol)
	lev, _ := ev.Leverage.MarshalBinary()
	writeBytes(buf, lev)
	return Event{Type: LeverageSet, Data: buf.Bytes()}
}

func DecodeLeverage(data []byte) (LeverageEvent, error) {
	buf := bytes.NewReader(data)
	var ev LeverageEvent
	if err := binary.Read(buf, binary.BigEndian, &ev.UserID); err != nil {
		return ev, err
	}
	symbol, err := readString(buf)
	if err != nil {
		return ev, err
	}
	ev.Symbol = symbol
	levBytes, err := readBytes(buf)
	if err != nil {
		return ev, err
	}
	if err := ev.Leverage.UnmarshalBinary(levBytes); err != nil {
		return ev, err
	}
	return ev, nil
}

func EncodeFundingCreated(req types.FundingRequest) Event {
	data := persistence.EncodeFunding(&req)
	return Event{Type: FundingCreated, Data: data}
}

func DecodeFundingCreated(data []byte) (*types.FundingRequest, error) {
	return persistence.DecodeFunding(data)
}

func EncodeFundingStatus(t Type, id types.FundingID) Event {
	buf := bytes.NewBuffer(make([]byte, 0, 16))
	_ = binary.Write(buf, binary.BigEndian, id)
	return Event{Type: t, Data: buf.Bytes()}
}

func DecodeFundingStatus(data []byte) (FundingStatusEvent, error) {
	buf := bytes.NewReader(data)
	var id types.FundingID
	if err := binary.Read(buf, binary.BigEndian, &id); err != nil {
		return FundingStatusEvent{}, err
	}
	return FundingStatusEvent{FundingID: id}, nil
}

func EncodeOrderTriggered(ev OrderTriggeredEvent) Event {
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	_ = binary.Write(buf, binary.BigEndian, ev.UserID)
	_ = binary.Write(buf, binary.BigEndian, ev.OrderID)
	_ = binary.Write(buf, binary.BigEndian, ev.Timestamp)
	return Event{Type: OrderTriggered, Data: buf.Bytes()}
}

func DecodeOrderTriggered(data []byte) (OrderTriggeredEvent, error) {
	buf := bytes.NewReader(data)
	var ev OrderTriggeredEvent
	if err := binary.Read(buf, binary.BigEndian, &ev.UserID); err != nil {
		return ev, err
	}
	if err := binary.Read(buf, binary.BigEndian, &ev.OrderID); err != nil {
		return ev, err
	}
	if err := binary.Read(buf, binary.BigEndian, &ev.Timestamp); err != nil {
		return ev, err
	}
	return ev, nil
}

func writeString(buf *bytes.Buffer, value string) {
	writeBytes(buf, []byte(value))
}

func writeBytes(buf *bytes.Buffer, value []byte) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(value)))
	_, _ = buf.Write(value)
}

func readString(buf *bytes.Reader) (string, error) {
	b, err := readBytes(buf)
	return string(b), err
}

func readBytes(buf *bytes.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(buf, binary.BigEndian, &size); err != nil {
		return nil, err
	}
	if size == 0 {
		return nil, nil
	}
	data := make([]byte, size)
	if _, err := buf.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}

func DecodeEvent(event Event) (interface{}, error) {
	switch event.Type {
	case OrderPlaced:
		return DecodeOrderPlaced(event.Data)
	case OrderAmended:
		return DecodeOrderAmended(event.Data)
	case OrderCanceled:
		return DecodeOrderCanceled(event.Data)
	case TradeExecuted:
		return DecodeTrade(event.Data)
	case LeverageSet:
		return DecodeLeverage(event.Data)
	case FundingCreated:
		return DecodeFundingCreated(event.Data)
	case FundingApproved, FundingRejected:
		return DecodeFundingStatus(event.Data)
	default:
		return nil, errors.New("unknown event type")
	}
}
