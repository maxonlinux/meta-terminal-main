package engine

import (
	"sync"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/robaho/fixed"
)

var (
	price50000   = types.Price(fixed.NewI(50000, 0))
	qty1         = types.Quantity(fixed.NewI(1, 0))
	priceOffset  = make([]types.Price, 1000)
	orderCmdPool = sync.Pool{
		New: func() interface{} {
			return &PlaceOrderCmd{}
		},
	}
)

func init() {
	for i := 0; i < 1000; i++ {
		priceOffset[i] = types.Price(fixed.NewI(50000+int64(i%1000), 0))
	}
}

func BenchmarkEngineThroughputWithPersistence(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	userStore, err := users.NewSQLiteStore(b.TempDir())
	if err != nil {
		b.Fatalf("create user store: %v", err)
	}
	defer userStore.Close()

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	numUsers := 100
	userIDs := make([]types.UserID, numUsers)
	for i := 0; i < numUsers; i++ {
		userIDs[i] = types.UserID(i + 1)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			userID := userIDs[(i*1000+j)%numUsers]
			side := int8(constants.ORDER_SIDE_BUY)
			if (i*1000+j)%2 == 0 {
				side = int8(constants.ORDER_SIDE_SELL)
			}

			cmd := orderCmdPool.Get().(*PlaceOrderCmd)
			cmd.Req = &types.PlaceOrderRequest{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     side,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    price50000,
				Quantity: qty1,
			}

			e.Cmd(cmd)
			orderCmdPool.Put(cmd)
		}
	}
}

func BenchmarkEngineThroughputNoPersistence(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	numUsers := 100
	userIDs := make([]types.UserID, numUsers)
	for i := 0; i < numUsers; i++ {
		userIDs[i] = types.UserID(i + 1)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			userID := userIDs[(i*1000+j)%numUsers]
			side := int8(constants.ORDER_SIDE_BUY)
			if (i*1000+j)%2 == 0 {
				side = int8(constants.ORDER_SIDE_SELL)
			}

			cmd := orderCmdPool.Get().(*PlaceOrderCmd)
			cmd.Req = &types.PlaceOrderRequest{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     side,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    price50000,
				Quantity: qty1,
			}

			e.Cmd(cmd)
			orderCmdPool.Put(cmd)
		}
	}
}

func BenchmarkEngineMatchingOnly(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	numUsers := 100
	userIDs := make([]types.UserID, numUsers)
	for i := 0; i < numUsers; i++ {
		userIDs[i] = types.UserID(i + 1)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			userID := userIDs[(i*1000+j)%numUsers]
			side := int8(constants.ORDER_SIDE_BUY)
			if (i*1000+j)%2 == 0 {
				side = int8(constants.ORDER_SIDE_SELL)
			}

			idx := (i*1000 + j) % 1000

			cmd := orderCmdPool.Get().(*PlaceOrderCmd)
			cmd.Req = &types.PlaceOrderRequest{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     side,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    priceOffset[idx],
				Quantity: qty1,
			}

			e.Cmd(cmd)
			orderCmdPool.Put(cmd)
		}
	}
}

var _ = utils.NowNano
