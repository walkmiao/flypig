# go-plugin gRPC 模板（IEC104）

这个目录演示：

- 如何用 `hashicorp/go-plugin` + gRPC 承载 IEC104 插件进程。
- 宿主如何加载插件并调用统一契约接口。

## 目录

- `contract/`: 插件契约定义（分片、健康、命令）
- `grpcplugin/`: go-plugin 的 gRPC 桥接层
- `runtime/`: 插件实现（`IEC104Plugin`，含真实 IEC104 收发）与 `MockPlugin`
- `cmd/iec104-plugin`: 插件进程入口
- `cmd/host-loader`: 宿主加载器入口

## 快速运行

先进入模板子模块目录：

```bash
cd examples/plugin-ha-template
go build -o /tmp/iec104-plugin ./cmd/iec104-plugin
go run ./cmd/host-loader /tmp/iec104-plugin
```

期望输出包含：

- `health: alive=true ...`
- `command: ok=true ...`

如需指定站点地址，可设置：

- `IEC104_HOST`（默认 `127.0.0.1`）
- `IEC104_PORT`（默认 `2404`）
- `IEC104_COMMON_ADDR`（默认 `1`）

## 如何接入真实 IEC104 连接

## CommandRequest.Payload 约定

`SendCommand` 使用 `Type` + JSON `Payload`：

```json
{
  "common_addr": 1,
  "ioa": 1001,
  "type_id": 45,
  "value": true,
  "timestamp": "2026-03-05T10:00:00+08:00"
}
```

- `interrogation` / `counter_interrogation` / `clock_sync` / `read` / `reset_process` / `test`
- `single_cmd` / `double_cmd` / `step_cmd` / `setpoint_normal` / `setpoint_scaled` / `setpoint_float` / `bitstring32_cmd`

其中后 7 种需要 `type_id`（IEC104 TypeID）与 `ioa`，并按类型提供 `value`。
