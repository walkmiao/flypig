package system

import (
	"context"

	"github.com/walkmiao/flypig/app/control-plane/api/system/v1"
)

func (c *ControllerV1) Health(_ context.Context, _ *v1.HealthReq) (res *v1.HealthRes, err error) {
	return &v1.HealthRes{Status: "ok", App: "control-plane"}, nil
}
