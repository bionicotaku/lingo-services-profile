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

## 常用命令
```bash
# 运行静态检查
make lint

# 运行全部测试（包含 Testcontainers 集成测试，需本地 Docker 环境）
make test

# 启动 gRPC 服务
make build && ./bin/grpc -conf configs/config.yaml

# 启动 Outbox 发布器任务
go run ./cmd/tasks/outbox -conf configs/config.yaml

# 启动 Catalog Inbox Runner（消费 catalog.video.* 并刷新投影）
go run ./cmd/tasks/catalog_inbox -conf configs/config.yaml
```

## 测试说明
- Service 层集成测试位于 `internal/services/test`，依赖 Testcontainers 启动 Postgres。
- Controller 层 gRPC 单测位于 `internal/controllers/test`，覆盖元数据解析与错误映射。

如需新增文档或任务，请同步更新本 README 的索引，确保同一信息只维护一份。
