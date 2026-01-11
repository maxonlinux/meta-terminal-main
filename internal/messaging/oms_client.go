package messaging

import (
	"context"
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var ErrNoOMSClient = errors.New("oms nats client not configured")

type OMSClient struct {
	nats *NATS
}

func NewOMSClient(cfg Config) (*OMSClient, error) {
	n, err := New(cfg)
	if err != nil {
		return nil, err
	}
	return &OMSClient{nats: n}, nil
}

func (c *OMSClient) Close() {
	if c.nats != nil {
		c.nats.Close()
	}
}

func (c *OMSClient) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	if c.nats == nil {
		return nil, ErrNoOMSClient
	}
	subject := OrderPlaceTopic(input.Symbol)
	if err := c.nats.PublishGob(ctx, subject, input); err != nil {
		return nil, err
	}
	return &types.OrderResult{
		Orders:    nil,
		Trades:    nil,
		Filled:    0,
		Remaining: input.Quantity,
		Status:    constants.ORDER_STATUS_NEW,
	}, nil
}
