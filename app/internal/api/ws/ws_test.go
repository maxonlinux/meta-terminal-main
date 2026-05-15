package ws

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

func TestShouldSendOrderUpdateDeduplicatesSameStatus(t *testing.T) {
	h := newWsHub(nil)
	userID := types.UserID(10)
	orderID := types.OrderID(77)

	if !h.shouldSendOrderUpdate(userID, orderID, "NEW") {
		t.Fatalf("first status must be sent")
	}
	if h.shouldSendOrderUpdate(userID, orderID, "NEW") {
		t.Fatalf("duplicate status must be dropped")
	}
	if !h.shouldSendOrderUpdate(userID, orderID, "PARTIALLY_FILLED") {
		t.Fatalf("status transition must be sent")
	}
}
