package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/types"
	sym "github.com/maxonlinux/meta-terminal-go/pkg/symbol"
)

func TestRegistry(t *testing.T) {
	r := New()
	r.Add(&types.Instrument{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT"})
	if r.Get("BTCUSDT").BaseAsset != "BTC" {
		t.Error()
	}
}

func TestAddFromList(t *testing.T) {
	r := New()
	r.AddFromList([]string{"BTCUSDT", "ETHUSDT"})
	if r.Get("BTCUSDT").BaseAsset != "BTC" {
		t.Error()
	}
}

func TestPrice(t *testing.T) {
	r := New()
	r.SetPrice("BTCUSDT", types.PriceTick{Price: 50000})
	p, ok := r.Price("BTCUSDT")
	if !ok || p.Price != 50000 {
		t.Error(p, ok)
	}
}

func TestLoader(t *testing.T) {
	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]struct{ Symbol string }{{"BTCUSDT"}, {"ETHUSDT"}})
	}))
	defer assets.Close()

	prices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(struct{ Price int64 }{50000})
	}))
	defer prices.Close()

	r := New()
	l := NewLoader(assets.URL, prices.URL, r, time.Minute)
	l.sync()

	if _, ok := r.Price("BTCUSDT"); !ok {
		t.Error()
	}
}

func TestTypesQuoteAsset(t *testing.T) {
	if sym.GetQuoteAsset("BTCUSDT") != "USDT" {
		t.Error()
	}
}

func TestTypesBaseAsset(t *testing.T) {
	if sym.GetBaseAsset("BTCUSDT") != "BTC" {
		t.Error()
	}
}

func TestGetBand(t *testing.T) {
	tests := []struct {
		price    float64
		wantPrec int8
	}{
		{50000.0, 2},
		{500.0, 3},
		{10.0, 4},
		{0.1, 5},
		{0.001, 8},
	}
	for _, tt := range tests {
		b := GetBand(tt.price)
		if b.PricePrecision != tt.wantPrec {
			t.Errorf("GetBand(%v).PricePrecision = %d, want %d", tt.price, b.PricePrecision, tt.wantPrec)
		}
	}
}

func TestGetBandBounds(t *testing.T) {
	_, min, max := GetBandBounds(50000.0)
	if min <= 0 {
		t.Error("min should be > 0")
	}
	if max <= 0 {
		t.Error("max should be > 0 for band 0")
	}
}
