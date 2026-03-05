# plugin-ha-template 架构梳理

## 1. 项目定位
本项目是一个 `hashicorp/go-plugin + gRPC` 的 IEC104 插件化模板，目标是把 IEC104 通信能力下沉到独立插件进程，由宿主进程进行生命周期管理、分片分配、健康探测和命令调用。

核心价值：
- 进程隔离：IEC104 连接与协议处理在插件进程内运行。
- 统一契约：宿主与插件通过 `contract.Plugin` 接口交互。
- 可扩展：内置 `orchestrator` 与 `LeaseStore` 抽象，支持高可用与故障转移策略。

## 2. 目录与职责
- `app/data-plane/internal/service/hostloader`
  - 宿主加载服务，负责拉起插件进程并调用契约接口。
- `app/data-plane/internal/service/pluginproc`
  - 插件进程服务，负责通过 `hplugin.Serve` 暴露插件实现。
- `app/data-plane/internal/service/contract`
  - 跨进程共享的领域契约：`StationConfig`、`ShardAssignment`、`HealthSnapshot`、`CommandRequest/Result`、`Plugin`、`LeaseStore`。
- `app/data-plane/internal/service/grpcplugin`
  - go-plugin 的 gRPC 桥接层。
  - 提供 `Handshake`、插件集注册、gRPC server/client 双端适配。
  - 使用自定义 `jsonCodec` 做序列化。
- `app/data-plane/internal/service/runtime`
  - 插件运行时实现。
  - `IEC104Plugin`：真实 IEC104 客户端连接、ASDU 回调解析、命令发送、健康统计。
  - `MockPlugin`：用于测试/演示的简化实现。
- `app/data-plane/internal/service/orchestrator`
  - 分片调度逻辑（与插件进程解耦）。
  - `FailoverLostOwners`：故障接管。
  - `Rebalance`：负载重平衡。

## 3. 进程与边界
### 3.1 进程模型
- 宿主进程：`app/data-plane` 子命令 `host-loader`
- 插件进程：`app/data-plane` 子命令 `plugin`
- 站点端：IEC104 设备/站点（TCP）

### 3.2 通信链路
1. 宿主通过 `exec.Command(pluginPath)` 拉起插件进程。
2. 双方通过 go-plugin 握手（Magic Cookie + ProtocolVersion）。
3. 宿主拿到 `contract.Plugin` 客户端代理（`grpcClient`）。
4. 方法调用以 gRPC unary 请求跨进程转发到插件实现（`grpcServer -> IEC104Plugin`）。
5. 插件内部通过 `github.com/wendy512/iec104/client` 与 IEC104 站点通信。

## 4. 核心调用流程
### 4.1 启动与装载
1. `host-loader` 读取环境变量：
   - `IEC104_HOST`（默认 `127.0.0.1`）
   - `IEC104_PORT`（默认 `2404`）
   - `IEC104_COMMON_ADDR`（默认 `1`）
2. 创建 go-plugin `Client`，`Dispense("iec104")` 获取插件。
3. 调用 `p.Start(ctx)` 初始化插件运行态。

### 4.2 分片分配
1. 宿主构造 `ShardAssignment`（含 `Stations` 列表）。
2. 调用 `AssignShard`。
3. 插件对每个 `StationConfig` 执行 `connectStation`：
   - 初始化 `iecc.Client`。
   - 注册回调：`SetServerActiveHandler` / `SetConnectionLostHandler`。
   - 连接成功后等待 `ready`。

### 4.3 健康检查
1. 宿主调用 `Health`（示例中启动后即调一次，并每 10 秒轮询）。
2. 插件返回：
   - `Alive`
   - `ActiveShards`
   - `ActiveConn`（`client.IsConnected()` 统计）
   - `CommandFailRate`（`commandFail / commandTotal`）

### 4.4 命令下发
1. 宿主调用 `SendCommand`，传入：
   - `Type`（如 `interrogation`）
   - `Payload`（JSON）
   - `Timeout`
2. 插件流程：
   - `decodePayload` 解析并填补 `common_addr` 默认值。
   - `waitStationReady` 等待站点可用。
   - 启动 goroutine 执行 `executeCommand`，并受 `Timeout` 控制。
3. 支持命令类型：
   - 基础：`interrogation` / `counter_interrogation` / `clock_sync` / `read` / `reset_process` / `test`
   - 带 `type_id`：`single_cmd` / `double_cmd` / `step_cmd` / `setpoint_normal` / `setpoint_scaled` / `setpoint_float` / `bitstring32_cmd`

### 4.5 停止与回收
1. 宿主接收 `SIGINT/SIGTERM` 后调用 `Stop`。
2. 插件关闭所有站点连接并清空内存状态（`shards/stations/values`）。

## 5. 关键数据结构
- `StationConfig`：单站点连接参数与自动连接开关。
- `ShardAssignment`：分片归属、站点集合、租约到期时间、版本号。
- `HealthSnapshot`：实例健康与负载指标。
- `CommandRequest/CommandResult`：控制面命令输入输出。
- `LeaseStore`：外部租约存储抽象（为 HA/故障切换提供持久化状态）。

## 6. 运行时状态与并发模型
- `IEC104Plugin` 内部状态：
  - `shards map[string]ShardAssignment`
  - `stations map[string]*stationRuntime`
  - `values map[string]map[uint32]string`
  - `commandTotal/commandFail`
- 并发控制：
  - 使用 `sync.RWMutex` 保护全部共享状态。
  - 回调线程与控制面调用共享同一状态容器。
- 就绪判定：
  - `waitStationReady` 每 50ms 轮询 `stationRuntime.ready`。

## 7. 协议桥接实现要点
- `app/data-plane/internal/service/grpcplugin/plugin.go`
  - `IEC104GRPCPlugin` 同时实现 `GRPCServer` 与 `GRPCClient`。
  - server 侧将 gRPC 调用转发到 `contract.Plugin` 实现。
  - client 侧通过 `cc.Invoke` 代理远程方法。
- `app/data-plane/internal/service/grpcplugin/service.go`
  - 手工注册 `grpc.ServiceDesc` 和方法 handler，避免依赖 `.proto` 生成代码。
- `app/data-plane/internal/service/grpcplugin/codec.go`
  - 注册 JSON 编解码器，并通过 `grpc.ForceCodec(jsonCodec{})` 指定。

## 8. 编排层（orchestrator）设计
### 8.1 FailoverLostOwners
- 输入：当前实例健康快照 + `LeaseStore` 中分片列表。
- 行为：
  - owner 不存活或租约过期时，迁移到当前 `ActiveShards` 最低的实例。
  - 使用 `ClaimShard` 完成接管。

### 8.2 Rebalance
- 当活跃实例数 >=2 且 `maxLoad - minLoad > 1` 时触发。
- 从最重实例释放一个分片，再由最轻实例声明，完成一次最小迁移。

## 9. 配置项
- `IEC104_HOST`：默认 `127.0.0.1`
- `IEC104_PORT`：默认 `2404`
- `IEC104_COMMON_ADDR`：默认 `1`
- `IEC104_DEBUG`：`1/true/yes/on` 时启用 IEC104 底层日志

## 10. 已知边界与改进方向
- 当前 `InstanceID` 为硬编码字符串，生产场景应改为可配置或自动生成。
- `waitStationReady` 使用轮询，可改为事件/条件变量以降低空转。
- `values` 仅写入无对外读取接口，可考虑增加查询/订阅能力。
- gRPC 采用 JSON codec，性能与类型安全弱于 protobuf 生成模型。
- `orchestrator` 仅提供调度算法，未与真实插件实例控制通道闭环（实际系统需补齐）。

## 11. 一句话总结
该模板实现了“宿主控制面 + 插件数据面 + IEC104站点连接”的最小可运行闭环，并通过契约抽象与分片编排接口为 HA 场景预留了扩展点。
