package system

import "github.com/walkmiao/flypig/app/data-plane/api/system"

type ControllerV1 struct{}

func NewV1() system.ISystemV1 {
	return &ControllerV1{}
}
