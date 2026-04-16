package events

import (
	"encoding/binary"
	"errors"

	"github.com/maxonlinux/meta-terminal-go/pkg/codec"
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
	RPNLRecorded    Type = 10
)

type Event struct {
	Type Type
	Data []byte
}

type OrderAmendedEvent struct {
	UserID    types.UserID
	OrderID   types.OrderID
	NewQty    types.Quantity
	NewPrice  types.Price
	Timestamp uint64
}

type OrderCanceledEvent struct {
	UserID    types.UserID
	OrderID   types.OrderID
	Timestamp uint64
}

type TradeEvent struct {
	TradeID        types.TradeID
	MakerUserID    types.UserID
	TakerUserID    types.UserID
	MakerOrderID   types.OrderID
	TakerOrderID   types.OrderID
	MakerOrderType int8
	TakerOrderType int8
	Instrument     *types.Instrument
	Symbol         string
	Category       int8
	Price          types.Price
	Quantity       types.Quantity
	TakerSide      int8
	Timestamp      uint64
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

type OrderPlacedEvent struct {
	Order      *types.Order
	Instrument *types.Instrument
}

type OrderTriggeredEvent struct {
	UserID    types.UserID
	OrderID   types.OrderID
	Timestamp uint64
}

type RPNLEvent struct {
	UserID    types.UserID
	OrderID   types.OrderID
	Symbol    string
	Category  int8
	Side      int8
	Price     types.Price
	Quantity  types.Quantity
	Realized  types.Quantity
	Timestamp uint64
}

func EncodeOrderPlaced(ev OrderPlacedEvent) Event {
	orderBytes := codec.EncodeOrder(ev.Order)
	data := make([]byte, 0, len(orderBytes)+64)
	data = appendBytes(data, orderBytes)
	instBytes := encodeInstrument(ev.Instrument)
	data = appendBytes(data, instBytes)
	return Event{Type: OrderPlaced, Data: data}
}

func DecodeOrderPlaced(data []byte) (OrderPlacedEvent, error) {
	if len(data) < 4 {
		return OrderPlacedEvent{}, errors.New("invalid order placed payload")
	}
	off := 0
	orderBytes, err := readBytesAt(data, &off)
	if err != nil {
		return OrderPlacedEvent{}, err
	}
	order, err := codec.DecodeOrder(orderBytes)
	if err != nil {
		return OrderPlacedEvent{}, err
	}
	ev := OrderPlacedEvent{Order: order}
	instBytes, err := readBytesAt(data, &off)
	if err != nil {
		return OrderPlacedEvent{}, err
	}
	inst, err := DecodeInstrument(instBytes)
	if err != nil {
		return OrderPlacedEvent{}, err
	}
	if inst == nil {
		return OrderPlacedEvent{}, errors.New("missing order instrument")
	}
	ev.Instrument = inst
	return ev, nil
}

func EncodeOrderAmended(ev OrderAmendedEvent) Event {
	qty, _ := ev.NewQty.MarshalBinary()
	price, _ := ev.NewPrice.MarshalBinary()
	data := make([]byte, 0, 28+len(qty)+len(price))
	data = appendU64(data, uint64(ev.UserID))
	data = appendU64(data, uint64(ev.OrderID))
	data = appendU32(data, uint32(len(qty)))
	data = append(data, qty...)
	data = appendU64(data, ev.Timestamp)
	if len(price) > 0 {
		data = appendU32(data, uint32(len(price)))
		data = append(data, price...)
	}
	return Event{Type: OrderAmended, Data: data}
}

func DecodeOrderAmended(data []byte) (OrderAmendedEvent, error) {
	if len(data) < 20 {
		return OrderAmendedEvent{}, errors.New("invalid order amended payload")
	}
	off := 0
	userID := types.UserID(readU64(data, &off))
	orderID := types.OrderID(readU64(data, &off))
	qtyLen := int(readU32(data, &off))
	if qtyLen < 0 || off+qtyLen+8 > len(data) {
		return OrderAmendedEvent{}, errors.New("invalid order amended payload")
	}
	var qty types.Quantity
	if err := qty.UnmarshalBinary(data[off : off+qtyLen]); err != nil {
		return OrderAmendedEvent{}, err
	}
	off += qtyLen
	ts := readU64(data, &off)
	var price types.Price
	if off < len(data) {
		priceLen := int(readU32(data, &off))
		if priceLen < 0 || off+priceLen > len(data) {
			return OrderAmendedEvent{}, errors.New("invalid order amended payload")
		}
		if err := price.UnmarshalBinary(data[off : off+priceLen]); err != nil {
			return OrderAmendedEvent{}, err
		}
	}
	return OrderAmendedEvent{UserID: userID, OrderID: orderID, NewQty: qty, NewPrice: price, Timestamp: ts}, nil
}

func EncodeOrderCanceled(ev OrderCanceledEvent) Event {
	data := make([]byte, 0, 24)
	data = appendU64(data, uint64(ev.UserID))
	data = appendU64(data, uint64(ev.OrderID))
	data = appendU64(data, ev.Timestamp)
	return Event{Type: OrderCanceled, Data: data}
}

func DecodeOrderCanceled(data []byte) (OrderCanceledEvent, error) {
	if len(data) < 24 {
		return OrderCanceledEvent{}, errors.New("invalid order canceled payload")
	}
	off := 0
	userID := types.UserID(readU64(data, &off))
	orderID := types.OrderID(readU64(data, &off))
	ts := readU64(data, &off)
	return OrderCanceledEvent{UserID: userID, OrderID: orderID, Timestamp: ts}, nil
}

func EncodeTrade(ev TradeEvent) Event {
	priceBytes, _ := ev.Price.MarshalBinary()
	qtyBytes, _ := ev.Quantity.MarshalBinary()
	data := make([]byte, 0, 64+len(ev.Symbol)+len(priceBytes)+len(qtyBytes))
	data = appendU64(data, uint64(ev.TradeID))
	data = appendU64(data, uint64(ev.MakerUserID))
	data = appendU64(data, uint64(ev.TakerUserID))
	data = appendU64(data, uint64(ev.MakerOrderID))
	data = appendU64(data, uint64(ev.TakerOrderID))
	data = append(data, byte(ev.Category), byte(ev.TakerSide), byte(ev.MakerOrderType), byte(ev.TakerOrderType))
	data = appendU64(data, ev.Timestamp)
	data = appendString(data, ev.Symbol)
	data = appendBytes(data, priceBytes)
	data = appendBytes(data, qtyBytes)
	instBytes := encodeInstrument(ev.Instrument)
	data = appendBytes(data, instBytes)
	return Event{Type: TradeExecuted, Data: data}
}

func DecodeTrade(data []byte) (TradeEvent, error) {
	ev, instPayload, err := decodeTradePayload(data, true)
	if err != nil {
		return ev, err
	}
	inst, err := DecodeInstrument(instPayload)
	if err != nil {
		return ev, err
	}
	if inst == nil {
		return ev, errors.New("missing trade instrument")
	}
	ev.Instrument = inst
	return ev, nil
}

func DecodeTradeNoSymbolWithPayload(data []byte) (TradeEvent, []byte, error) {
	return decodeTradePayload(data, false)
}

func decodeTradePayload(data []byte, decodeSymbol bool) (TradeEvent, []byte, error) {
	var ev TradeEvent
	if len(data) < 48 {
		return ev, nil, errors.New("invalid trade payload")
	}
	off := 0
	ev.TradeID = types.TradeID(readU64(data, &off))
	ev.MakerUserID = types.UserID(readU64(data, &off))
	ev.TakerUserID = types.UserID(readU64(data, &off))
	ev.MakerOrderID = types.OrderID(readU64(data, &off))
	ev.TakerOrderID = types.OrderID(readU64(data, &off))
	ev.Category = int8(data[off])
	ev.TakerSide = int8(data[off+1])
	ev.MakerOrderType = int8(data[off+2])
	ev.TakerOrderType = int8(data[off+3])
	off += 4
	ev.Timestamp = readU64(data, &off)
	if decodeSymbol {
		symbol, err := readStringAt(data, &off)
		if err != nil {
			return ev, nil, err
		}
		ev.Symbol = symbol
	} else {
		if err := skipStringAt(data, &off); err != nil {
			return ev, nil, err
		}
	}
	priceBytes, err := readBytesAt(data, &off)
	if err != nil {
		return ev, nil, err
	}
	qtyBytes, err := readBytesAt(data, &off)
	if err != nil {
		return ev, nil, err
	}
	if err := ev.Price.UnmarshalBinary(priceBytes); err != nil {
		return ev, nil, err
	}
	if err := ev.Quantity.UnmarshalBinary(qtyBytes); err != nil {
		return ev, nil, err
	}
	instBytes, err := readBytesAt(data, &off)
	if err != nil {
		return ev, nil, err
	}
	if len(instBytes) == 0 {
		return ev, nil, errors.New("missing trade instrument")
	}
	return ev, instBytes, nil
}

func skipStringAt(data []byte, off *int) error {
	if *off+4 > len(data) {
		return errors.New("invalid string payload")
	}
	length := int(readU32(data, off))
	if length < 0 || *off+length > len(data) {
		return errors.New("invalid string payload")
	}
	*off += length
	return nil
}

func EncodeLeverage(ev LeverageEvent) Event {
	lev, _ := ev.Leverage.MarshalBinary()
	data := make([]byte, 0, 16+len(ev.Symbol)+len(lev))
	data = appendU64(data, uint64(ev.UserID))
	data = appendString(data, ev.Symbol)
	data = appendBytes(data, lev)
	return Event{Type: LeverageSet, Data: data}
}

func DecodeLeverage(data []byte) (LeverageEvent, error) {
	var ev LeverageEvent
	if len(data) < 8 {
		return ev, errors.New("invalid leverage payload")
	}
	off := 0
	ev.UserID = types.UserID(readU64(data, &off))
	symbol, err := readStringAt(data, &off)
	if err != nil {
		return ev, err
	}
	ev.Symbol = symbol
	levBytes, err := readBytesAt(data, &off)
	if err != nil {
		return ev, err
	}
	if err := ev.Leverage.UnmarshalBinary(levBytes); err != nil {
		return ev, err
	}
	return ev, nil
}

func EncodeFundingCreated(req types.FundingRequest) Event {
	data := codec.EncodeFunding(&req)
	return Event{Type: FundingCreated, Data: data}
}

func DecodeFundingCreated(data []byte) (*types.FundingRequest, error) {
	return codec.DecodeFunding(data)
}

func EncodeFundingStatus(t Type, id types.FundingID) Event {
	data := make([]byte, 0, 8)
	data = appendU64(data, uint64(id))
	return Event{Type: t, Data: data}
}

func DecodeFundingStatus(data []byte) (FundingStatusEvent, error) {
	if len(data) < 8 {
		return FundingStatusEvent{}, errors.New("invalid funding status payload")
	}
	off := 0
	id := types.FundingID(readU64(data, &off))
	return FundingStatusEvent{FundingID: id}, nil
}

func EncodeOrderTriggered(ev OrderTriggeredEvent) Event {
	data := make([]byte, 0, 24)
	data = appendU64(data, uint64(ev.UserID))
	data = appendU64(data, uint64(ev.OrderID))
	data = appendU64(data, ev.Timestamp)
	return Event{Type: OrderTriggered, Data: data}
}

func EncodeRPNL(ev RPNLEvent) Event {
	// Encodes realized PnL events for persistence history.
	priceBytes, _ := ev.Price.MarshalBinary()
	qtyBytes, _ := ev.Quantity.MarshalBinary()
	rpnlBytes, _ := ev.Realized.MarshalBinary()
	data := make([]byte, 0, 64+len(ev.Symbol)+len(priceBytes)+len(qtyBytes)+len(rpnlBytes))
	data = appendU64(data, uint64(ev.UserID))
	data = appendU64(data, uint64(ev.OrderID))
	data = appendU64(data, ev.Timestamp)
	data = appendU32(data, uint32(ev.Category))
	data = appendU32(data, uint32(ev.Side))
	data = appendU32(data, uint32(len(ev.Symbol)))
	data = append(data, []byte(ev.Symbol)...)
	data = appendU32(data, uint32(len(priceBytes)))
	data = append(data, priceBytes...)
	data = appendU32(data, uint32(len(qtyBytes)))
	data = append(data, qtyBytes...)
	data = appendU32(data, uint32(len(rpnlBytes)))
	data = append(data, rpnlBytes...)
	return Event{Type: RPNLRecorded, Data: data}
}

func DecodeRPNL(data []byte) (RPNLEvent, error) {
	// Decodes realized PnL events from the outbox stream.
	if len(data) < 32 {
		return RPNLEvent{}, errors.New("invalid rpnl payload")
	}
	off := 0
	userID := types.UserID(readU64(data, &off))
	orderID := types.OrderID(readU64(data, &off))
	ts := readU64(data, &off)
	category := int8(readU32(data, &off))
	side := int8(readU32(data, &off))
	nameLen := int(readU32(data, &off))
	if nameLen < 0 || off+nameLen > len(data) {
		return RPNLEvent{}, errors.New("invalid rpnl payload")
	}
	symbol := string(data[off : off+nameLen])
	off += nameLen
	priceLen := int(readU32(data, &off))
	if priceLen < 0 || off+priceLen > len(data) {
		return RPNLEvent{}, errors.New("invalid rpnl payload")
	}
	var price types.Price
	if err := price.UnmarshalBinary(data[off : off+priceLen]); err != nil {
		return RPNLEvent{}, err
	}
	off += priceLen
	qtyLen := int(readU32(data, &off))
	if qtyLen < 0 || off+qtyLen > len(data) {
		return RPNLEvent{}, errors.New("invalid rpnl payload")
	}
	var qty types.Quantity
	if err := qty.UnmarshalBinary(data[off : off+qtyLen]); err != nil {
		return RPNLEvent{}, err
	}
	off += qtyLen
	rpnlLen := int(readU32(data, &off))
	if rpnlLen < 0 || off+rpnlLen > len(data) {
		return RPNLEvent{}, errors.New("invalid rpnl payload")
	}
	var rpnl types.Quantity
	if err := rpnl.UnmarshalBinary(data[off : off+rpnlLen]); err != nil {
		return RPNLEvent{}, err
	}
	return RPNLEvent{
		UserID:    userID,
		OrderID:   orderID,
		Symbol:    symbol,
		Category:  category,
		Side:      side,
		Price:     price,
		Quantity:  qty,
		Realized:  rpnl,
		Timestamp: ts,
	}, nil
}

func DecodeOrderTriggered(data []byte) (OrderTriggeredEvent, error) {
	if len(data) < 24 {
		return OrderTriggeredEvent{}, errors.New("invalid order triggered payload")
	}
	off := 0
	userID := types.UserID(readU64(data, &off))
	orderID := types.OrderID(readU64(data, &off))
	ts := readU64(data, &off)
	return OrderTriggeredEvent{UserID: userID, OrderID: orderID, Timestamp: ts}, nil
}

func appendU64(dst []byte, v uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	return append(dst, buf[:]...)
}

func appendU32(dst []byte, v uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	return append(dst, buf[:]...)
}

func appendString(dst []byte, s string) []byte {
	dst = appendU32(dst, uint32(len(s)))
	return append(dst, s...)
}

func appendBytes(dst []byte, data []byte) []byte {
	dst = appendU32(dst, uint32(len(data)))
	return append(dst, data...)
}

func readU64(data []byte, off *int) uint64 {
	v := binary.BigEndian.Uint64(data[*off : *off+8])
	*off += 8
	return v
}

func readU32(data []byte, off *int) uint32 {
	v := binary.BigEndian.Uint32(data[*off : *off+4])
	*off += 4
	return v
}

func readStringAt(data []byte, off *int) (string, error) {
	if *off+4 > len(data) {
		return "", errors.New("invalid string payload")
	}
	length := int(readU32(data, off))
	if length == 0 {
		return "", nil
	}
	if *off+length > len(data) {
		return "", errors.New("invalid string payload")
	}
	start := *off
	*off += length
	return string(data[start:*off]), nil
}

func readBytesAt(data []byte, off *int) ([]byte, error) {
	if *off+4 > len(data) {
		return nil, errors.New("invalid bytes payload")
	}
	length := int(readU32(data, off))
	if length == 0 {
		return nil, nil
	}
	if *off+length > len(data) {
		return nil, errors.New("invalid bytes payload")
	}
	start := *off
	*off += length
	return data[start:*off], nil
}

func encodeInstrument(inst *types.Instrument) []byte {
	if inst == nil {
		return nil
	}
	minQty, _ := inst.MinQty.MarshalBinary()
	minNotional, _ := inst.MinNotional.MarshalBinary()
	tickSize, _ := inst.TickSize.MarshalBinary()
	stepSize, _ := inst.StepSize.MarshalBinary()
	data := make([]byte, 0, 64+len(inst.Symbol)+len(inst.BaseAsset)+len(inst.QuoteAsset)+len(inst.AssetType)+len(minQty)+len(minNotional)+len(tickSize)+len(stepSize))
	data = appendString(data, inst.Symbol)
	data = appendString(data, inst.BaseAsset)
	data = appendString(data, inst.QuoteAsset)
	data = appendString(data, inst.AssetType)
	data = append(data, byte(inst.PricePrec), byte(inst.QtyPrec))
	data = appendBytes(data, minQty)
	data = appendBytes(data, minNotional)
	data = appendBytes(data, tickSize)
	data = appendBytes(data, stepSize)
	return data
}

func DecodeInstrument(data []byte) (*types.Instrument, error) {
	if len(data) == 0 {
		return nil, nil
	}
	off := 0
	symbol, err := readStringAt(data, &off)
	if err != nil {
		return nil, err
	}
	base, err := readStringAt(data, &off)
	if err != nil {
		return nil, err
	}
	quote, err := readStringAt(data, &off)
	if err != nil {
		return nil, err
	}
	assetType, err := readStringAt(data, &off)
	if err != nil {
		return nil, err
	}
	if off+2 > len(data) {
		return nil, errors.New("invalid instrument payload")
	}
	pricePrec := int8(data[off])
	qtyPrec := int8(data[off+1])
	off += 2
	minQtyBytes, err := readBytesAt(data, &off)
	if err != nil {
		return nil, err
	}
	minNotionalBytes, err := readBytesAt(data, &off)
	if err != nil {
		return nil, err
	}
	tickBytes, err := readBytesAt(data, &off)
	if err != nil {
		return nil, err
	}
	stepBytes, err := readBytesAt(data, &off)
	if err != nil {
		return nil, err
	}
	inst := &types.Instrument{
		Symbol:     symbol,
		BaseAsset:  base,
		QuoteAsset: quote,
		AssetType:  assetType,
		PricePrec:  pricePrec,
		QtyPrec:    qtyPrec,
	}
	if len(minQtyBytes) > 0 {
		if err := inst.MinQty.UnmarshalBinary(minQtyBytes); err != nil {
			return nil, err
		}
	}
	if len(minNotionalBytes) > 0 {
		if err := inst.MinNotional.UnmarshalBinary(minNotionalBytes); err != nil {
			return nil, err
		}
	}
	if len(tickBytes) > 0 {
		if err := inst.TickSize.UnmarshalBinary(tickBytes); err != nil {
			return nil, err
		}
	}
	if len(stepBytes) > 0 {
		if err := inst.StepSize.UnmarshalBinary(stepBytes); err != nil {
			return nil, err
		}
	}
	return inst, nil
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
	case OrderTriggered:
		return DecodeOrderTriggered(event.Data)
	case RPNLRecorded:
		return DecodeRPNL(event.Data)
	default:
		return nil, errors.New("unknown event type")
	}
}
