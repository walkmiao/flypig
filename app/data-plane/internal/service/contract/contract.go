package contract

import (
	"context"
	"time"
)

// StationConfig describes one IEC104 endpoint.
type StationConfig struct {
	StationID   string
	Host        string
	Port        int
	CommonAddr  uint16
	AutoConnect bool
}

// ShardAssignment binds a plugin instance with a shard and stations.
type ShardAssignment struct {
	ShardID     string
	OwnerID     string
	Stations    []StationConfig
	LeaseExpire time.Time
	Version     int64
}

// HealthSnapshot is reported by plugin instances.
type HealthSnapshot struct {
	InstanceID      string
	Alive           bool
	HeartbeatAt     time.Time
	ActiveShards    int
	ActiveConn      int
	EventQueueDepth int
	CommandFailRate float64
}

// Event is emitted from plugin data-plane to upper services.
type Event struct {
	StationID string
	Type      string
	IOA       uint32
	Value     string
	At        time.Time
}

// CommandRequest wraps control operations (YK/YT/total-call/time-sync).
type CommandRequest struct {
	RequestID string
	StationID string
	Type      string
	Payload   []byte
	Timeout   time.Duration
}

type CommandResult struct {
	RequestID string
	OK        bool
	Message   string
	At        time.Time
}

// Plugin is the minimal contract a go-plugin process should provide.
type Plugin interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	AssignShard(ctx context.Context, assignment ShardAssignment) error
	RevokeShard(ctx context.Context, shardID string) error

	Health(ctx context.Context) (HealthSnapshot, error)
	SendCommand(ctx context.Context, req CommandRequest) (CommandResult, error)
}

// LeaseStore persists shard ownership so failover can happen out-of-process.
type LeaseStore interface {
	ListAssignments(ctx context.Context) ([]ShardAssignment, error)
	ClaimShard(ctx context.Context, shardID, ownerID string, leaseTTL time.Duration) (ShardAssignment, error)
	RenewShard(ctx context.Context, shardID, ownerID string, leaseTTL time.Duration) error
	ReleaseShard(ctx context.Context, shardID, ownerID string) error
}
