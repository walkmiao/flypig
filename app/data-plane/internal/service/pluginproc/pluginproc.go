package pluginproc

import (
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/walkmiao/flypig/app/data-plane/internal/service/grpcplugin"
	pluginruntime "github.com/walkmiao/flypig/app/data-plane/internal/service/runtime"
)

func Run() {
	impl := pluginruntime.NewIEC104Plugin()

	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: grpcplugin.Handshake,
		Plugins:         grpcplugin.PluginSetWithServer(impl),
		GRPCServer:      hplugin.DefaultGRPCServer,
	})
}
