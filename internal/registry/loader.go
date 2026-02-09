package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
		client: &http.Client{Timeout: cfg.RegistryTimeout},
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
	assets, err := l.fetchSymbols(ctx)
	if err != nil {
		fmt.Printf("registry loader: failed to fetch symbols: %v\n", err)
		return
	}

	for _, asset := range assets {
		price, err := l.fetchPrice(ctx, asset.Symbol)
		if err != nil {
			fmt.Printf("registry loader: skipping %s: %v\n", asset.Symbol, err)
			continue
		}
		if price <= 0 {
			fmt.Printf("registry loader: skipping %s: invalid price %.8f\n", asset.Symbol, price)
			continue
		}

		if l.reg.GetInstrument(asset.Symbol) != nil {
			continue
		}

		inst := FromSymbol(asset.Symbol, price, asset.Type)
		l.reg.SetInstrument(asset.Symbol, inst)
		fmt.Printf("registry loader: loaded %s (price=%.8f)\n", asset.Symbol, price)
	}
}

type assetInfo struct {
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
}

func (l *Loader) fetchSymbols(ctx context.Context) ([]assetInfo, error) {
	url := strings.TrimRight(l.cfg.AssetsURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var assets []assetInfo
	if err := json.Unmarshal(body, &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

func (l *Loader) fetchPrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s?symbol=%s", strings.TrimRight(l.cfg.MultiplexerURL, "/"), symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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

	price, err := strconv.ParseFloat(strings.TrimSpace(string(body)), 64)
	if err != nil {
		return 0, err
	}
	return price, nil
}
