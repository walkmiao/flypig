package grpcplugin

import (
	"context"

	hplugin "github.com/hashicorp/go-plugin"
	"github.com/wendy512/iec104/examples/plugin-ha-template/contract"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var Handshake = hplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "IEC104_PLUGIN",
	MagicCookieValue: "iec104_grpc_v1",
}

const PluginName = "iec104"

func PluginSetWithClientOnly() map[string]hplugin.Plugin {
	return map[string]hplugin.Plugin{
		PluginName: &IEC104GRPCPlugin{},
	}
}

func PluginSetWithServer(impl contract.Plugin) map[string]hplugin.Plugin {
	return map[string]hplugin.Plugin{
		PluginName: &IEC104GRPCPlugin{Impl: impl},
	}
}

type IEC104GRPCPlugin struct {
	hplugin.NetRPCUnsupportedPlugin
	Impl contract.Plugin
}

func (p *IEC104GRPCPlugin) GRPCServer(_ *hplugin.GRPCBroker, s *grpc.Server) error {
	if p.Impl == nil {
		return status.Error(codes.FailedPrecondition, "nil plugin impl")
	}
	registerPluginService(s, &grpcServer{impl: p.Impl})
	return nil
}

func (p *IEC104GRPCPlugin) GRPCClient(_ context.Context, _ *hplugin.GRPCBroker, cc *grpc.ClientConn) (any, error) {
	return &grpcClient{cc: cc}, nil
}

type grpcServer struct {
	impl contract.Plugin
}

func (s *grpcServer) Start(ctx context.Context, _ *empty) (*empty, error) {
	return &empty{}, s.impl.Start(ctx)
}

func (s *grpcServer) Stop(ctx context.Context, _ *empty) (*empty, error) {
	return &empty{}, s.impl.Stop(ctx)
}

func (s *grpcServer) AssignShard(ctx context.Context, req *assignShardRequest) (*empty, error) {
	return &empty{}, s.impl.AssignShard(ctx, req.Assignment)
}

func (s *grpcServer) RevokeShard(ctx context.Context, req *revokeShardRequest) (*empty, error) {
	return &empty{}, s.impl.RevokeShard(ctx, req.ShardID)
}

func (s *grpcServer) Health(ctx context.Context, _ *empty) (*healthResponse, error) {
	h, err := s.impl.Health(ctx)
	if err != nil {
		return nil, err
	}
	return &healthResponse{Health: h}, nil
}

func (s *grpcServer) SendCommand(ctx context.Context, req *commandRequest) (*commandResponse, error) {
	r, err := s.impl.SendCommand(ctx, req.Request)
	if err != nil {
		return nil, err
	}
	return &commandResponse{Result: r}, nil
}

type grpcClient struct {
	cc *grpc.ClientConn
}

func (c *grpcClient) Start(ctx context.Context) error {
	return c.cc.Invoke(ctx, "/"+serviceName+"/Start", &empty{}, &empty{}, grpc.ForceCodec(jsonCodec{}))
}

func (c *grpcClient) Stop(ctx context.Context) error {
	return c.cc.Invoke(ctx, "/"+serviceName+"/Stop", &empty{}, &empty{}, grpc.ForceCodec(jsonCodec{}))
}

func (c *grpcClient) AssignShard(ctx context.Context, assignment contract.ShardAssignment) error {
	return c.cc.Invoke(ctx, "/"+serviceName+"/AssignShard", &assignShardRequest{Assignment: assignment}, &empty{}, grpc.ForceCodec(jsonCodec{}))
}

func (c *grpcClient) RevokeShard(ctx context.Context, shardID string) error {
	return c.cc.Invoke(ctx, "/"+serviceName+"/RevokeShard", &revokeShardRequest{ShardID: shardID}, &empty{}, grpc.ForceCodec(jsonCodec{}))
}

func (c *grpcClient) Health(ctx context.Context) (contract.HealthSnapshot, error) {
	out := new(healthResponse)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/Health", &empty{}, out, grpc.ForceCodec(jsonCodec{}))
	return out.Health, err
}

func (c *grpcClient) SendCommand(ctx context.Context, req contract.CommandRequest) (contract.CommandResult, error) {
	out := new(commandResponse)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/SendCommand", &commandRequest{Request: req}, out, grpc.ForceCodec(jsonCodec{}))
	return out.Result, err
}
