package cmd

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/walkmiao/flypig/app/data-plane/internal/controller/system"
	"github.com/walkmiao/flypig/app/data-plane/internal/service/hostloader"
	"github.com/walkmiao/flypig/app/data-plane/internal/service/pluginproc"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start data-plane http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server("data-plane")
			s.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(ghttp.MiddlewareHandlerResponse)
				group.Bind(system.NewV1())
			})
			s.Run()
			return nil
		},
	}

	Plugin = gcmd.Command{
		Name:  "plugin",
		Usage: "plugin",
		Brief: "start iec104 plugin process",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			pluginproc.Run()
			return nil
		},
	}

	HostLoader = gcmd.Command{
		Name:  "host-loader",
		Usage: "host-loader <plugin_binary_path>",
		Brief: "run host loader and call plugin contract",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			pluginPath := parser.GetArg(2).String()
			if pluginPath == "" {
				pluginPath = parser.GetArg(1).String()
			}
			if pluginPath == "" {
				return fmt.Errorf("usage: host-loader <plugin_binary_path>")
			}
			return hostloader.Run(pluginPath)
		},
	}
)

func init() {
	_ = Main.AddCommand(&Plugin, &HostLoader)
}
