package persistence

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func BenchmarkGOBEncodeOrder(b *testing.B) {
	order := &types.Order{
		ID:       types.OrderID(123456789),
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: 0,
		Side:     1,
		Type:     1,
		TIF:      1,
		Status:   1,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(1, 0)),
		Filled:   types.Quantity(fixed.NewI(0, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bytes.Buffer{}
		gob.NewEncoder(&buf).Encode(order)
		_ = buf.Bytes()
	}
}

func BenchmarkCustomEncodeOrder(b *testing.B) {
	order := &types.Order{
		ID:       types.OrderID(123456789),
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: 0,
		Side:     1,
		Type:     1,
		TIF:      1,
		Status:   1,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(1, 0)),
		Filled:   types.Quantity(fixed.NewI(0, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EncodeOrder(order)
	}
}

func BenchmarkGOBDecodeOrder(b *testing.B) {
	order := &types.Order{
		ID:       types.OrderID(123456789),
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: 0,
		Side:     1,
		Type:     1,
		TIF:      1,
		Status:   1,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(1, 0)),
		Filled:   types.Quantity(fixed.NewI(0, 0)),
	}

	buf := bytes.Buffer{}
	gob.NewEncoder(&buf).Encode(order)
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var o types.Order
		gob.NewDecoder(bytes.NewReader(data)).Decode(&o)
	}
}

func BenchmarkCustomDecodeOrder(b *testing.B) {
	order := &types.Order{
		ID:       types.OrderID(123456789),
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: 0,
		Side:     1,
		Type:     1,
		TIF:      1,
		Status:   1,
		Price:    types.Price(fixed.NewI(50000, 0)),
		Quantity: types.Quantity(fixed.NewI(1, 0)),
		Filled:   types.Quantity(fixed.NewI(0, 0)),
	}

	data := EncodeOrder(order)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeOrder(data)
	}
}
