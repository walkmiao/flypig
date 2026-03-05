# Phase 1（两周）落地任务清单

目标：先打通“多实例自动分片 + 自动故障接管”的最小闭环。

## 0. 技术基线（GoFrame）

- 框架：GoFrame v2
- 仓库模式：建议 `MonoRepo`
- 骨架创建：优先使用 `gf init`（当前环境未安装 `gf`，需先安装 CLI）

建议目录：
- `app/control-plane`（GoFrame HTTP/gRPC）
- `app/data-plane`（采集与分片执行）
- `manifest/config`（配置文件）
- `utility`（共享库）

## 1. 里程碑

### M1（Day 1-3）控制面基础
- [ ] 初始化 GoFrame Monorepo 骨架（`gf init <name> -m`）
- [ ] 在 `app/control-plane` 建立 API 与 Scheduler 启动骨架
- [ ] 建库与表：`endpoint`、`point_table`、`point_item`、`shard`、`shard_lease`
- [ ] 实现 LeaseStore（建议 etcd）
- [ ] 实现实例注册与心跳表/缓存

验收：可写入分片、可写入租约、可读实例健康。

### M2（Day 4-6）调度最小闭环
- [ ] Scheduler 周期任务（30s）
- [ ] 初始分配：按 `active_shards` 最小分配
- [ ] 故障接管：`lease_expire` 后自动重分配
- [ ] Rebalance：负载差>1 时迁移 1 个分片

验收：起 2 个实例，停掉 1 个后 30 秒内分片迁移完成。

### M3（Day 7-10）数据面分片状态机
- [ ] 扩展插件契约：`DrainShard`
- [ ] 实现状态机：`INIT->RUNNING->DRAINING->DRAINED->REVOKED`
- [ ] 实现 Draining 判定：`inflight=0 && wal_depth=0`

验收：滚动升级时分片可优雅迁移，无硬断连。

### M4（Day 11-14）观测与压测
- [ ] 指标：`active_conn/send_lag_ms/wal_depth/draining_shards`
- [ ] 告警规则（基础）
- [ ] 压测脚本：100~1000 连接规模验证

验收：输出压测报告（吞吐、延迟、迁移时长、失败率）。

## 2. 模块拆分（代码层）

## 2.1 control-plane
建议目录：
- `app/control-plane/internal/controller`
- `app/control-plane/internal/service`
- `app/control-plane/internal/scheduler`
- `app/control-plane/internal/lease`
- `app/control-plane/internal/repo`

职责：
- API：配置与分片管理接口
- Scheduler：分片分配/故障接管/rebalance
- Lease：租约读写（etcd）
- Repo：配置元数据读写（PostgreSQL）

## 2.2 data-plane（你现有 plugin 进程）
建议目录：
- `app/data-plane/internal/controller`
- `app/data-plane/internal/service`
- `app/data-plane/internal/service/runtime/`（协议连接实现可复用）
- `app/data-plane/internal/shardstate`
- `app/data-plane/internal/metrics`

职责：
- runtime：协议连接与命令
- shardstate：状态机与 draining 执行
- metrics：健康指标采集上报

## 2.3 shared contract
- `app/data-plane/internal/service/contract/contract.go` 扩展字段与接口

## 3. 第一阶段必须改动（最小接口）

在 `contract.Plugin` 增加：

```go
DrainShard(ctx context.Context, shardID string, deadline time.Time) error
```

在 `HealthSnapshot` 增加：

```go
SendLagMs      int64
WALDepth       int64
DrainingShards int
CPUPercent     float64
MemPercent     float64
```

在 `ShardAssignment` 增加：

```go
Prewarm bool
```

## 4. 最小 API 设计（Control Plane）

### 4.1 HTTP
- `POST /api/v1/shards/reconcile`
  - 触发一次调度（初始分配+故障接管+rebalance）
- `POST /api/v1/shards/{id}/drain`
  - 请求旧 owner 排空
- `GET /api/v1/instances/health`
  - 查看实例健康

### 4.2 内部 gRPC（Control -> Data）
- `AssignShard(ShardAssignment)`
- `DrainShard(DrainRequest)`
- `RevokeShard(RevokeRequest)`
- `Health(Empty)`

## 5. 数据库最小 DDL（可直接落地）

```sql
create table if not exists shard (
  shard_id        varchar(64) primary key,
  status          varchar(16) not null,
  point_table_id  bigint not null,
  updated_at      timestamptz not null default now()
);

create table if not exists shard_lease (
  shard_id        varchar(64) primary key,
  owner_id        varchar(128) not null,
  lease_expire_at timestamptz not null,
  version         bigint not null,
  updated_at      timestamptz not null default now()
);

create index if not exists idx_lease_expire on shard_lease(lease_expire_at);
```

## 6. 自动负载均衡实现策略（Phase 1 简化版）

### 6.1 分配规则
- 只在健康实例里分配。
- 选择 `active_shards` 最小实例。
- 同分片迁移期间（DRAINING/MIGRATING）禁止再次迁移。

### 6.2 故障接管
- 触发条件：
  - `instance heartbeat timeout` 或
  - `lease_expire_at < now()`
- 动作：
  1. 选新 owner
  2. `ClaimShard`
  3. 对新 owner 发 `AssignShard`

### 6.3 Rebalance
- 条件：`max_active_shards - min_active_shards > 1`
- 每轮只迁移 1 个 shard，避免抖动。

## 7. 两周执行清单（按天）

- Day 1: 安装 `gf` + 初始化 Monorepo + 建表脚本
- Day 2: control-plane 骨架 + LeaseStore 接口定义
- Day 3: etcd LeaseStore 实现 + 实例健康 API 雏形
- Day 4: Scheduler 初始分配
- Day 5: 故障接管逻辑
- Day 6: Rebalance + 冷却时间
- Day 7: `DrainShard` 契约扩展 + grpcplugin 桥接
- Day 8: runtime 状态机实现（含 DRAINING）
- Day 9: host-loader 演练脚本（assign/drain/revoke）
- Day10: 指标扩展 + 健康输出
- Day11: 2 实例联调
- Day12: 3 实例联调 + 故障注入
- Day13: 压测 + 参数调优
- Day14: 回归 + 发布文档

## 8. 完成定义（Definition of Done）

- [ ] 3 个采集实例同时运行，分片自动均衡。
- [ ] 任意 1 个实例下线，30 秒内分片自动接管。
- [ ] 升级过程支持 `drain -> revoke`，无批量丢数告警。
- [ ] 有基础观测面板（连接数、滞后、WAL 深度、分片状态）。

## 9. 你现在就可以执行的第一步

1. 建表并起 etcd。
2. 在 `contract` 增加 `DrainShard` 与 Health 扩展字段。
3. 在 `app/data-plane/internal/service/grpcplugin` 补上 `DrainShard` 转发。
4. 在 `runtime` 做 `DRAINING` 最小实现。
5. 在 `app/data-plane/internal/service/orchestrator` 里把迁移改成 `drain->claim->revoke`。
