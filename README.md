# Profile Service

Profile 服务负责用户档案、收藏/点赞、观看历史三大核心领域的数据主权。仓库遵循 Kratos + MVC 结构，所有业务逻辑都通过 Service 层协调仓储与 Outbox/Inbox 任务。

## 快速索引
- 架构与数据模型：`ARCHITECTURE.md`
- 重构执行计划：`profile_refactor_plan.md`
- Pub/Sub & 投影：`docs/README.md`（集中了投影方案、事件规范、GCP 设置）

## 程序入口
- gRPC 服务：`cmd/grpc`
- Outbox 发布器：`cmd/tasks/outbox`
- Catalog Inbox Runner：`cmd/tasks/catalog_inbox`

## 环境前置
- Go 1.22+
- Docker Desktop（Testcontainers 启动所需，运行 `make test` 前需确保 Docker 正常运行）
- `mockgen` 工具：`go install github.com/golang/mock/mockgen@latest`（供 `go generate ./internal/services/mocks` 使用）
- 所有运行时配置以 `configs/config.yaml` 为唯一来源（可按环境复制 `config.$ENV.yaml` 覆盖）；`.env` 仅保留 `DATABASE_URL` 等敏感信息，不再驱动业务开关。

## 常用命令
```bash
# 运行静态检查
make lint

# 运行全部测试（包含 Testcontainers 集成测试，需本地 Docker 环境）
make test

# 更新 GoMock 仓储桩（服务/仓储接口变更后执行）
go generate ./internal/services/mocks

# 启动 gRPC 服务
make build && ./bin/grpc -conf configs/config.yaml

# 启动 Outbox 发布器任务
go run ./cmd/tasks/outbox -conf configs/config.yaml

# 启动 Catalog Inbox Runner（消费 catalog.video.* 并刷新投影）
go run ./cmd/tasks/catalog_inbox -conf configs/config.yaml
```

## 可观测性
- 统一使用 `lingo-utils/observability` 初始化 OpenTelemetry Tracer/Meter Provider，配置位于 `configs/config.yaml` 的 `observability` 段。
- 默认启用追踪与指标导出，使用 `stdout` 便于本地调试；若接入云端/自建 Collector，可将 `exporter` 调整为 `otlp_grpc` 并设置 `endpoint`/`headers`。
- gRPC 服务与独立任务进程（Outbox/Catalog Inbox）均复用同一套配置，确保 Outbox、Inbox、gRPC 指标落入统一 Meter Provider。
- 新增的领域事件指标（收藏/观看 Outbox enqueue、Watch Progress 发布链路）会自动写入 OTel 指标，可通过调整 `observability.metrics.interval` 控制推送频率。

## 测试说明
- Service 层集成测试位于 `internal/services/test`，依赖 Testcontainers 启动 Postgres。
- Controller 层 gRPC 单测位于 `internal/controllers/test`，覆盖元数据解析与错误映射。
- 如果本地未运行 Docker，Testcontainers 将无法启动，请在执行测试前确保 Docker/Colima 已经准备就绪。

如需新增文档或任务，请同步更新本 README 的索引，确保同一信息只维护一份。
