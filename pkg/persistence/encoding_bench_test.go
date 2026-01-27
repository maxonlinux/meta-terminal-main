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
		if err := gob.NewEncoder(&buf).Encode(order); err != nil {
			b.Fatal(err)
		}
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
	if err := gob.NewEncoder(&buf).Encode(order); err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var o types.Order
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&o); err != nil {
			b.Fatal(err)
		}
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
