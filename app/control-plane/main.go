package main

import (
	_ "github.com/walkmiao/flypig/app/control-plane/internal/packed"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/walkmiao/flypig/app/control-plane/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
