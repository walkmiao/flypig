package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wendy512/iec104/examples/plugin-ha-template/contract"
)

type MockPlugin struct {
	mu      sync.RWMutex
	started bool
	shards  map[string]contract.ShardAssignment
}

func NewMockPlugin() *MockPlugin {
	return &MockPlugin{
		shards: make(map[string]contract.ShardAssignment),
	}
}

func (p *MockPlugin) Start(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = true
	return nil
}

func (p *MockPlugin) Stop(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = false
	p.shards = make(map[string]contract.ShardAssignment)
	return nil
}

func (p *MockPlugin) AssignShard(_ context.Context, assignment contract.ShardAssignment) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return fmt.Errorf("plugin not started")
	}
	p.shards[assignment.ShardID] = assignment
	return nil
}

func (p *MockPlugin) RevokeShard(_ context.Context, shardID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.shards, shardID)
	return nil
}

func (p *MockPlugin) Health(context.Context) (contract.HealthSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return contract.HealthSnapshot{
		InstanceID:      "mock-plugin-instance",
		Alive:           p.started,
		HeartbeatAt:     time.Now(),
		ActiveShards:    len(p.shards),
		ActiveConn:      len(p.shards) * 3,
		EventQueueDepth: 0,
		CommandFailRate: 0,
	}, nil
}

func (p *MockPlugin) SendCommand(_ context.Context, req contract.CommandRequest) (contract.CommandResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.started {
		return contract.CommandResult{}, fmt.Errorf("plugin not started")
	}
	return contract.CommandResult{
		RequestID: req.RequestID,
		OK:        true,
		Message:   "accepted by mock plugin",
		At:        time.Now(),
	}, nil
}
