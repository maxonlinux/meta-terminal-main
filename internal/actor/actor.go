package actor

import (
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Message interface{}

type MultiUserState struct {
	Users map[types.UserID]*UserActorState
}

func NewMultiUserState() *MultiUserState {
	return &MultiUserState{
		Users: make(map[types.UserID]*UserActorState),
	}
}

type Actor struct {
	inbox     chan Message
	state     any
	handler   func(state any, msg Message)
	workerID  int64
	processed int64
}

type ActorConfig struct {
	BufferSize   int
	Workers      int
	StateFactory func() any
	Handler      func(state any, msg Message)
}

func New(cfg ActorConfig) *Actor {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1024
	}
	if cfg.StateFactory == nil {
		cfg.StateFactory = func() any { return nil }
	}
	if cfg.Handler == nil {
		cfg.Handler = func(state any, msg Message) {}
	}

	a := &Actor{
		inbox:   make(chan Message, cfg.BufferSize),
		state:   cfg.StateFactory(),
		handler: cfg.Handler,
	}

	for i := 0; i < cfg.Workers; i++ {
		go func(workerID int) {
			a.workerID = int64(workerID)
			for msg := range a.inbox {
				a.handler(a.state, msg)
				atomic.AddInt64(&a.processed, 1)
				runtime.Gosched()
			}
		}(i)
	}

	return a
}

func (a *Actor) Send(msg Message) {
	select {
	case a.inbox <- msg:
	default:
		a.handleFull(msg)
	}
}

func (a *Actor) SendBlocking(msg Message) {
	a.inbox <- msg
}

func (a *Actor) handleFull(msg Message) {
	for {
		select {
		case a.inbox <- msg:
			return
		default:
			runtime.Gosched()
		}
	}
}

func (a *Actor) State() any {
	return a.state
}

func (a *Actor) Processed() int64 {
	return atomic.LoadInt64(&a.processed)
}

func (a *Actor) Len() int {
	return len(a.inbox)
}

func (a *Actor) Cap() int {
	return cap(a.inbox)
}

type ShardedActor struct {
	shards    []*Actor
	numShards int
	hasher    func(msg Message) uint64
	cfg       ActorConfig
	totalSent int64
	totalProc int64
}

func NewSharded(numShards int, cfg ActorConfig, hasher func(msg Message) uint64) *ShardedActor {
	if numShards <= 0 {
		numShards = runtime.NumCPU() * 2
	}
	if hasher == nil {
		hasher = func(msg Message) uint64 {
			return uint64(uintptr(unsafe.Pointer(&msg))) % uint64(numShards)
		}
	}

	shards := make([]*Actor, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = New(cfg)
	}

	return &ShardedActor{
		shards:    shards,
		numShards: numShards,
		hasher:    hasher,
		cfg:       cfg,
	}
}

func NewShardedWithUserHash(numShards int, cfg ActorConfig) *ShardedActor {
	if numShards <= 0 {
		numShards = runtime.NumCPU() * 2
	}

	hasher := func(msg Message) uint64 {
		switch m := msg.(type) {
		case MsgPlaceOrder:
			return uint64(m.UserID)
		case MsgCancelOrder:
			return uint64(m.UserID)
		case MsgPositionUpdate:
			return uint64(m.UserID)
		case MsgTradeExecuted:
			return uint64(m.UserID)
		case MsgTriggerOrder:
			return uint64(m.UserID)
		case MsgDeactivateOrder:
			return uint64(m.UserID)
		case MsgOCOTriggered:
			return uint64(m.UserID)
		case MsgGetState:
			return uint64(m.UserID)
		case MsgGetOrder:
			return uint64(m.UserID)
		case MsgGetOrders:
			return uint64(m.UserID)
		case MsgAddUserOrder:
			return uint64(m.UserID)
		}
		return uint64(uintptr(unsafe.Pointer(&msg))) % uint64(numShards)
	}

	shards := make([]*Actor, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = New(cfg)
	}

	return &ShardedActor{
		shards:    shards,
		numShards: numShards,
		hasher:    hasher,
		cfg:       cfg,
	}
}

func (s *ShardedActor) Send(msg Message) {
	shard := s.shards[s.shardIndex(msg)]
	shard.Send(msg)
	atomic.AddInt64(&s.totalSent, 1)
}

func (s *ShardedActor) shardIndex(msg Message) int {
	return int(s.hasher(msg) % uint64(s.numShards))
}

func (s *ShardedActor) GetShard(userID uint64) *Actor {
	return s.shards[int(userID)%s.numShards]
}

func (s *ShardedActor) Len() int {
	total := 0
	for _, shard := range s.shards {
		total += shard.Len()
	}
	return total
}

func (s *ShardedActor) Cap() int {
	return s.cfg.BufferSize * s.numShards
}

func (s *ShardedActor) TotalProcessed() int64 {
	var total int64
	for _, shard := range s.shards {
		total += shard.Processed()
	}
	return atomic.LoadInt64(&s.totalProc)
}
