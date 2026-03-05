package system

import (
	"context"

	"github.com/walkmiao/flypig/app/data-plane/api/system/v1"
)

type ISystemV1 interface {
	Health(ctx context.Context, req *v1.HealthReq) (res *v1.HealthRes, err error)
}
