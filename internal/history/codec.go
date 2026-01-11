package history

import (
	"bytes"
	"encoding/gob"
	"errors"
)

var errUnknownKind = errors.New("history: unknown record kind")

type Record struct {
	Kind        byte
	OrderClosed *OrderClosed
	Trade       *Trade
	PnL         *PnL
}

func EncodeOrderClosed(e *OrderClosed) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodeOrderClosed(b []byte) (*OrderClosed, error) {
	var e OrderClosed
	dec := gob.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

func EncodeTrade(e *Trade) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodeTrade(b []byte) (*Trade, error) {
	var e Trade
	dec := gob.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

func EncodePnL(e *PnL) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodePnL(b []byte) (*PnL, error) {
	var e PnL
	dec := gob.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

func DecodeRecord(kind byte, payload []byte) (Record, error) {
	switch kind {
	case KIND_ORDER_CLOSED:
		e, err := DecodeOrderClosed(payload)
		if err != nil {
			return Record{}, err
		}
		return Record{Kind: kind, OrderClosed: e}, nil
	case KIND_TRADE:
		e, err := DecodeTrade(payload)
		if err != nil {
			return Record{}, err
		}
		return Record{Kind: kind, Trade: e}, nil
	case KIND_PNL:
		e, err := DecodePnL(payload)
		if err != nil {
			return Record{}, err
		}
		return Record{Kind: kind, PnL: e}, nil
	default:
		return Record{}, errUnknownKind
	}
}
