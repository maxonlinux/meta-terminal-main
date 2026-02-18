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
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
	"github.com/rs/zerolog"
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
		zerolog.Ctx(ctx).Error().Err(err).Msg("registry loader: failed to fetch symbols")
		return
	}

	for _, asset := range assets {
		price, err := l.fetchPrice(ctx, asset.Symbol)
		if err != nil {
			zerolog.Ctx(ctx).Warn().Str("symbol", asset.Symbol).Err(err).Msg("registry loader: skipping asset")
			continue
		}
		if math.Sign(price) <= 0 {
			zerolog.Ctx(ctx).Warn().Str("symbol", asset.Symbol).Str("price", price.String()).Msg("registry loader: skipping asset with invalid price")
			continue
		}

		if l.reg.GetInstrument(asset.Symbol) != nil {
			continue
		}

		inst := FromSymbol(asset.Symbol, price, asset.Type)
		l.reg.SetInstrument(asset.Symbol, inst)
		zerolog.Ctx(ctx).Info().Str("symbol", asset.Symbol).Str("price", price.String()).Msg("registry loader: loaded instrument")
	}
}

type assetInfo struct {
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
}

func (l *Loader) fetchSymbols(ctx context.Context) ([]assetInfo, error) {
	url := l.cfg.CoreURL + "/assets"
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

func (l *Loader) fetchPrice(ctx context.Context, symbol string) (types.Price, error) {
	url := fmt.Sprintf("%s?symbol=%s", strings.TrimRight(l.cfg.MultiplexerURL, "/"), symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return types.Price{}, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return types.Price{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return types.Price{}, fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return types.Price{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.Price{}, err
	}

	if len(body) == 0 || string(body) == "null" {
		return types.Price{}, fmt.Errorf("empty price")
	}
	// Parse prices as fixed-point to avoid float rounding.
	price, err := fixed.Parse(strings.TrimSpace(string(body)))
	if err != nil {
		return types.Price{}, err
	}
	return types.Price(price), nil
}
