package matching

import (
	"testing"

	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/robaho/fixed"
)

// price builds a fixed-point price used in tests.
func price(value int64) types.Price {
	return types.Price(fixed.NewI(value, 0))
}

// qty builds a fixed-point quantity used in tests.
func qty(value int64) types.Quantity {
	return types.Quantity(fixed.NewI(value, 0))
}

// makeOrder creates a basic order used in matching tests.
func makeOrder(
	id int64,
	user types.UserID,
	symbol string,
	category int8,
	side int8,
	otype int8,
	tif int8,
	limit types.Price,
	amount types.Quantity,
) *types.Order {
	return &types.Order{
		ID:       types.OrderID(id),
		UserID:   user,
		Symbol:   symbol,
		Category: category,
		Side:     side,
		Type:     otype,
		TIF:      tif,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    limit,
		Quantity: amount,
		Filled:   qty(0),
	}
}

func applyTrade(match types.Match) {
	if match.MakerOrder == nil {
		return
	}

	now := utils.NowNano()
	remaining := math.Sub(match.MakerOrder.Quantity, match.MakerOrder.Filled)
	if remaining.Sign() == 0 {
		match.MakerOrder.Status = constants.ORDER_STATUS_FILLED
		match.MakerOrder.UpdatedAt = now
		return
	}

	match.MakerOrder.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
	match.MakerOrder.UpdatedAt = now
}

func TestMatch_PostOnlyRejectsCross(t *testing.T) {
	book := orderbook.New()

	maker := makeOrder(1, types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_SELL, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC, price(100), qty(1))
	if err := MatchOrder(maker, book, applyTrade); err != nil {
		t.Fatalf("maker match failed: %v", err)
	}

	postOnly := makeOrder(2, types.UserID(2), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_POST_ONLY, price(100), qty(1))
	if err := MatchOrder(postOnly, book, applyTrade); err != constants.ErrPostOnlyWouldMatch {
		t.Fatalf("expected post-only reject, got %v", err)
	}
}

func TestMatch_FOKRejectsWithoutLiquidity(t *testing.T) {
	book := orderbook.New()

	order := makeOrder(1, types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_FOK, price(100), qty(1))
	if err := MatchOrder(order, book, applyTrade); err != constants.ErrFOKInsufficientLiquidity {
		t.Fatalf("expected FOK reject, got %v", err)
	}
}

func TestMatch_FOKFillsFully(t *testing.T) {
	book := orderbook.New()

	maker := makeOrder(1, types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_SELL, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC, price(100), qty(2))
	if err := MatchOrder(maker, book, applyTrade); err != nil {
		t.Fatalf("maker match failed: %v", err)
	}

	taker := makeOrder(2, types.UserID(2), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_FOK, price(100), qty(2))
	if err := MatchOrder(taker, book, applyTrade); err != nil {
		t.Fatalf("FOK match failed: %v", err)
	}
	if taker.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED, got %d", taker.Status)
	}
	if taker.Filled.Cmp(qty(2)) != 0 {
		t.Fatalf("expected filled=2, got %s", taker.Filled)
	}
	if maker.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected maker FILLED, got %d", maker.Status)
	}
}

func TestMatch_IOCPartialCancel(t *testing.T) {
	book := orderbook.New()

	maker := makeOrder(1, types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_SELL, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC, price(100), qty(1))
	if err := MatchOrder(maker, book, applyTrade); err != nil {
		t.Fatalf("maker match failed: %v", err)
	}

	taker := makeOrder(2, types.UserID(2), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_IOC, price(100), qty(2))
	if err := MatchOrder(taker, book, applyTrade); err != nil {
		t.Fatalf("IOC match failed: %v", err)
	}
	if taker.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("expected PARTIALLY_FILLED_CANCELED, got %d", taker.Status)
	}
	if taker.Filled.Cmp(qty(1)) != 0 {
		t.Fatalf("expected filled=1, got %s", taker.Filled)
	}
}

func TestMatch_GTCAddsRestingOrder(t *testing.T) {
	book := orderbook.New()

	buyer := makeOrder(1, types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC, price(100), qty(1))
	if err := MatchOrder(buyer, book, applyTrade); err != nil {
		t.Fatalf("GTC match failed: %v", err)
	}
	if buyer.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("expected NEW, got %d", buyer.Status)
	}

	seller := makeOrder(2, types.UserID(2), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_SELL, constants.ORDER_TYPE_LIMIT, constants.TIF_IOC, price(100), qty(1))
	if err := MatchOrder(seller, book, applyTrade); err != nil {
		t.Fatalf("sell match failed: %v", err)
	}
	if buyer.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected buyer FILLED, got %d", buyer.Status)
	}
}
