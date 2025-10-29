# Profile Service 全量重构方案（草案 · 2025-10-29）

> 目标：按《services-profile/ARCHITECTURE.md》定义的 Profile 领域职责，完成从「Catalog 视频模板」到「用户档案/互动/观看历史」的全面重构。采用“先增量引入新业务，再安全移除旧视频代码”的策略，确保目录骨架与模板一致，同时逐步替换业务实现。

---

## 1. 范围与验收标准

### 1.1 重构范围

- **数据模型**：新增 `profile.users`、`profile.engagements`、`profile.watch_logs`、`profile.videos_projection`、`profile.video_stats`、`profile.outbox_events`、`profile.inbox_events` 表，并迁移/拆除旧的 `catalog.*` schema 依赖。
- **API 契约**：重新定义 Profile 专属 gRPC/REST 契约（GetProfile、MutateFavorite、UpsertWatchProgress 等），替换现有 `CatalogQueryService`/`CatalogLifecycleService`。
- **服务分层**：重写 Controller/Service/Repository/Tasks 以匹配 Profile 领域模型；保留 kratos-template 的基础设施（配置、Wire 装配、Observability、Outbox 框架）。
- **异步事件**：发布 `profile.engagement.*`、`profile.watch.progressed`，消费 `catalog.video.*` 并维护 `profile.videos_projection`。

### 1.2 成功标准

1. **架构一致**：服务内部目录继续符合 `internal/{controllers,services,repositories,models,clients,tasks}` 规范，且导出接口与《ARCHITECTURE.md》字段/事件一一对应。
2. **契约通过**：`buf lint && buf breaking`、`spectral lint`、`go test ./...`、`make lint` 全部通过；服务层新增代码覆盖率 ≥ 80%。
3. **数据安全迁移**：引入 Profile schema 的迁移脚本能幂等执行；同时保留 catalog 数据直至切换完成；切换窗口内支持灰度（新旧 API 并存）。
4. **事件链路**：Outbox 发布与 Inbox 投影在本地 Pub/Sub 或进程内模式下打通，提供集成测试。
5. **上线回滚**：可在配置层回退到旧 API（若未删除），或通过 feature flag 禁用新端点。

---

## 2. 契约与接口设计

### 2.1 Proto 目录调整

文件：`api/profile/v1/profile.proto`（新建）  
拆分模块：

- `ProfileService`（gRPC）：
  - `GetProfile`, `UpdateProfile`, `UpdatePreferences`
  - `MutateFavorite`, `BatchQueryFavorite`
  - `UpsertWatchProgress`, `ListWatchHistory`
  - `ListFavorites`, `PurgeUserData`
- 公共消息：
  - `Profile`（`profile_version`, `display_name`, `avatar_url`, `preferences_json`）
  - `PreferenceDelta`, `FavoriteState`, `WatchProgress`
  - `VideoStats`（含 `like_count`, `bookmark_count`, `unique_watchers`, `total_watch_seconds`）

### 2.2 REST 映射

- Gateway 暴露 `/api/v1/profile/*`、`/api/v1/user/*`，按文档定义的 Problem Details 语义实现。
- 旧的 `/api/v1/video/*` 端点在迁移完成后下线；迁移期启用 feature flag 切换路由。

### 2.3 事件 Schema

- 新建 `api/events/profile/v1/*.json` 描述 `profile.engagement.added/removed`、`profile.watch.progressed` Payload。
- 发布流程复用 `lingo-utils/outbox`，消费侧（可能是 Catalog/Feed/Report）将以 JSON Schema 验证。

---

## 3. 数据模型与迁移策略

### 3.1 新增迁移脚本

目录：`migrations/101_create_profile_schema.sql`（以 100+ 序号置于旧 catalog 前，便于并行运行）

- 创建 `profile` schema。
- 建表顺序：
  1. `profile.users`
  2. `profile.engagements`
  3. `profile.watch_logs`
  4. `profile.videos_projection`
  5. `profile.video_stats`
  6. `profile.outbox_events`（复制模板 `002_create_catalog_event_tables.sql`，替换 schema/索引名）
  7. `profile.inbox_events`
- 安装通用触发器函数 `profile.tg_set_updated_at()`。
- 配置 RLS（MVP 可在迁移文件中创建 policy 草案，但默认禁用，等服务切换后启用）。

### 3.2 Catalog → Profile 投影过渡

- 保留现有 catalog schema 迁移不变，直到新 Profile 服务上线；`profile.videos_projection` 通过 Inbox 同步 Catalog 事件。
- `video_user_engagements_projection` 表（catalog schema）在新 Profile 生效后废弃；迁移期保留但不再写入。

### 3.3 数据清理计划

- 分阶段：
  1. 上线新 Profile 表后，冻结旧表写入（停止 engagement runner）。
  2. 导出 catalog 投影与用户态数据，迁移/转换为 Profile schema（可选离线脚本）。
  3. 完成验证后 drop 旧 catalog 相关表，或保留只读备份表 `catalog.videos_legacy` 供回滚。

---

## 4. 控制器与 DTO 设计

### 4.1 新控制器目录

```
internal/controllers/
├── profile_handler.go        // 档案 + 偏好
├── engagement_handler.go     // 收藏/点赞
├── watch_handler.go          // 观看历史
├── video_projection_handler.go (只读补水接口，供内部调用)
└── dto/
    ├── profile.go
    ├── engagement.go
    ├── watch.go
    └── pagination.go
```

### 4.2 功能要点

- `BaseHandler` 保留；扩展 `HandlerTypeCommand` 超时时间配置。
- DTO 层负责验证字段、抽取 metadata (`x-apigateway-api-userinfo`)、生成 Problem Details。
- REST 层（若 Gateway 直连）将通过 gRPC Adapter 暴露一致行为；此处聚焦 gRPC Handler。

### 4.3 兼容旧 Handler

- 在新文件加入前保留 `video_query_handler.go` 等旧 Handler；待新 API 灰度通过后统一删除。
- Wire 中可通过 feature flag 注入不同 Handler 集合（`ProvideGRPCServer` 根据配置决定注册哪些服务）。

---

## 5. 服务层重建

### 5.1 服务组件划分

- `ProfileService`：管理 `profile.users`，负责档案/偏好乐观锁 (`profile_version`)、偏好差异计算、Outbox 事件（Post-MVP）。
- `EngagementService`：负责点赞/收藏写入（`profile.engagements` + `profile.video_stats`）、发布 `profile.engagement.*` 事件、缓存失效。
- `WatchHistoryService`：维护 `profile.watch_logs`、累计 `total_watch_seconds`、按 5% 阈值发布 `profile.watch.progressed`。
- `VideoProjectionService`：消费 Catalog 事件，维护 `profile.videos_projection`，提供内部查询。
- `VideoStatsService`：聚合/读取 `profile.video_stats`（提供 Query 级别的统计补水）。

### 5.2 事务与幂等

- 所有写路径通过 `txmanager.Manager.WithinTx` 进行事务控制，并在事务内写 Outbox。
- `EngagementService` 使用 `INSERT ... ON CONFLICT` 复合主键 `(user_id, video_id, engagement_type)`，软删除代表撤销，必要时写 Outbox。
- `WatchHistoryService` 在更新 `profile.watch_logs` 时维护 `expires_at = now() + retention_days`，并写 `video_stats.unique_watchers` / `total_watch_seconds`。

### 5.3 缓存与扩展

- 默认启用内存 LRU（per-instance）；接口预留 `Cache` 抽象，后续可替换 Redis。
- WatchHistory 可选批量更新模式（留 TODO）。

---

## 6. 仓储与 SQLC 生成

### 6.1 SQLC 目录重构

```
sqlc/
├── schema/
│   ├── 101_profile_schema.sql          // 与 migrations 一致，供 sqlc 引用
│   └── 102_profile_views.sql           // 衍生视图（如分页辅助）
├── profile/                            // 新生成代码（package profiledb）
│   ├── engagement.sql
│   ├── users.sql
│   ├── watch_logs.sql
│   ├── videos_projection.sql
│   ├── video_stats.sql
│   └── outbox_inbox.sql
└── catalog_legacy/                     // 迁移期保留旧查询，标记待移除
```

### 6.2 Repository 接口

- `ProfileRepository`：`Get`, `Upsert`, `IncrementVersion`.
- `EngagementRepository`：`Upsert`, `SoftDelete`, `ListByUser`, `BatchGet`.
- `WatchLogRepository`：`UpsertProgress`, `ListRecent`, `PruneExpired`.
- `VideoProjectionRepository`：`UpsertFromCatalogEvent`, `ListByIDs`.
- `VideoStatsRepository`：`Increment`（点赞/收藏/观看）、`Get`, `BatchGet`.

### 6.3 旧仓储移除计划

- 第一阶段：保留旧 `video_repo`、`video_user_state_repo` 与新仓储并存；新服务不使用旧仓储。
- 第二阶段：新 API 落地并稳定后，删除旧仓储、SQLC 生成文件、`migrations/00X_catalog_*`（保留备份）。

---

## 7. 异步任务与事件

### 7.1 Outbox Publisher

- Runner 沿用模板 `internal/tasks/outbox`，配置 `profile.outbox_events`。
- 发布事件类型：
  - `profile.engagement.added`
  - `profile.engagement.removed`
  - `profile.watch.progressed`
  - （Post-MVP）`profile.preferences.updated`

### 7.2 Inbox Consumer

- 新建 `internal/tasks/catalog_inbox`：
  - 订阅 `catalog.video.published`（或通配 `catalog.video.*`）。
  - Handler 对比事件 version，调用 `VideoProjectionService.Upsert`.
- Engagement Runner 替换为 Profile 版本：消费内部 topic（如 Replay/Report Service 可能写回）。

### 7.3 Watch Log Pruner（Post-MVP）

- 额外任务：周期性删除 `expires_at < now()` 的日志，并同步减少 `video_stats.total_watch_seconds` / `unique_watchers`（需保留原始增量，不在 MVP 内实现）。

---

## 8. 配置与基础设施

- `configs/config.yaml`：
  - `data.postgres.schema` 修改为 `profile`。
  - `messaging.pubsub.topic_id`/`subscription_id` 使用 Profile 专属名称（例如 `profile.events`）。
  - 新增 `messaging.catalog_inbox` 配置块。
- Wire：
  - 更新 `internal/infrastructure` Provider，注入新服务/仓储。
  - Feature flag：`features.enable_catalog_legacy` 决定是否注册旧 Handler。

---

## 9. 渐进式迁移策略

1. **阶段 A：基础设施到位**
   - 引入新 proto、迁移脚本、仓储层（不影响旧代码）。
   - 配置 schema=profile，数据库迁移上线。
2. **阶段 B：并行实现**
   - 增量开发新 Controller/Service/Repo。
   - Gateway 引入新路由（隐藏在 feature flag 下）。
3. **阶段 C：数据同步**
   - 启动 Inbox 同步 `profile.videos_projection`。
   - 导入历史收藏/观看数据至新表（脚本）。
4. **阶段 D：切流**
   - 打开新 API flag，监控 metrics（错误率、延迟、outbox/inbox lag）。
   - 收敛客户端到新接口。
5. **阶段 E：拆除旧实现**
   - 删除旧 proto、handler、service、repo、sqlc 生成。
   - Drop catalog 相关迁移（或迁移至 archive）。

---

## 10. 测试与验证

### 10.1 单元测试

- `internal/services/test/profile_service_test.go`
- `internal/services/test/engagement_service_test.go`
- `internal/services/test/watch_history_service_test.go`

### 10.2 仓储集成测试

- 使用 testcontainers PG，针对 `users`、`engagements`、`watch_logs`、`video_stats`、`videos_projection` 编写 CRUD 测试。

### 10.3 任务/事件测试

- Mock Pub/Sub（或使用 emulator）测试 Outbox 发布、Inbox 消费。
- Watch progress 事件节流（<5% 变动不触发）覆盖。

### 10.4 契约/端到端

- `buf lint && buf breaking`（新 proto）。
- `spectral lint` 校验 REST 文档更新。
- e2e 脚本：`test/e2e/profile_flow_test.sh`（注册档案 → 收藏/取消 → 观看进度 → 查询 Watch History）。

---

## 11. 任务拆解（执行列表 · 细项）

1. **契约与文档**
   - [ ] 创建 `api/profile/v1/profile.proto`（定义 RPC、消息、枚举、错误码）。
   - [ ] 新建 `api/profile/v1/events.proto`（Outbox 事件 payload）。
   - [ ] 调整 `buf.yaml`、`buf.gen.yaml` 引用新 proto；临时将 `api/video/v1` 移至 `api/_legacy/video/v1`。
   - [ ] 运行 `buf generate && gofumpt && goimports`，确保 `buf lint && buf breaking` 通过。
   - [ ] 更新 REST/OpenAPI 文档（若存在）：新增 Profile 端点、Problem 详情、示例请求。
   - [ ] 更新 `docs/api` 或 README 中的 API 索引链接。

2. **数据库迁移与 SQLC**
   - [ ] 编写 `migrations/101_create_profile_schema.sql`，包含全部表、索引、触发器、RLS TODO。
   - [ ] 将脚本拷贝到 `sqlc/schema/101_profile_schema.sql`，供 SQLC 使用。
   - [ ] 更新 `sqlc.yaml`：新增 profile 输出包（如 `internal/repositories/profiledb`），保留 catalog legacy 配置。
   - [ ] 运行 `sqlc generate`，验证新生成代码编译通过。
   - [ ] 编写数据迁移脚本（可选）：`tools/scripts/migrate_catalog_to_profile.sh`，用于迁移历史交互数据。

3. **模型层调整**
   - [ ] 在 `internal/models/po` 新增 `profile_user.go`、`profile_engagement.go`、`profile_watch_log.go`、`profile_video_projection.go`、`profile_video_stats.go`。
   - [ ] 在 `internal/models/vo` 新增相应视图对象与转换方法。
   - [ ] 更新 `internal/models/outbox_events`，添加 profile 事件常量、Payload struct、序列化逻辑。

4. **仓储实现与测试**
   - [ ] 新建 `internal/repositories/profile_users_repo.go`，实现档案读写与乐观锁。
   - [ ] 新建 `internal/repositories/profile_engagements_repo.go`，实现复合主键 UPSERT/软删、分页。
   - [ ] 新建 `internal/repositories/profile_watch_logs_repo.go`，实现进度写入、TTL、分页。
   - [ ] 新建 `internal/repositories/profile_video_projection_repo.go`，实现 Catalog 投影维护。
   - [ ] 新建 `internal/repositories/profile_video_stats_repo.go`，实现计数累加与读取。
   - [ ] 更新 `internal/repositories/init.go` 注入新仓储，旧视频仓储标注 `// TODO(legacy)`。
   - [ ] 编写集成测试（testcontainers）：针对上述仓储验证幂等、事务、索引行为。

5. **服务层重建**
   - [ ] 新建 `ProfileService`（档案/偏好），实现 `GetProfile`、`UpdateProfile`、`UpdatePreferences`、Profile 版本冲突处理。
   - [ ] 新建 `EngagementService`，处理点赞/收藏写入、事件发布、缓存失效。
   - [ ] 新建 `WatchHistoryService`，处理进度上报、5% 阈值判断、watch log TTL、视频统计累加。
   - [ ] 新建 `VideoProjectionService`，消费 Catalog 事件更新投影。
   - [ ] 新建 `VideoStatsService`，提供统计读取/补水接口。
   - [ ] 更新 `internal/services/init.go` 注入新服务；旧视频相关服务打上 feature flag。
   - [ ] 写服务单测（gomock 仓储 + fake clock/cache），覆盖成功/错误路径、事件发布逻辑。

6. **控制器与 DTO**
   - [ ] 新建 `profile_handler.go`、`engagement_handler.go`、`watch_handler.go`，注册新 gRPC 服务。
   - [ ] 在 `internal/controllers/dto` 创建 `profile.go`、`engagement.go`、`watch.go`、`pagination.go`，处理请求解析/验证。
   - [ ] `BaseHandler` 增加 Profile 专属 metadata 提取、幂等键辅助。
   - [ ] 更新 `internal/controllers/init.go` 和 `internal/infrastructure/grpc_server/grpc_server.go`，根据 feature flag 注册新旧 Handler。
   - [ ] 编写 Handler 单测（使用 gomock Service），覆盖 Problem Details / metadata / 超时。

7. **异步任务与事件链路**
   - [ ] 更新 `internal/tasks/outbox` 配置使用 `profile.outbox_events`，新增事件编码器/metrics 标签。
   - [ ] 新建 `internal/tasks/catalog_inbox` Runner（Pub/Sub Subscriber + Inbox Repo + VideoProjectionService）。
   - [ ] 调整/替换 `internal/tasks/engagement`：根据新事件语义重命名或废弃；若仍需消费外部事件，更新 handler。
   - [ ] 添加任务级测试：模拟消息、校验幂等、监控指标。

8. **配置、Wire、Feature Flag**
   - [ ] 更新 `configs/config.yaml`：`data.postgres.schema=profile`，新增 `messaging.catalog_inbox`，调整 topic/subscription，加入 feature 开关。
   - [ ] 同步 `.env`、`.env.example`、`.env.test`，新增 PROFILE_* 环境变量。
   - [ ] 更新 `cmd/grpc/wire.go` 注入新仓储/服务/任务，支持 feature flag。重新生成 `wire_gen.go`。
   - [ ] 评估缓存实现：若引入 Redis，新增配置与 init Provider；若仅 LRU，确保配置项可关闭。

9. **质量与验证**
   - [ ] `make lint`（含 gofumpt、goimports、staticcheck、revive、buf、spectral）。
   - [ ] `go test ./...`（确保服务/仓储/任务测试覆盖率目标达成）。
   - [ ] `sqlc generate`、`buf lint && buf breaking`、`spectral lint`、`make proto`（若依赖）。
   - [ ] 编写 e2e 脚本 `test/e2e/profile_flow_test.sh` 并运行一次完整流程。

10. **并行写与切流计划**
    - [ ] 在新服务中实现 catalog→profile 双写（可通过 feature flag 打开/关闭）。
    - [ ] 与 Gateway/Catalog/Feed 团队对齐事件消费与 API 切换时间表。
    - [ ] 配置监控仪表板，关注 outbox/inbox lag、错误率、统计数据对账。
    - [ ] 制定灰度策略（按 user_id / tenant 分批），记录回滚步骤。

11. **清理与文档**
    - [ ] 确认新 API 稳定后，删除旧 proto/handler/service/repo/sqlc/migrations，保留必要备份。
    - [ ] 更新 `services-profile/README.md`、`ARCHITECTURE.md` 反映新实现；在 `CHANGELOG` 或 release notes 记录重构信息。
    - [ ] 维护 `profile_refactor_plan.md` 勾选完成项，存档旧实现要点。

---

## 12. 风险与回滚策略

| 风险 | 描述 | 缓解/回滚 |
| --- | --- | --- |
| 数据迁移错误 | 批量导入旧收藏/观看数据可能出现缺失 | 先导入到临时表 `profile_tmp.*`，校验后再合并；保留 catalog 表以快速回滚 |
| 事件风暴 | Watch progress 事件过多造成 Outbox 堵塞 | 服务端限制 ≥5% 变动策略，Outbox worker 扩容；支持关闭事件发布的 feature flag |
| 缓存不一致 | 收藏状态缓存失效不及时 | 写操作后主动失效 + TTL；出现异常时可禁用缓存组件 |
| 依赖服务未就绪 | Catalog/Feed 等尚未消费新事件 | 部署前与其他团队对齐；保留旧事件输出一段时间；提供回退到旧 Handler 的 flag |
| 合规字段缺失 | Post-MVP 字段未同步 | 文档/代码标注 TODO，等合规流程就绪后逐步引入 |

---

## 13. 后续扩展（Post-MVP）

- 拆分 `profile.preferences` 独立表，启用 `supabase_sub`、`account_status` 字段。
- Watch log pruner & 分区策略，降低历史数据膨胀。
- `profile.audit_trail` 表与操作审计事件。
- Redis/Cloud Memorystore 缓存层，跨实例共享收藏/观看状态。
- GraphQL / REST BFF 适配层（供 Web/App 使用）。

---

> **执行提醒**：遵循“先新增再删除”原则。任何阶段若需要回滚，可通过禁用新 Handler/Feature Flag + 恢复旧 schema/任务来回退。文档、迁移脚本、测试必须同步更新，确保 CI 通过后才允许提交。***
