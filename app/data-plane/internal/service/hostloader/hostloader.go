package hostloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	hplugin "github.com/hashicorp/go-plugin"
	"github.com/walkmiao/flypig/app/data-plane/internal/service/contract"
	"github.com/walkmiao/flypig/app/data-plane/internal/service/grpcplugin"
)

func Run(pluginPath string) error {
	stationHost := getEnv("IEC104_HOST", "127.0.0.1")
	stationPort := getEnvInt("IEC104_PORT", 2404)
	commonAddr := uint16(getEnvInt("IEC104_COMMON_ADDR", 1))

	client := hplugin.NewClient(&hplugin.ClientConfig{
		HandshakeConfig:  grpcplugin.Handshake,
		Plugins:          grpcplugin.PluginSetWithClientOnly(),
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []hplugin.Protocol{hplugin.ProtocolGRPC},
		SyncStdout:       os.Stdout,
		SyncStderr:       os.Stderr,
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return fmt.Errorf("create rpc client failed: %w", err)
	}

	raw, err := rpcClient.Dispense(grpcplugin.PluginName)
	if err != nil {
		return fmt.Errorf("dispense plugin failed: %w", err)
	}

	p, ok := raw.(contract.Plugin)
	if !ok {
		return fmt.Errorf("unexpected plugin type: %T", raw)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = p.Start(ctx); err != nil {
		return fmt.Errorf("start plugin failed: %w", err)
	}

	assign := contract.ShardAssignment{
		ShardID: "shard-cn-east-1",
		OwnerID: "host-loader-demo",
		Stations: []contract.StationConfig{
			{StationID: "station-1001", Host: stationHost, Port: stationPort, CommonAddr: commonAddr, AutoConnect: true},
		},
		LeaseExpire: time.Now().Add(15 * time.Second),
		Version:     1,
	}
	if err = p.AssignShard(ctx, assign); err != nil {
		return fmt.Errorf("assign shard failed: %w", err)
	}

	health, err := p.Health(ctx)
	if err != nil {
		return fmt.Errorf("health failed: %w", err)
	}
	fmt.Printf("health: alive=%v activeShards=%d activeConn=%d\n", health.Alive, health.ActiveShards, health.ActiveConn)

	result, err := p.SendCommand(ctx, contract.CommandRequest{
		RequestID: "cmd-1",
		StationID: "station-1001",
		Type:      "interrogation",
		Timeout:   2 * time.Second,
	})
	if err != nil {
		time.Sleep(800 * time.Millisecond)
		result, err = p.SendCommand(ctx, contract.CommandRequest{
			RequestID: "cmd-1-retry",
			StationID: "station-1001",
			Type:      "interrogation",
			Timeout:   2 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("send command failed: %w", err)
		}
	}
	fmt.Printf("command: ok=%v message=%s at=%s\n", result.OK, result.Message, result.At.Format(time.RFC3339))

	runCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	fmt.Println("host-loader running, press Ctrl+C to stop")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-runCtx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := p.Stop(shutdownCtx); err != nil {
				fmt.Printf("stop plugin failed: %v\n", err)
			}
			cancel()
			return nil
		case <-ticker.C:
			healthCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			h, err := p.Health(healthCtx)
			cancel()
			if err != nil {
				fmt.Printf("periodic health failed: %v\n", err)
				continue
			}
			fmt.Printf("health-tick: alive=%v activeShards=%d activeConn=%d at=%s\n",
				h.Alive, h.ActiveShards, h.ActiveConn, time.Now().Format(time.RFC3339))
		}
	}
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
