package api

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
)

type Server struct {
	cfg    *config.Config
	engine *engine.Engine
}

func NewServer(cfg *config.Config, e *engine.Engine) *Server {
	return &Server{
		cfg:    cfg,
		engine: e,
	}
}

func (s *Server) Start(ctx context.Context) error {
	return nil
}
