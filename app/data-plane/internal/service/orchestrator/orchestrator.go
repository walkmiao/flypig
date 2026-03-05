package orchestrator

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/walkmiao/flypig/app/data-plane/internal/service/contract"
)

// Orchestrator manages shard failover and balancing for plugin instances.
type Orchestrator struct {
	LeaseTTL time.Duration
	Store    contract.LeaseStore
}

func (o *Orchestrator) FailoverLostOwners(
	ctx context.Context,
	instances []contract.HealthSnapshot,
) error {
	if o.Store == nil {
		return errors.New("nil lease store")
	}

	alive := make(map[string]contract.HealthSnapshot, len(instances))
	for _, h := range instances {
		if h.Alive {
			alive[h.InstanceID] = h
		}
	}
	if len(alive) == 0 {
		return errors.New("no alive instances")
	}

	assignments, err := o.Store.ListAssignments(ctx)
	if err != nil {
		return err
	}

	for _, a := range assignments {
		_, ok := alive[a.OwnerID]
		if ok && a.LeaseExpire.After(time.Now()) {
			continue
		}

		target := pickLowestLoad(alive)
		if target == "" {
			return errors.New("no target instance for failover")
		}

		if _, err = o.Store.ClaimShard(ctx, a.ShardID, target, o.leaseTTL()); err != nil {
			return err
		}
		h := alive[target]
		h.ActiveShards++
		alive[target] = h
	}
	return nil
}

func (o *Orchestrator) Rebalance(
	ctx context.Context,
	instances []contract.HealthSnapshot,
) error {
	if o.Store == nil {
		return errors.New("nil lease store")
	}
	if len(instances) == 0 {
		return nil
	}

	alive := make([]contract.HealthSnapshot, 0, len(instances))
	for _, h := range instances {
		if h.Alive {
			alive = append(alive, h)
		}
	}
	if len(alive) < 2 {
		return nil
	}

	sort.Slice(alive, func(i, j int) bool {
		return alive[i].ActiveShards < alive[j].ActiveShards
	})
	minLoad := alive[0].ActiveShards
	maxLoad := alive[len(alive)-1].ActiveShards
	if maxLoad-minLoad <= 1 {
		return nil
	}

	assignments, err := o.Store.ListAssignments(ctx)
	if err != nil {
		return err
	}

	from := alive[len(alive)-1].InstanceID
	to := alive[0].InstanceID
	for _, a := range assignments {
		if a.OwnerID != from {
			continue
		}
		if err = o.Store.ReleaseShard(ctx, a.ShardID, from); err != nil {
			return err
		}
		if _, err = o.Store.ClaimShard(ctx, a.ShardID, to, o.leaseTTL()); err != nil {
			return err
		}
		break
	}
	return nil
}

func pickLowestLoad(instances map[string]contract.HealthSnapshot) string {
	id := ""
	load := int(^uint(0) >> 1)
	for k, h := range instances {
		if h.ActiveShards < load {
			id = k
			load = h.ActiveShards
		}
	}
	return id
}

func (o *Orchestrator) leaseTTL() time.Duration {
	if o.LeaseTTL <= 0 {
		return 15 * time.Second
	}
	return o.LeaseTTL
}
