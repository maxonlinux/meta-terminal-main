package clearing

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type mockP struct{ reserved, released map[string]types.Quantity }

func newMock() *mockP {
	return &mockP{make(map[string]types.Quantity), make(map[string]types.Quantity)}
}
func (m *mockP) Reserve(uid types.UserID, a string, amt types.Quantity) error {
	m.reserved[a] = amt
	return nil
}
func (m *mockP) Release(uid types.UserID, a string, amt types.Quantity) { m.released[a] = amt }
func (m *mockP) ExecuteTrade(*types.Trade, *types.Order, *types.Order)  {}
func (m *mockP) GetPosition(uid types.UserID, s string) *types.Position {
	return &types.Position{Leverage: 10}
}

func TestReserveSpotBuy(t *testing.T) {
	m := newMock()
	New(m).Reserve(1, "BTCUSDT", constants.CATEGORY_SPOT, constants.ORDER_SIDE_BUY, 100, 50000)
	if m.reserved["USDT"] != 5000000 {
		t.Fatal(m.reserved)
	}
}

func TestReserveSpotSell(t *testing.T) {
	m := newMock()
	New(m).Reserve(1, "BTCUSDT", constants.CATEGORY_SPOT, constants.ORDER_SIDE_SELL, 100, 50000)
	if m.reserved["BTC"] != 100 {
		t.Fatal(m.reserved)
	}
}

func TestReserveLinear(t *testing.T) {
	m := newMock()
	New(m).Reserve(1, "BTCUSDT", constants.CATEGORY_LINEAR, constants.ORDER_SIDE_BUY, 100, 50000)
	if m.reserved["USDT"] != 500000 {
		t.Fatal(m.reserved)
	}
}

func TestReleaseSpot(t *testing.T) {
	m := newMock()
	New(m).Release(1, "BTCUSDT", constants.CATEGORY_SPOT, constants.ORDER_SIDE_BUY, 100, 50000)
	if m.released["USDT"] != 5000000 {
		t.Fatal(m.released)
	}
}
