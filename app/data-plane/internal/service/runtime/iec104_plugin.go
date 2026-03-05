package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/walkmiao/flypig/app/data-plane/internal/service/contract"
	"github.com/wendy512/go-iecp5/asdu"
	iecc "github.com/wendy512/iec104/client"
)

type IEC104Plugin struct {
	mu sync.RWMutex

	started bool
	shards  map[string]contract.ShardAssignment

	stations map[string]*stationRuntime
	values   map[string]map[uint32]string

	commandTotal int64
	commandFail  int64
}

type stationRuntime struct {
	cfg    contract.StationConfig
	client *iecc.Client
	ready  bool
}

type stationCall struct {
	plugin    *IEC104Plugin
	stationID string
}

type commandPayload struct {
	CommonAddr uint16 `json:"common_addr"`
	IOA        uint   `json:"ioa"`
	TypeID     uint8  `json:"type_id"`
	Value      any    `json:"value"`
	Timestamp  string `json:"timestamp"`
}

func NewIEC104Plugin() *IEC104Plugin {
	return &IEC104Plugin{
		shards:   make(map[string]contract.ShardAssignment),
		stations: make(map[string]*stationRuntime),
		values:   make(map[string]map[uint32]string),
	}
}

func (p *IEC104Plugin) Start(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = true
	return nil
}

func (p *IEC104Plugin) Stop(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, rt := range p.stations {
		_ = rt.client.Close()
	}
	p.started = false
	p.shards = make(map[string]contract.ShardAssignment)
	p.stations = make(map[string]*stationRuntime)
	p.values = make(map[string]map[uint32]string)
	return nil
}

func (p *IEC104Plugin) AssignShard(ctx context.Context, assignment contract.ShardAssignment) error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return fmt.Errorf("plugin not started")
	}
	old, hadOld := p.shards[assignment.ShardID]
	p.shards[assignment.ShardID] = assignment
	p.mu.Unlock()

	// Close old stations first if shard reassigned.
	if hadOld {
		for _, s := range old.Stations {
			if err := p.closeStation(s.StationID); err != nil {
				return err
			}
		}
	}

	for _, s := range assignment.Stations {
		if err := p.connectStation(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (p *IEC104Plugin) RevokeShard(ctx context.Context, shardID string) error {
	p.mu.Lock()
	assignment, ok := p.shards[shardID]
	if ok {
		delete(p.shards, shardID)
	}
	p.mu.Unlock()
	if !ok {
		return nil
	}
	for _, s := range assignment.Stations {
		if err := p.closeStationWithCtx(ctx, s.StationID); err != nil {
			return err
		}
	}
	return nil
}

func (p *IEC104Plugin) Health(context.Context) (contract.HealthSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	activeConn := 0
	for _, rt := range p.stations {
		if rt.client.IsConnected() {
			activeConn++
		}
	}

	failRate := 0.0
	if p.commandTotal > 0 {
		failRate = float64(p.commandFail) / float64(p.commandTotal)
	}

	return contract.HealthSnapshot{
		InstanceID:      "iec104-plugin-instance",
		Alive:           p.started,
		HeartbeatAt:     time.Now(),
		ActiveShards:    len(p.shards),
		ActiveConn:      activeConn,
		EventQueueDepth: 0,
		CommandFailRate: failRate,
	}, nil
}

func (p *IEC104Plugin) SendCommand(ctx context.Context, req contract.CommandRequest) (contract.CommandResult, error) {
	p.mu.Lock()
	p.commandTotal++
	p.mu.Unlock()

	result := contract.CommandResult{RequestID: req.RequestID, At: time.Now()}

	rt, err := p.getStation(req.StationID)
	if err != nil {
		p.incrFail()
		return result, err
	}

	payload, err := decodePayload(req.Payload, rt.cfg.CommonAddr)
	if err != nil {
		p.incrFail()
		return result, err
	}

	if err := p.waitStationReady(ctx, req.StationID); err != nil {
		p.incrFail()
		return result, err
	}

	log.Printf("iec104 send command request_id=%s station=%s type=%s common_addr=%d ioa=%d type_id=%d",
		req.RequestID, req.StationID, req.Type, payload.CommonAddr, payload.IOA, payload.TypeID)

	cmdCtx := ctx
	cancel := func() {}
	if req.Timeout > 0 {
		cmdCtx, cancel = context.WithTimeout(ctx, req.Timeout)
	}
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- executeCommand(rt.client, req.Type, payload)
	}()

	select {
	case <-cmdCtx.Done():
		p.incrFail()
		result.OK = false
		result.Message = cmdCtx.Err().Error()
		log.Printf("iec104 send command timeout request_id=%s station=%s type=%s err=%v",
			req.RequestID, req.StationID, req.Type, cmdCtx.Err())
		return result, cmdCtx.Err()
	case err = <-done:
		if err != nil {
			p.incrFail()
			result.OK = false
			result.Message = err.Error()
			log.Printf("iec104 send command failed request_id=%s station=%s type=%s err=%v",
				req.RequestID, req.StationID, req.Type, err)
			return result, err
		}
		result.OK = true
		result.Message = "command sent"
		log.Printf("iec104 send command done request_id=%s station=%s type=%s",
			req.RequestID, req.StationID, req.Type)
		return result, nil
	}
}

func (p *IEC104Plugin) connectStation(ctx context.Context, cfg contract.StationConfig) error {
	settings := iecc.NewSettings()
	settings.Host = cfg.Host
	settings.Port = cfg.Port
	settings.AutoConnect = cfg.AutoConnect
	settings.ReconnectInterval = 5 * time.Second
	if debugEnabled() {
		settings.LogCfg = &iecc.LogCfg{Enable: true}
		log.Printf("iec104 debug log enabled station=%s endpoint=%s:%d", cfg.StationID, cfg.Host, cfg.Port)
	}

	call := &stationCall{plugin: p, stationID: cfg.StationID}
	client := iecc.New(settings, call)
	client.SetServerActiveHandler(func(c *iecc.Client) {
		p.setStationReady(cfg.StationID, true)
		_ = c.SendInterrogationCmd(cfg.CommonAddr)
	})
	client.SetConnectionLostHandler(func(_ *iecc.Client) {
		p.setStationReady(cfg.StationID, false)
	})

	p.mu.Lock()
	p.stations[cfg.StationID] = &stationRuntime{
		cfg:    cfg,
		client: client,
		ready:  false,
	}
	if _, ok := p.values[cfg.StationID]; !ok {
		p.values[cfg.StationID] = make(map[uint32]string)
	}
	p.mu.Unlock()

	if err := client.Connect(); err != nil {
		p.mu.Lock()
		delete(p.stations, cfg.StationID)
		p.mu.Unlock()
		return fmt.Errorf("connect station %s failed: %w", cfg.StationID, err)
	}

	if err := p.waitStationReady(ctx, cfg.StationID); err != nil {
		return fmt.Errorf("station %s not ready: %w", cfg.StationID, err)
	}
	return nil
}

func (p *IEC104Plugin) closeStation(stationID string) error {
	return p.closeStationWithCtx(context.Background(), stationID)
}

func (p *IEC104Plugin) closeStationWithCtx(_ context.Context, stationID string) error {
	p.mu.Lock()
	rt, ok := p.stations[stationID]
	if ok {
		delete(p.stations, stationID)
	}
	p.mu.Unlock()
	if !ok {
		return nil
	}
	if err := rt.client.Close(); err != nil {
		return fmt.Errorf("close station %s failed: %w", stationID, err)
	}
	return nil
}

func (p *IEC104Plugin) getStation(stationID string) (*stationRuntime, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	rt, ok := p.stations[stationID]
	if !ok {
		return nil, fmt.Errorf("station not found: %s", stationID)
	}
	return rt, nil
}

func (p *IEC104Plugin) setStationReady(stationID string, ready bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rt, ok := p.stations[stationID]
	if !ok {
		return
	}
	rt.ready = ready
}

func (p *IEC104Plugin) waitStationReady(ctx context.Context, stationID string) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		p.mu.RLock()
		rt, ok := p.stations[stationID]
		ready := ok && rt.ready
		p.mu.RUnlock()
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *IEC104Plugin) updateValue(stationID string, ioa uint32, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.values[stationID]; !ok {
		p.values[stationID] = make(map[uint32]string)
	}
	p.values[stationID][ioa] = value
}

func (p *IEC104Plugin) incrFail() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commandFail++
}

func decodePayload(raw []byte, defaultAddr uint16) (commandPayload, error) {
	p := commandPayload{CommonAddr: defaultAddr}
	if len(raw) == 0 {
		return p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("decode payload failed: %w", err)
	}
	if p.CommonAddr == 0 {
		p.CommonAddr = defaultAddr
	}
	return p, nil
}

func executeCommand(client *iecc.Client, typ string, payload commandPayload) error {
	switch typ {
	case "interrogation":
		return client.SendInterrogationCmd(payload.CommonAddr)
	case "counter_interrogation":
		return client.SendCounterInterrogationCmd(payload.CommonAddr)
	case "clock_sync":
		ts := time.Now()
		if payload.Timestamp != "" {
			parsed, err := time.Parse(time.RFC3339, payload.Timestamp)
			if err != nil {
				return err
			}
			ts = parsed
		}
		return client.SendClockSynchronizationCmd(payload.CommonAddr, ts)
	case "read":
		return client.SendReadCmd(payload.CommonAddr, payload.IOA)
	case "reset_process":
		return client.SendResetProcessCmd(payload.CommonAddr)
	case "test":
		return client.SendTestCmd(payload.CommonAddr)
	case "single_cmd", "double_cmd", "step_cmd", "setpoint_normal", "setpoint_scaled", "setpoint_float", "bitstring32_cmd":
		if payload.TypeID == 0 {
			return fmt.Errorf("type_id is required for %s", typ)
		}
		return client.SendCmd(payload.CommonAddr, asdu.TypeID(payload.TypeID), asdu.InfoObjAddr(payload.IOA), payload.Value)
	default:
		return fmt.Errorf("unsupported command type: %s", typ)
	}
}

func (c *stationCall) OnInterrogation(packet *asdu.ASDU) error {
	log.Printf("iec104 recv interrogation station=%s cot=%d type=%d", c.stationID, packet.Coa.Cause, packet.Type)
	return nil
}

func (c *stationCall) OnCounterInterrogation(packet *asdu.ASDU) error {
	log.Printf("iec104 recv counter_interrogation station=%s cot=%d type=%d", c.stationID, packet.Coa.Cause, packet.Type)
	return nil
}

func (c *stationCall) OnRead(packet *asdu.ASDU) error { return c.OnASDU(packet) }

func (c *stationCall) OnTestCommand(packet *asdu.ASDU) error {
	log.Printf("iec104 recv test station=%s cot=%d type=%d", c.stationID, packet.Coa.Cause, packet.Type)
	return nil
}

func (c *stationCall) OnClockSync(packet *asdu.ASDU) error {
	log.Printf("iec104 recv clock_sync station=%s cot=%d type=%d", c.stationID, packet.Coa.Cause, packet.Type)
	return nil
}

func (c *stationCall) OnResetProcess(packet *asdu.ASDU) error {
	log.Printf("iec104 recv reset_process station=%s cot=%d type=%d", c.stationID, packet.Coa.Cause, packet.Type)
	return nil
}

func (c *stationCall) OnDelayAcquisition(*asdu.ASDU) error { return nil }

func (c *stationCall) OnASDU(packet *asdu.ASDU) error {
	log.Printf("iec104 recv asdu station=%s cot=%d type=%d", c.stationID, packet.Coa.Cause, packet.Type)
	switch iecc.GetDataType(packet.Type) {
	case iecc.SinglePoint:
		for _, p := range packet.GetSinglePoint() {
			c.plugin.updateValue(c.stationID, uint32(p.Ioa), fmt.Sprintf("%t", p.Value))
		}
	case iecc.DoublePoint:
		for _, p := range packet.GetDoublePoint() {
			c.plugin.updateValue(c.stationID, uint32(p.Ioa), fmt.Sprintf("%d", p.Value))
		}
	case iecc.MeasuredValueScaled:
		for _, p := range packet.GetMeasuredValueScaled() {
			c.plugin.updateValue(c.stationID, uint32(p.Ioa), fmt.Sprintf("%d", p.Value))
		}
	case iecc.MeasuredValueNormal:
		for _, p := range packet.GetMeasuredValueNormal() {
			c.plugin.updateValue(c.stationID, uint32(p.Ioa), fmt.Sprintf("%v", p.Value))
		}
	case iecc.MeasuredValueFloat:
		for _, p := range packet.GetMeasuredValueFloat() {
			c.plugin.updateValue(c.stationID, uint32(p.Ioa), fmt.Sprintf("%f", p.Value))
		}
	case iecc.IntegratedTotals:
		for _, p := range packet.GetIntegratedTotals() {
			c.plugin.updateValue(c.stationID, uint32(p.Ioa), fmt.Sprintf("%d", p.Value.CounterReading))
		}
	}
	return nil
}

func debugEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("IEC104_DEBUG")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
