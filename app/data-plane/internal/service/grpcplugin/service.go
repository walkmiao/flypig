package grpcplugin

import (
	"context"

	"github.com/walkmiao/flypig/app/data-plane/internal/service/contract"
	"google.golang.org/grpc"
)

const serviceName = "iec104.plugin.v1.PluginService"

type empty struct{}

type assignShardRequest struct {
	Assignment contract.ShardAssignment
}

type revokeShardRequest struct {
	ShardID string
}

type commandRequest struct {
	Request contract.CommandRequest
}

type commandResponse struct {
	Result contract.CommandResult
}

type healthResponse struct {
	Health contract.HealthSnapshot
}

type pluginService interface {
	Start(context.Context, *empty) (*empty, error)
	Stop(context.Context, *empty) (*empty, error)
	AssignShard(context.Context, *assignShardRequest) (*empty, error)
	RevokeShard(context.Context, *revokeShardRequest) (*empty, error)
	Health(context.Context, *empty) (*healthResponse, error)
	SendCommand(context.Context, *commandRequest) (*commandResponse, error)
}

func registerPluginService(s grpc.ServiceRegistrar, impl pluginService) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*pluginService)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Start", Handler: handleStart(impl)},
			{MethodName: "Stop", Handler: handleStop(impl)},
			{MethodName: "AssignShard", Handler: handleAssignShard(impl)},
			{MethodName: "RevokeShard", Handler: handleRevokeShard(impl)},
			{MethodName: "Health", Handler: handleHealth(impl)},
			{MethodName: "SendCommand", Handler: handleSendCommand(impl)},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "plugin-service",
	}, impl)
}

func handleStart(impl pluginService) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(empty)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.Start(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/Start"}
		handler := func(ctx context.Context, req any) (any, error) {
			return impl.Start(ctx, req.(*empty))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func handleStop(impl pluginService) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(empty)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.Stop(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/Stop"}
		handler := func(ctx context.Context, req any) (any, error) {
			return impl.Stop(ctx, req.(*empty))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func handleAssignShard(impl pluginService) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(assignShardRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.AssignShard(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/AssignShard"}
		handler := func(ctx context.Context, req any) (any, error) {
			return impl.AssignShard(ctx, req.(*assignShardRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func handleRevokeShard(impl pluginService) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(revokeShardRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.RevokeShard(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/RevokeShard"}
		handler := func(ctx context.Context, req any) (any, error) {
			return impl.RevokeShard(ctx, req.(*revokeShardRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func handleHealth(impl pluginService) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(empty)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.Health(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/Health"}
		handler := func(ctx context.Context, req any) (any, error) {
			return impl.Health(ctx, req.(*empty))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func handleSendCommand(impl pluginService) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(commandRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.SendCommand(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/SendCommand"}
		handler := func(ctx context.Context, req any) (any, error) {
			return impl.SendCommand(ctx, req.(*commandRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}
