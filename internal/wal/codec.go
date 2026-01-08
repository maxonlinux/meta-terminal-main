package wal

import (
	"encoding/binary"
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var ErrInvalidPayload = errors.New("invalid wal payload")

func EncodePlaceOrder(orderID types.OrderID, input *types.OrderInput) []byte {
	buf := make([]byte, 0, 128+len(input.Symbol))
	buf = appendUint64(buf, uint64(orderID))
	buf = appendUint64(buf, uint64(input.UserID))
	buf = appendString(buf, input.Symbol)
	buf = appendInt8(buf, input.Category)
	buf = appendInt8(buf, input.Side)
	buf = appendInt8(buf, input.Type)
	buf = appendInt8(buf, input.TIF)
	buf = appendInt64(buf, int64(input.Quantity))
	buf = appendInt64(buf, int64(input.Price))
	buf = appendInt64(buf, int64(input.TriggerPrice))
	buf = appendUint8(buf, boolToByte(input.ReduceOnly))
	buf = appendUint8(buf, boolToByte(input.CloseOnTrigger))
	buf = appendInt8(buf, input.StopOrderType)
	buf = appendInt8(buf, input.Leverage)
	return buf
}

func DecodePlaceOrder(payload []byte) (types.OrderID, types.OrderInput, error) {
	var input types.OrderInput
	var err error
	idx := 0
	orderID, ok := readUint64(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	userID, ok := readUint64(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.UserID = types.UserID(userID)
	input.Symbol, ok = readString(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.Category, ok = readInt8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.Side, ok = readInt8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.Type, ok = readInt8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.TIF, ok = readInt8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	qty, ok := readInt64(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.Quantity = types.Quantity(qty)
	price, ok := readInt64(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.Price = types.Price(price)
	trigger, ok := readInt64(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.TriggerPrice = types.Price(trigger)
	reduce, ok := readUint8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	closeTrig, ok := readUint8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.ReduceOnly = reduce == 1
	input.CloseOnTrigger = closeTrig == 1
	input.StopOrderType, ok = readInt8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	input.Leverage, ok = readInt8(payload, &idx)
	if !ok {
		return 0, input, ErrInvalidPayload
	}
	return types.OrderID(orderID), input, err
}

func EncodeCancelOrder(orderID types.OrderID, userID types.UserID) []byte {
	buf := make([]byte, 0, 16)
	buf = appendUint64(buf, uint64(orderID))
	buf = appendUint64(buf, uint64(userID))
	return buf
}

func DecodeCancelOrder(payload []byte) (types.OrderID, types.UserID, error) {
	idx := 0
	orderID, ok := readUint64(payload, &idx)
	if !ok {
		return 0, 0, ErrInvalidPayload
	}
	userID, ok := readUint64(payload, &idx)
	if !ok {
		return 0, 0, ErrInvalidPayload
	}
	return types.OrderID(orderID), types.UserID(userID), nil
}

func EncodePriceTick(symbol string, price types.Price) []byte {
	buf := make([]byte, 0, 16+len(symbol))
	buf = appendString(buf, symbol)
	buf = appendInt64(buf, int64(price))
	return buf
}

func DecodePriceTick(payload []byte) (string, types.Price, error) {
	idx := 0
	symbol, ok := readString(payload, &idx)
	if !ok {
		return "", 0, ErrInvalidPayload
	}
	price, ok := readInt64(payload, &idx)
	if !ok {
		return "", 0, ErrInvalidPayload
	}
	return symbol, types.Price(price), nil
}

func EncodeSetLeverage(userID types.UserID, symbol string, leverage int8) []byte {
	buf := make([]byte, 0, 16+len(symbol))
	buf = appendUint64(buf, uint64(userID))
	buf = appendString(buf, symbol)
	buf = appendInt8(buf, leverage)
	return buf
}

func DecodeSetLeverage(payload []byte) (types.UserID, string, int8, error) {
	idx := 0
	userID, ok := readUint64(payload, &idx)
	if !ok {
		return 0, "", 0, ErrInvalidPayload
	}
	symbol, ok := readString(payload, &idx)
	if !ok {
		return 0, "", 0, ErrInvalidPayload
	}
	lev, ok := readInt8(payload, &idx)
	if !ok {
		return 0, "", 0, ErrInvalidPayload
	}
	return types.UserID(userID), symbol, lev, nil
}

func EncodeSetBalance(userID types.UserID, asset string, available int64) []byte {
	buf := make([]byte, 0, 24+len(asset))
	buf = appendUint64(buf, uint64(userID))
	buf = appendString(buf, asset)
	buf = appendInt64(buf, available)
	return buf
}

func DecodeSetBalance(payload []byte) (types.UserID, string, int64, error) {
	idx := 0
	userID, ok := readUint64(payload, &idx)
	if !ok {
		return 0, "", 0, ErrInvalidPayload
	}
	asset, ok := readString(payload, &idx)
	if !ok {
		return 0, "", 0, ErrInvalidPayload
	}
	amount, ok := readInt64(payload, &idx)
	if !ok {
		return 0, "", 0, ErrInvalidPayload
	}
	return types.UserID(userID), asset, amount, nil
}

func EncodeAddInstrument(symbol, base, quote string, category int8, price types.Price) []byte {
	buf := make([]byte, 0, 64+len(symbol)+len(base)+len(quote))
	buf = appendString(buf, symbol)
	buf = appendString(buf, base)
	buf = appendString(buf, quote)
	buf = appendInt8(buf, category)
	buf = appendInt64(buf, int64(price))
	return buf
}

func DecodeAddInstrument(payload []byte) (string, string, string, int8, types.Price, error) {
	idx := 0
	sym, ok := readString(payload, &idx)
	if !ok {
		return "", "", "", 0, 0, ErrInvalidPayload
	}
	base, ok := readString(payload, &idx)
	if !ok {
		return "", "", "", 0, 0, ErrInvalidPayload
	}
	quote, ok := readString(payload, &idx)
	if !ok {
		return "", "", "", 0, 0, ErrInvalidPayload
	}
	cat, ok := readInt8(payload, &idx)
	if !ok {
		return "", "", "", 0, 0, ErrInvalidPayload
	}
	price, ok := readInt64(payload, &idx)
	if !ok {
		return "", "", "", 0, 0, ErrInvalidPayload
	}
	return sym, base, quote, cat, types.Price(price), nil
}

func appendUint64(buf []byte, v uint64) []byte {
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], v)
	return append(buf, tmp[:]...)
}

func appendInt64(buf []byte, v int64) []byte {
	return appendUint64(buf, uint64(v))
}

func appendUint8(buf []byte, v uint8) []byte {
	return append(buf, v)
}

func appendInt8(buf []byte, v int8) []byte {
	return append(buf, byte(v))
}

func appendString(buf []byte, s string) []byte {
	buf = appendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

func appendUint32(buf []byte, v uint32) []byte {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	return append(buf, tmp[:]...)
}

func readUint64(payload []byte, idx *int) (uint64, bool) {
	if *idx+8 > len(payload) {
		return 0, false
	}
	v := binary.LittleEndian.Uint64(payload[*idx:])
	*idx += 8
	return v, true
}

func readInt64(payload []byte, idx *int) (int64, bool) {
	v, ok := readUint64(payload, idx)
	return int64(v), ok
}

func readUint8(payload []byte, idx *int) (uint8, bool) {
	if *idx+1 > len(payload) {
		return 0, false
	}
	v := payload[*idx]
	*idx += 1
	return v, true
}

func readInt8(payload []byte, idx *int) (int8, bool) {
	v, ok := readUint8(payload, idx)
	return int8(v), ok
}

func readString(payload []byte, idx *int) (string, bool) {
	if *idx+4 > len(payload) {
		return "", false
	}
	length := binary.LittleEndian.Uint32(payload[*idx:])
	*idx += 4
	if *idx+int(length) > len(payload) {
		return "", false
	}
	s := string(payload[*idx : *idx+int(length)])
	*idx += int(length)
	return s, true
}

func boolToByte(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}
