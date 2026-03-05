package main

import (
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/walkmiao/flypig/grpcplugin"
	pluginruntime "github.com/walkmiao/flypig/runtime"
)

func main() {
	impl := pluginruntime.NewIEC104Plugin()

	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: grpcplugin.Handshake,
		Plugins:         grpcplugin.PluginSetWithServer(impl),
		GRPCServer:      hplugin.DefaultGRPCServer,
	})
}
