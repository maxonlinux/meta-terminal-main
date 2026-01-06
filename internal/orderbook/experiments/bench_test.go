package orderbook

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/orderbook/experiments/dead_stack"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook/experiments/embedded_timestamp"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook/experiments/hash_slice"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook/experiments/linked_list"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook/experiments/validity_flag"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

const (
	numPrices  = 1000
	numOrders  = 5000
	benchLimit = 50
)

func BenchmarkAddOrder(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
		})
	}
}

func BenchmarkAddAsk(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddAsk(ss.(*hash_slice.OrderBookState), price, qty, id)
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddAsk(ss.(*validity_flag.OrderBookState), price, qty, id)
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddAsk(ss.(*dead_stack.OrderBookState), price, qty, id)
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddAsk(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddAsk(ss.(*linked_list.OrderBookState), price, qty, id)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
		})
	}
}

func BenchmarkRemoveOrder(b *testing.B) {
	type testCase struct {
		name   string
		ob     interface{}
		ss     interface{}
		add    func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		remove func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).RemoveOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).RemoveOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).RemoveOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).RemoveOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).RemoveOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.remove(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
		})
	}
}

func BenchmarkGetBestBid(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		get  func(ob interface{}, ss interface{}) types.Price
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*hash_slice.OrderBook).GetBestBid(ss.(*hash_slice.OrderBookState))
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*validity_flag.OrderBook).GetBestBid(ss.(*validity_flag.OrderBookState))
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*dead_stack.OrderBook).GetBestBid(ss.(*dead_stack.OrderBookState))
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*embedded_timestamp.OrderBook).GetBestBid(ss.(*embedded_timestamp.OrderBookState))
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*linked_list.OrderBook).GetBestBid(ss.(*linked_list.OrderBookState))
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.get(tc.ob, tc.ss)
			}
		})
	}
}

func BenchmarkGetBestAsk(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		get  func(ob interface{}, ss interface{}) types.Price
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddAsk(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*hash_slice.OrderBook).GetBestAsk(ss.(*hash_slice.OrderBookState))
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddAsk(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*validity_flag.OrderBook).GetBestAsk(ss.(*validity_flag.OrderBookState))
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddAsk(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*dead_stack.OrderBook).GetBestAsk(ss.(*dead_stack.OrderBookState))
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddAsk(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*embedded_timestamp.OrderBook).GetBestAsk(ss.(*embedded_timestamp.OrderBookState))
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddAsk(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}) types.Price {
			return ob.(*linked_list.OrderBook).GetBestAsk(ss.(*linked_list.OrderBookState))
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.get(tc.ob, tc.ss)
			}
		})
	}
}

func BenchmarkGetDepth(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		get  func(ob interface{}, ss interface{}, limit int) []int64
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*hash_slice.OrderBook).GetDepth(ss.(*hash_slice.OrderBookState), limit)
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*validity_flag.OrderBook).GetDepth(ss.(*validity_flag.OrderBookState), limit)
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*dead_stack.OrderBook).GetDepth(ss.(*dead_stack.OrderBookState), limit)
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*embedded_timestamp.OrderBook).GetDepth(ss.(*embedded_timestamp.OrderBookState), limit)
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*linked_list.OrderBook).GetDepth(ss.(*linked_list.OrderBookState), limit)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.get(tc.ob, tc.ss, benchLimit)
			}
		})
	}
}

func BenchmarkGetAskDepth(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		get  func(ob interface{}, ss interface{}, limit int) []int64
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddAsk(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*hash_slice.OrderBook).GetAskDepth(ss.(*hash_slice.OrderBookState), limit)
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddAsk(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*validity_flag.OrderBook).GetAskDepth(ss.(*validity_flag.OrderBookState), limit)
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddAsk(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*dead_stack.OrderBook).GetAskDepth(ss.(*dead_stack.OrderBookState), limit)
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddAsk(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*embedded_timestamp.OrderBook).GetAskDepth(ss.(*embedded_timestamp.OrderBookState), limit)
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddAsk(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, limit int) []int64 {
			return ob.(*linked_list.OrderBook).GetAskDepth(ss.(*linked_list.OrderBookState), limit)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.get(tc.ob, tc.ss, benchLimit)
			}
		})
	}
}

func BenchmarkWouldCross(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		wc   func(ob interface{}, ss interface{}, price types.Price) bool
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddAsk(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*hash_slice.OrderBook).WouldCross(price, ss.(*hash_slice.OrderBookState))
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddAsk(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*validity_flag.OrderBook).WouldCross(price, ss.(*validity_flag.OrderBookState))
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddAsk(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*dead_stack.OrderBook).WouldCross(price, ss.(*dead_stack.OrderBookState))
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddAsk(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*embedded_timestamp.OrderBook).WouldCross(price, ss.(*embedded_timestamp.OrderBookState))
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddAsk(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*linked_list.OrderBook).WouldCross(price, ss.(*linked_list.OrderBookState))
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.wc(tc.ob, tc.ss, types.Price(50000+i%numPrices))
			}
		})
	}
}

func BenchmarkWouldCrossAsk(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		wc   func(ob interface{}, ss interface{}, price types.Price) bool
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*hash_slice.OrderBook).WouldCrossAsk(price, ss.(*hash_slice.OrderBookState))
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*validity_flag.OrderBook).WouldCrossAsk(price, ss.(*validity_flag.OrderBookState))
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*dead_stack.OrderBook).WouldCrossAsk(price, ss.(*dead_stack.OrderBookState))
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*embedded_timestamp.OrderBook).WouldCrossAsk(price, ss.(*embedded_timestamp.OrderBookState))
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, price types.Price) bool {
			return ob.(*linked_list.OrderBook).WouldCrossAsk(price, ss.(*linked_list.OrderBookState))
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.wc(tc.ob, tc.ss, types.Price(50000+i%numPrices))
			}
		})
	}
}

func BenchmarkMarkLevelEmpty(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		mark func(ob interface{}, ss interface{}, isBid bool, price types.Price)
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*hash_slice.OrderBook).MarkLevelEmpty(ss.(*hash_slice.OrderBookState), isBid, price)
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*validity_flag.OrderBook).MarkLevelEmpty(ss.(*validity_flag.OrderBookState), isBid, price)
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*dead_stack.OrderBook).MarkLevelEmpty(ss.(*dead_stack.OrderBookState), isBid, price)
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*embedded_timestamp.OrderBook).MarkLevelEmpty(ss.(*embedded_timestamp.OrderBookState), isBid, price)
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*linked_list.OrderBook).MarkLevelEmpty(ss.(*linked_list.OrderBookState), isBid, price)
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.mark(tc.ob, tc.ss, true, price)
			}
		})
	}
}

func BenchmarkCompact(b *testing.B) {
	type testCase struct {
		name string
		ob   interface{}
		ss   interface{}
		add  func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID)
		mark func(ob interface{}, ss interface{}, isBid bool, price types.Price)
		comp func(ob interface{}, ss interface{})
	}

	cases := []testCase{
		{"hash_slice", hash_slice.New(), hash_slice.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*hash_slice.OrderBook).AddOrder(ss.(*hash_slice.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*hash_slice.OrderBook).MarkLevelEmpty(ss.(*hash_slice.OrderBookState), isBid, price)
		}, func(ob interface{}, ss interface{}) {
			ob.(*hash_slice.OrderBook).Compact(ss.(*hash_slice.OrderBookState))
		}},
		{"validity_flag", validity_flag.New(), validity_flag.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*validity_flag.OrderBook).AddOrder(ss.(*validity_flag.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*validity_flag.OrderBook).MarkLevelEmpty(ss.(*validity_flag.OrderBookState), isBid, price)
		}, func(ob interface{}, ss interface{}) {
			ob.(*validity_flag.OrderBook).Compact(ss.(*validity_flag.OrderBookState))
		}},
		{"dead_stack", dead_stack.New(), dead_stack.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*dead_stack.OrderBook).AddOrder(ss.(*dead_stack.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*dead_stack.OrderBook).MarkLevelEmpty(ss.(*dead_stack.OrderBookState), isBid, price)
		}, func(ob interface{}, ss interface{}) {
			ob.(*dead_stack.OrderBook).Compact(ss.(*dead_stack.OrderBookState))
		}},
		{"embedded_timestamp", embedded_timestamp.New(), embedded_timestamp.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*embedded_timestamp.OrderBook).AddOrder(ss.(*embedded_timestamp.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*embedded_timestamp.OrderBook).MarkLevelEmpty(ss.(*embedded_timestamp.OrderBookState), isBid, price)
		}, func(ob interface{}, ss interface{}) {
			ob.(*embedded_timestamp.OrderBook).Compact(ss.(*embedded_timestamp.OrderBookState))
		}},
		{"linked_list", linked_list.New(), linked_list.NewState(), func(ob interface{}, ss interface{}, price types.Price, qty types.Quantity, id types.OrderID) {
			ob.(*linked_list.OrderBook).AddOrder(ss.(*linked_list.OrderBookState), price, qty, id)
		}, func(ob interface{}, ss interface{}, isBid bool, price types.Price) {
			ob.(*linked_list.OrderBook).MarkLevelEmpty(ss.(*linked_list.OrderBookState), isBid, price)
		}, func(ob interface{}, ss interface{}) {
			ob.(*linked_list.OrderBook).Compact(ss.(*linked_list.OrderBookState))
		}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < numOrders; i++ {
				price := types.Price(50000 + i%numPrices)
				tc.add(tc.ob, tc.ss, price, 10, types.OrderID(i))
				if i%10 == 0 {
					tc.mark(tc.ob, tc.ss, true, price)
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.comp(tc.ob, tc.ss)
			}
		})
	}
}
