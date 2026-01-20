package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/config"
)

type Loader struct {
	cfg    config.Config
	reg    *Registry
	client *http.Client
}

func NewLoader(cfg config.Config, reg *Registry) *Loader {
	return &Loader{
		cfg:    cfg,
		reg:    reg,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (l *Loader) Start(ctx context.Context) {
	l.sync(ctx)
	ticker := time.NewTicker(l.cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.sync(ctx)
		}
	}
}

func (l *Loader) sync(ctx context.Context) {
	symbols, err := l.fetchSymbols(ctx)
	if err != nil {
		fmt.Printf("registry loader: failed to fetch symbols: %v\n", err)
		return
	}

	for _, symbol := range symbols {
		price, err := l.fetchPrice(ctx, symbol)
		if err != nil {
			fmt.Printf("registry loader: skipping %s: %v\n", symbol, err)
			continue
		}
		if price <= 0 {
			fmt.Printf("registry loader: skipping %s: invalid price %d\n", symbol, price)
			continue
		}

		if l.reg.GetInstrument(symbol) != nil {
			continue
		}

		inst := FromSymbol(symbol, price)
		l.reg.SetInstrument(symbol, inst)
		fmt.Printf("registry loader: loaded %s (price=%d)\n", symbol, price)
	}
}

func (l *Loader) fetchSymbols(ctx context.Context) ([]string, error) {
	url := strings.TrimRight(l.cfg.AssetsURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var assets []struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(body, &assets); err != nil {
		return nil, err
	}

	symbols := make([]string, 0, len(assets))
	for _, a := range assets {
		symbols = append(symbols, a.Symbol)
	}
	return symbols, nil
}

func (l *Loader) fetchPrice(ctx context.Context, symbol string) (int64, error) {
	url := fmt.Sprintf("%s?symbol=%s", strings.TrimRight(l.cfg.MultiplexerURL, "/"), symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return 0, fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if len(body) == 0 || string(body) == "null" {
		return 0, fmt.Errorf("empty price")
	}

	var price int64
	if _, err := fmt.Sscanf(string(body), "%d", &price); err != nil {
		return 0, err
	}
	return price, nil
}
