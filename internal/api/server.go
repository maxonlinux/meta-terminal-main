package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/anomalyco/meta-terminal-go/config"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

type Server struct {
	cfg    *config.Config
	state  *state.EngineState
	spot   *spot.Spot
	linear *linear.Linear
	server *http.Server
	mu     sync.RWMutex
}

func NewServer(cfg *config.Config, s *state.EngineState, spot *spot.Spot, linear *linear.Linear) *Server {
	return &Server{
		cfg:    cfg,
		state:  s,
		spot:   spot,
		linear: linear,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/orders", s.handleOrders)
	mux.HandleFunc("/api/v1/orders/", s.handleOrderByID)
	mux.HandleFunc("/api/v1/balances", s.handleBalances)
	mux.HandleFunc("/api/v1/positions", s.handlePositions)
	mux.HandleFunc("/api/v1/orderbook", s.handleOrderBook)

	s.server = &http.Server{Addr: ":8080", Handler: mux}
	return s.server.ListenAndServe()
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Symbol         string `json:"symbol"`
		Category       int8   `json:"category"`
		Side           int8   `json:"side"`
		Type           int8   `json:"type"`
		TIF            int8   `json:"tif"`
		Quantity       int64  `json:"quantity"`
		Price          int64  `json:"price"`
		ReduceOnly     bool   `json:"reduceOnly"`
		CloseOnTrigger bool   `json:"closeOnTrigger"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	result, _ := s.spot.PlaceOrder(&types.OrderInput{
		UserID:         1,
		Symbol:         input.Symbol,
		Category:       input.Category,
		Side:           input.Side,
		Type:           input.Type,
		TIF:            input.TIF,
		Quantity:       types.Quantity(input.Quantity),
		Price:          types.Price(input.Price),
		ReduceOnly:     input.ReduceOnly,
		CloseOnTrigger: input.CloseOnTrigger,
	})

	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleOrderByID(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request)  {}
func (s *Server) handlePositions(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleOrderBook(w http.ResponseWriter, r *http.Request) {}
