package pool

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOrderPoolReset(t *testing.T) {
	o := GetOrder()
	o.ID = 1
	o.UserID = 2
	o.Symbol = "BTCUSDT"
	o.Price = 50000
	o.Quantity = 10
	PutOrder(o)

	o2 := GetOrder()
	if o2.ID != 0 || o2.UserID != 0 || o2.Symbol != "" || o2.Price != 0 || o2.Quantity != 0 {
		t.Fatalf("expected order reset")
	}
	PutOrder(o2)
}

func TestTradePoolReset(t *testing.T) {
	trade := GetTrade()
	trade.ID = 1
	trade.Symbol = "BTCUSDT"
	trade.TakerID = 1
	trade.MakerID = 2
	trade.Price = 50000
	trade.Quantity = 10
	PutTrade(trade)

	t2 := GetTrade()
	if t2.ID != 0 || t2.Symbol != "" || t2.TakerID != 0 || t2.MakerID != 0 || t2.Price != 0 || t2.Quantity != 0 {
		t.Fatalf("expected trade reset")
	}
	PutTrade(t2)
}

func TestOrderResultPoolReset(t *testing.T) {
	r := GetOrderResult()
	r.Order = &types.Order{ID: 1}
	r.Trades = []*types.Trade{{ID: 1}}
	PutOrderResult(r)

	r2 := GetOrderResult()
	if r2.Order != nil || r2.Trades != nil {
		t.Fatalf("expected result reset")
	}
	PutOrderResult(r2)
}
