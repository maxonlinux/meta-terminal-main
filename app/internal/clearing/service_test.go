package clearing

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestIsImmediateLiquidationLeverage(t *testing.T) {
	tests := []struct {
		name     string
		leverage string
		want     bool
	}{
		{name: "safe 100x", leverage: "100", want: false},
		{name: "safe 20x", leverage: "20", want: false},
		{name: "unsafe 250x", leverage: "250", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lev, err := fixed.Parse(tt.leverage)
			if err != nil {
				t.Fatalf("parse leverage: %v", err)
			}
			got := IsImmediateLiquidationLeverage(types.Leverage(lev))
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
