package main

import (
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/wendy512/iec104/examples/plugin-ha-template/grpcplugin"
	pluginruntime "github.com/wendy512/iec104/examples/plugin-ha-template/runtime"
)

func main() {
	impl := pluginruntime.NewIEC104Plugin()

	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: grpcplugin.Handshake,
		Plugins:         grpcplugin.PluginSetWithServer(impl),
		GRPCServer:      hplugin.DefaultGRPCServer,
	})
}
