package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	hplugin "github.com/hashicorp/go-plugin"
	"github.com/wendy512/iec104/examples/plugin-ha-template/contract"
	"github.com/wendy512/iec104/examples/plugin-ha-template/grpcplugin"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <plugin_binary_path>", os.Args[0])
	}

	pluginPath := os.Args[1]
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
		log.Fatalf("create rpc client failed: %v", err)
	}

	raw, err := rpcClient.Dispense(grpcplugin.PluginName)
	if err != nil {
		log.Fatalf("dispense plugin failed: %v", err)
	}

	p, ok := raw.(contract.Plugin)
	if !ok {
		log.Fatalf("unexpected plugin type: %T", raw)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = p.Start(ctx); err != nil {
		log.Fatalf("start plugin failed: %v", err)
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
		log.Fatalf("assign shard failed: %v", err)
	}

	health, err := p.Health(ctx)
	if err != nil {
		log.Fatalf("health failed: %v", err)
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
			log.Fatalf("send command failed: %v", err)
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
				log.Printf("stop plugin failed: %v", err)
			}
			cancel()
			return
		case <-ticker.C:
			healthCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			h, err := p.Health(healthCtx)
			cancel()
			if err != nil {
				log.Printf("periodic health failed: %v", err)
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
