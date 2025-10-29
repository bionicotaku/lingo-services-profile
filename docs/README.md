# Profile Service 文档概览（MVP）

> 本文列出 Profile 微服务的核心参考资料，便于快速定位架构说明、任务 Runner 与数据库结构。所有文档均需与 `profile_refactor_plan.md`、`ARCHITECTURE.md` 保持一致。

## 1. 核心架构
- `../ARCHITECTURE.md` – 微服务职责、数据模型、Outbox/Inbox 工作流以及后台任务清单。
- `../profile_refactor_plan.md` – 当前重构任务的执行计划与进度追踪。

## 2. 事件与投影
- `docs/只读投影方案.md` – Catalog → Profile 投影流程、版本幂等策略、错误处理规范。
- `docs/投影一致性问题解决方案.md` – Inbox 处理顺序、补偿策略、死信处理方案。
- `docs/pubsub-conventions.md` – 事件命名、属性字段、幂等约束。
- `docs/gcp-pubsub-setup.md` – 本地/云端 Pub/Sub 准备与权限配置说明。

## 3. 后台任务
- `cmd/tasks/outbox/` – Outbox 发布器可执行入口，负责推送 `profile.engagement.*`、`profile.watch.progressed`。
- `cmd/tasks/catalog_inbox/` – Catalog Inbox Runner 入口，消费 `catalog.video.*` 并刷新 `profile.videos_projection`。
- `internal/tasks/catalog_inbox/` – Inbox Runner 实现与集成测试（Testcontainers）。

## 4. 测试参考
- `internal/services/test/` – 以真实仓储验证 Profile/Engagement/WatchHistory 服务的关键语义。
- `internal/controllers/test/` – Profile gRPC Handler 的参数校验、错误映射单测。

> 如需更新或新增文档，请保持结构简洁、与现有信息一致，并在 README 中同步登记。
