package v1

import "github.com/gogf/gf/v2/frame/g"

type HealthReq struct {
	g.Meta `path:"/healthz" tags:"System" method:"get" summary:"Service health check"`
}

type HealthRes struct {
	Status string `json:"status" dc:"service status"`
	App    string `json:"app" dc:"application name"`
}
