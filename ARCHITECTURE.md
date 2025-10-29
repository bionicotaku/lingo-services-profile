# Profile Service Detailed Design (v0.1 · 2025-10-27)

> 本文是 Profile 微服务的工程实现说明，聚焦 MVP → 可演进阶段的设计。内容覆盖领域职责、数据主权、契约、事件、非功能需求与开发路线。请在阅读前确认已熟悉《1项目概述》《2对外API概述》《3后端微服务模块》《4MVC架构》《6语言范式与骨架》。

---

## 1. 使命与边界

- **核心使命**：维护用户侧画像、学习偏好与互动状态（收藏、点赞、观看历史）的权威真相，向上游（Gateway）提供个性化接口，向下游（Feed/Catalog/Progress/Report/Telemetry）提供可靠的用户态视图。
- **关键原则**
  - **单一事实表**：Profile 唯一负责 `profile.*` schema 的写入；其他服务通过公开端口获取/更新，不允许跨库写入。
  - **最小暴露**：对外接口按需裁剪字段，敏感信息默认不对外公开，仅在用户本人或受信内部服务请求时返回。
  - **事件驱动**：偏好、收藏、观看等变更必须写入 Outbox，供推荐/报表/触达链路消费。
  - **隐私优先**：遵循最小存储与可控保留策略，满足用户数据导出/删除要求；所有查询均受限于用户身份或服务身份。
  - **高可用读**：热点查询（收藏状态、偏好配置）提供缓存/读模型，确保 Catalog/Feed 页面毫秒级补数。

---

## 2. 领域模型

### 2.1 聚合根 `UserProfile`

| 层级 | 字段 | 说明 | 来源 |
| --- | --- | --- | --- |
| 基础信息 | `user_id`(UUID)、`supabase_sub`、`email`(可选)、`display_name`、`avatar_url`、`preferred_locale`、`preferred_timezone`、`account_status`(`active`/`suspended`/`deleted`)、`created_at`、`updated_at` | 用户基本属性与账户状态；`supabase_sub` 用于与 Supabase 同步 | Profile 同步 Supabase |
| 学习偏好 | `learning_goal`(enum)、`target_score`、`daily_quota_minutes`、`preferred_difficulty_band`、`interests[]`（主题标签）、`content_filters`（如区域/口音/字幕偏好） | 驱动 Feed/Progress 个性化 | 用户设置 via Gateway |
| 通知偏好 | `email_opt_in`、`push_opt_in`、`reminder_schedule`、`quiet_hours` | 控制运营通知与学习提醒 | 用户设置 & Support |
| 安全与合规 | `gdpr_consent_at`、`marketing_consent_at`、`last_export_at`、`pending_deletion_at` | 数据出入流程的合规字段 | Support/运营流程 |

- **不变量**：
  - `account_status=deleted` 时禁止返回任何非公开字段，并触发异步清理历史互动数据。
  - `preferred_timezone` 必须符合 IANA TZ 名称；若缺省则继承 Supabase 用户设置。
  - 偏好字段需具备版本号（`preferences_version`），便于幂等与冲突检测。
- **领域行为**：
  - `UpdateProfileInfo`：更新显示名、头像、语言等，同时递增 `profile_version`。
  - `UpdatePreferences`：局部更新学习/通知偏好，记录差异并写入 Outbox。
  - `ScheduleDeletion` / `CancelDeletion`：管理数据删除申请，并触发 Support 工作流。

### 2.2 聚合 `EngagementSet`

| 维度 | 字段 | 说明 | 来源 |
| --- | --- | --- | --- |
| 收藏 | `favorite_id`(ULID)、`user_id`、`video_id`、`created_at`、`source`(manual/recommendation/system) | 点赞/收藏记录；`source` 用于推荐策略回溯。视频元数据通过 `profile.videos_projection` 补水，不直接存放在此表。 | Gateway → Profile |
| 点赞 | `like_state`（bool）、`last_liked_at` | 是否点赞与时间；与收藏复用同张表（`favorite_type`）或独立列 | Gateway |
| 观看历史 | `watch_id`(ULID)、`video_id`、`position_seconds`、`progress_ratio`、`session_id`、`last_watched_at`、`device` | 记录最近观看进度；用于继续播放、冷启动推荐；同样依赖 `profile.videos_projection` 承担详情补水。 | Telemetry/客户端回调 |
| compliance | `expires_at`（可空）、`redacted_at` | Watch log 的保留与清理状态 | 数据保留策略 |

- **不变量**：
  - 同一 `user_id + video_id` 只允许存在一条收藏记录（ON CONFLICT UPSERT）；删除操作使用软删除字段 `deleted_at`。
  - Watch log 仅保留最近 `N` 条（默认 200），超出部分通过后台任务归档至冷存储或删除。
- **行为**：
  - `ToggleFavorite`：幂等切换收藏/点赞状态，返回最新状态。
  - `RecordWatchProgress`：更新或插入观看记录；若进度 <5% 则视为未观看，自动清除记录。
  - `PruneWatchHistory`：后台任务定期清理冗余记录。

### 2.3 值对象与策略

- `PreferenceDelta`：记录偏好变更字段、旧值/新值，随事件一起发布。
- `FavoriteSummary`：聚合收藏数量、最近一次收藏时间，用于 Feed 个性化。
- `WatchCursor`：游标分页结构，包含 `last_watched_at` 与 `watch_id`。

---

## 3. 数据模型（Postgres `profile` schema）

### 3.1 表结构

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
#### `profile.users`
- `user_id` (uuid, PK)：主键，同步于 Supabase 用户 ID（当前阶段直接复用 Supabase `sub`）。
- `supabase_sub` (text, UNIQUE, post-MVP)：可选的外部身份映射字段，MVP 暂不创建，留待未来需要多身份源或改动主键时再引入。
- `display_name` (text)：用户展示昵称。
- `avatar_url` (text)：头像地址，允许为空。
- `preferred_locale` (text, post-MVP)：首选语言，采用 BCP47/ISO 代码；MVP 仅记录在客户端，不落库。
- `preferred_timezone` (text, post-MVP)：首选时区，IANA TZ；MVP 阶段使用默认值。
- `account_status` (enum `active`/`suspended`/`deleted`, post-MVP)：账户状态管理，MVP 先默认 `active`，待接入风控后再引入。
- `profile_version` (int)：档案乐观锁版本号，写入时需匹配。
- `preferences_json` (jsonb)：MVP 仅包含 `learning_goal`、`daily_quota_minutes` 两个字段；其余偏好（难度带、兴趣、过滤、通知设置等）标记为 Post-MVP 扩展。
- `created_at` / `updated_at` (timestamptz)：创建与最近更新时间。
- `deleted_at` (timestamptz, nullable, post-MVP)：账号删除标记；MVP 阶段暂不存储，待合规流程上线后再启用。

> Post-MVP 计划拆分出 `profile.preferences` 独立表；若那时需要 `supabase_sub` 等外部身份字段，再一起引入。

约束与索引：`PRIMARY KEY (user_id)`；当启用 `supabase_sub` 后补充 `UNIQUE (supabase_sub)`；常用查询对 `account_status`、`user_id` 增加复合索引（post-MVP）；开启 RLS。

迁移 SQL（幂等）：
```sql
create schema if not exists profile;

create table if not exists profile.users (
  user_id         uuid primary key,                                  -- 用户主键，复用 Supabase sub
  display_name    text not null,                                     -- 展示昵称
  avatar_url      text,                                              -- 头像 URL
  profile_version bigint not null default 1,                         -- 乐观锁版本号
  preferences_json jsonb not null default jsonb_build_object(
    'learning_goal', null,
    'daily_quota_minutes', null
  ),                                                                -- 偏好 JSON（MVP 仅含 learning_goal、daily_quota_minutes）
  created_at      timestamptz not null default now(),                -- 创建时间
  updated_at      timestamptz not null default now()                 -- 最近更新时间
);

comment on table profile.users is 'Profile 档案主表，MVP 合并偏好字段';
comment on column profile.users.user_id is '用户主键，复用 Supabase sub';
comment on column profile.users.display_name is '展示昵称';
comment on column profile.users.avatar_url is '头像 URL';
comment on column profile.users.profile_version is '乐观锁版本号';
comment on column profile.users.preferences_json is '学习/通知偏好 JSON：MVP 仅包含 learning_goal、daily_quota_minutes';
comment on column profile.users.created_at is '记录创建时间';
comment on column profile.users.updated_at is '最近更新时间（触发器维护）';

create or replace function profile.tg_set_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at := now();
  return new;
end;
$$;

do $$
begin
  if not exists (
    select 1 from pg_trigger
    where tgname = 'set_updated_at_on_profile_users'
  ) then
    create trigger set_updated_at_on_profile_users
      before update on profile.users
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;
```

#### `profile.engagements`
- `user_id` (uuid, PK part)：互动所属用户。
- `video_id` (uuid/ulid, PK part)：目标视频。
- `engagement_type` (enum `like`/`bookmark`/... , PK part)：互动类别，可扩展。
- `engagement_id` (ulid, post-MVP)：预留单主键，便于未来支持多条记录、外键引用与事件对账。MVP 阶段主键采用 `(user_id, video_id, engagement_type)`，保持表结构简单。
- `source` (enum `manual`/`recommendation`/`system`, post-MVP)：互动来源，MVP 阶段默认视为 `manual`，待行为分析需求明确后再引入枚举字段。
- `metadata` (jsonb, post-MVP)：可选上下文（终端、入口等），MVP 暂不存储，待需要更多分析维度时再启用。
- `created_at` / `updated_at` (timestamptz)：创建与最近更新时间。
- `deleted_at` (timestamptz, nullable)：软删除标记，表示互动被撤销。

约束与索引：复合主键 `(user_id, video_id, engagement_type)` 确保幂等；热点读取使用覆盖索引 `ON (video_id, user_id)` 及 `PARTIAL INDEX WHERE deleted_at IS NULL`。后续若启用单主键 `engagement_id`，需改为 `PRIMARY KEY (engagement_id)` 并保留 `UNIQUE (user_id, video_id, engagement_type)`。

迁移 SQL（幂等）：
```sql
create table if not exists profile.engagements (
  user_id          uuid not null,                                    -- 互动所属用户 ID
  video_id         uuid not null,                                    -- 互动目标视频 ID
  engagement_type  text not null,                                    -- 互动类型（like/bookmark）
  created_at       timestamptz not null default now(),               -- 创建时间
  updated_at       timestamptz not null default now(),               -- 最近更新时间
  deleted_at       timestamptz,                                      -- 软删除标记
  primary key (user_id, video_id, engagement_type),
  check (engagement_type in ('like', 'bookmark'))
);

comment on table profile.engagements is '用户对视频的互动记录（点赞、收藏等）';
comment on column profile.engagements.user_id is '互动所属用户 ID';
comment on column profile.engagements.video_id is '互动目标视频 ID';
comment on column profile.engagements.engagement_type is '互动类型：MVP 支持 like/bookmark';
comment on column profile.engagements.created_at is '创建时间';
comment on column profile.engagements.updated_at is '最近更新时间';
comment on column profile.engagements.deleted_at is '软删除标记，表示互动被撤销';

create index if not exists profile_engagements_video_idx
  on profile.engagements (video_id, user_id)
  where deleted_at is null;

do $$
begin
  if not exists (
    select 1 from pg_trigger
    where tgname = 'set_updated_at_on_profile_engagements'
  ) then
    create trigger set_updated_at_on_profile_engagements
      before update on profile.engagements
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;
```

#### `profile.watch_logs`
- `user_id` (uuid, PK part)：所属用户。
- `video_id` (uuid/ulid, PK part)：观看视频。
- `watch_id` (ulid, post-MVP)：保留为将来支持多条观看记录、外键引用或跨系统对账的扩展主键。MVP 阶段不创建，仅使用 `(user_id, video_id)` 作为复合主键。
- `session_id` (text, post-MVP)：播放器/Telemetry 生成的播放会话 ID，用于跨系统串联同一播放过程。MVP 阶段暂不写入，待 Telemetry 管道完善后再启用，可辅助去重与追踪。 
- `position_seconds` (numeric)：最近播放位置（秒）。
- `progress_ratio` (numeric)：观看进度 0~1。
- `total_watch_seconds` (numeric)：累计观看时长（秒），会在每次上报时累加，用于活跃度与学习时长统计。
- `device_info` (jsonb, post-MVP)：终端/客户端信息，MVP 暂不记录。
- `first_watched_at` (timestamptz)：首次观看时间。
- `last_watched_at` (timestamptz)：最近一次观看时间，用于排序分页。
- `expires_at` (timestamptz, nullable)：记录过期时间；MVP 阶段直接设为 `last_watched_at + retention_days`（配置常量），便于后台按 TTL 清理。
- `redacted_at` (timestamptz, nullable, post-MVP)：合规删除标记；MVP 阶段暂不使用，待推出自动化隐私删除流程后再启用。
- `created_at` (timestamptz)：记录写入时间。

索引：`INDEX (user_id, last_watched_at DESC)` 支撑倒序分页；针对 `redacted_at IS NULL` 的部分索引用于有效数据查询；`INDEX (expires_at)` 支撑过期扫描。复合主键 `(user_id, video_id)` 保证幂等，后续若引入 `watch_id` 再调整为单主键并补唯一约束。

迁移 SQL（幂等）：
```sql
create table if not exists profile.watch_logs (
  user_id             uuid not null,                                 -- 用户 ID
  video_id            uuid not null,                                 -- 视频 ID
  position_seconds    numeric not null,                              -- 最近播放位置（秒）
  progress_ratio      numeric not null,                              -- 观看进度（0~1）
  total_watch_seconds numeric not null default 0,                    -- 累计观看时长（秒）
  first_watched_at    timestamptz not null default now(),            -- 首次观看时间
  last_watched_at     timestamptz not null default now(),            -- 最近观看时间
  expires_at          timestamptz,                                   -- TTL 到期时间
  redacted_at         timestamptz,                                   -- 合规脱敏标记
  created_at          timestamptz not null default now(),            -- 记录创建时间
  primary key (user_id, video_id)
);

comment on table profile.watch_logs is '用户观看进度日志（MVP 复合主键 user_id+video_id）';
comment on column profile.watch_logs.position_seconds is '最近播放位置（秒）';
comment on column profile.watch_logs.progress_ratio is '观看进度（0~1）';
comment on column profile.watch_logs.total_watch_seconds is '累计观看时长（秒）';
comment on column profile.watch_logs.first_watched_at is '首次观看时间';
comment on column profile.watch_logs.last_watched_at is '最近观看时间';
comment on column profile.watch_logs.expires_at is '保留截止时间（用于 TTL 清理）';
comment on column profile.watch_logs.redacted_at is '合规脱敏标记（post-MVP）';
comment on column profile.watch_logs.created_at is '记录创建时间';

create index if not exists profile_watch_logs_user_last_idx
  on profile.watch_logs (user_id, last_watched_at desc);

create index if not exists profile_watch_logs_expires_idx
  on profile.watch_logs (expires_at);

do $$
begin
  if not exists (
    select 1 from pg_trigger
    where tgname = 'set_updated_at_on_profile_watch_logs'
  ) then
    create trigger set_updated_at_on_profile_watch_logs
      before update on profile.watch_logs
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;
```

#### `profile.videos_projection`
- `video_id` (uuid/ulid, PK)：视频主键。
- `title` (text)：标题。
- `description` (text)：简介，供收藏/历史列表展示。
- `duration_micros` (bigint)：视频时长（微秒）。
- `thumbnail_url` (text)：封面 URL。
- `hls_master_playlist` (text)：播放清单 URL（来自 Catalog `videos.hls_master_playlist`）。
- `status` (enum)：Catalog 生命周期状态（`pending`/`ready`/`published`...）。
- `visibility_status` (enum `public`/`unlisted`/`private`, post-MVP)：可见性状态；Catalog 当前字段仍在预留阶段，后续上线后补入。
- `published_at` (timestamptz, nullable, post-MVP)：发布时间，待 Safety/运营流程启用时回填。
- `version` (bigint)：同步自 Catalog `videos.version`，用于幂等/增量更新。若对外需要 ETag，可在响应层基于 `version` 生成。
- `updated_at` (timestamptz)：最近同步时间。

索引：`PRIMARY KEY (video_id)`；必要时在 `updated_at` 上建立索引用于陈旧检测。

迁移 SQL（幂等）：
```sql
create table if not exists profile.videos_projection (
  video_id            uuid primary key,                              -- 视频主键
  title               text not null,                                 -- 标题
  description         text,                                          -- 简介
  duration_micros     bigint,                                        -- 时长（微秒）
  thumbnail_url       text,                                          -- 封面 URL
  hls_master_playlist text,                                          -- 播放清单 URL
  status              text,                                          -- Catalog 生命周期状态
  visibility_status   text,                                          -- 可见性状态
  published_at        timestamptz,                                   -- 发布时间
  version             bigint not null,                               -- Catalog 版本号
  updated_at          timestamptz not null default now()             -- 投影同步时间
);

comment on table profile.videos_projection is 'Catalog 视频元数据投影（Profile 消费 catalog.video.* 事件）';
comment on column profile.videos_projection.video_id is '视频主键';
comment on column profile.videos_projection.title is '视频标题';
comment on column profile.videos_projection.description is '视频简介';
comment on column profile.videos_projection.duration_micros is '视频时长（微秒）';
comment on column profile.videos_projection.thumbnail_url is '封面 URL';
comment on column profile.videos_projection.hls_master_playlist is '播放清单 URL（来自 Catalog）';
comment on column profile.videos_projection.status is 'Catalog 生命周期状态';
comment on column profile.videos_projection.visibility_status is '可见性状态';
comment on column profile.videos_projection.published_at is '发布时间';
comment on column profile.videos_projection.version is 'Catalog 版本号（乐观锁）';
comment on column profile.videos_projection.updated_at is '同步更新时间';

create index if not exists profile_videos_projection_updated_idx
  on profile.videos_projection (updated_at);
```

#### `profile.video_stats`
- `video_id` (uuid/ulid, PK)：目标视频。
- `like_count` (bigint)：点赞总数（`engagement_type=like` 且未软删）。
- `favorite_count` (bigint)：收藏总数（`engagement_type=bookmark` 且未软删）。
- `unique_watchers` (bigint)：累计观看人数，依据 `profile.watch_logs` 中首次观看记录（按 `user_id` 去重）计算。
- `total_watch_seconds` (bigint)：累计观看时长，来源于 watch log 聚合。
- `updated_at` (timestamptz)：最近刷新时间。

> **维护方式**：MVP 由 Profile Service 在 `MutateFavorite`/`UpsertWatchProgress` 成功后同步更新（使用 `INSERT ... ON CONFLICT` 累加），确保请求链路即可返回全局统计信息。后续可替换为批处理或事件驱动方案。

迁移 SQL（幂等）：
```sql
create table if not exists profile.video_stats (
  video_id            uuid primary key,                              -- 视频主键
  like_count          bigint not null default 0,                     -- 点赞总数
  favorite_count      bigint not null default 0,                     -- 收藏总数
  unique_watchers     bigint not null default 0,                     -- 独立观看用户数
  total_watch_seconds bigint not null default 0,                     -- 累计观看时长（秒）
  updated_at          timestamptz not null default now()             -- 统计更新时间
);

comment on table profile.video_stats is '视频全局互动/观看统计（MVP 由 Profile 同步维护）';
comment on column profile.video_stats.video_id is '视频主键';
comment on column profile.video_stats.like_count is '点赞总数';
comment on column profile.video_stats.favorite_count is '收藏总数';
comment on column profile.video_stats.unique_watchers is '独立观看用户数';
comment on column profile.video_stats.total_watch_seconds is '累计观看时长（秒）';
comment on column profile.video_stats.updated_at is '统计更新时间';

do $$
begin
  if not exists (
    select 1 from pg_trigger
    where tgname = 'set_updated_at_on_profile_video_stats'
  ) then
    create trigger set_updated_at_on_profile_video_stats
      before update on profile.video_stats
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;
```

#### `profile.inbox_events`
- 完全复用模板 `inbox_events` 结构，记录已消费的 Catalog 事件，保证幂等：
  - `event_id` (uuid, PK)
  - `source_service` (text)
  - `event_type` (text)
  - `aggregate_type` (text)
  - `aggregate_id` (text)
  - `payload` (bytea)
  - `received_at` / `processed_at` (timestamptz)
  - `last_error` (text, nullable)

- #### `profile.subscription_offsets`（post-MVP）
- 复用标准 offset 存储；MVP 阶段可先依赖 Pub/Sub 默认位点，后续需要精确控制与回放能力时再引入：
  - `consumer_group` (text, PK)
  - `subscription` (text)
  - `message_id` (text)
  - `publish_time` (timestamptz)
  - `updated_at` (timestamptz 默认 now())
  - `updated_by` (text)
  - `lag_millis` (bigint)

#### `profile.outbox_events`
- 完全复用模板 Outbox 表结构：`event_id`, `aggregate_type`, `aggregate_id`, `event_type`, `payload`, `headers`, `occurred_at`, `available_at`, `published_at`, `delivery_attempts`, `last_error`, `lock_token`, `locked_at`。

> **Outbox / Inbox 工作流**：写路径复用模板 Outbox；当点赞/收藏/观看统计更新时分别写入 `profile.engagement.added/removed`、`profile.watch.progressed` 事件。`profile.watch.progressed` 仅在首次观看或进度变化 ≥5% 时发出，避免播放心跳导致事件风暴；若后续吞吐增长，再考虑批量/定时聚合。读路径通过 StreamingPull 订阅 `catalog.video.events`。每次消费事件：
> 1. 事务内 `INSERT` 至 `profile.inbox_events`（以 `event_id` 幂等）。
> 2. 对比事件 `version` 与 `profile.videos_projection.version`，若更大则 `UPSERT`。
> 3. 同事务可选更新 `profile.subscription_offsets`（post-MVP，记录 message_id/publish_time/lag）。
> 4. 事务提交成功后 Ack，保证“成功处理 ⇒ 位点推进”。


#### `profile.audit_trail`（post-MVP）
- 计划在后续版本启用，结构参考模板：`event_id`, `user_id`, `actor`, `event_type`, `payload`, `created_at`，用于长期审计。

**索引与权限补充**：
- `profile.engagements`：建议建立覆盖索引 `ON (video_id, user_id)` 及 `PARTIAL INDEX WHERE deleted_at IS NULL`。
- `profile.watch_logs`：`INDEX (user_id, last_watched_at DESC)` + `INDEX (expires_at)`；有效数据查询配合 `WHERE redacted_at IS NULL` 部分索引。
- 所有核心表启用 Supabase RLS（规则 `user_id = auth.uid()`），服务角色持有跨用户访问权。

### 3.2 读模型（可选增强）

- `profile.user_state_view`（物化视图/缓存表）：字段 `user_id`、`total_favorites`、`recent_favorite_video_ids`(int[])、`last_watch_video_id`、`last_watch_progress`；由后台任务或 SQL 刷新，用于 Feed 冷启动。
- `profile.engagement_counts_by_video`：供 Catalog/Feed 查询特定视频下当前用户是否收藏 + 全局互动数；与 Telemetry 汇总区分。
- `profile.videos_projection`：Inbox 从 Catalog 事件同步，确保 Profile 可以在不跨服务查询的情况下补充收藏/历史响应的基础元数据；每次更新对比 Catalog `version`，并将最新版本写入投影，便于条件请求或缓存控制。

---

## 4. 服务结构与仓库布局

```
services-profile/
├── cmd/grpc/                 # Kratos gRPC 入口
├── cmd/http/                 # 调试 HTTP（可选）
├── configs/                  # 配置（YAML + .env）
├── internal/
│   ├── controllers/http      # REST Handler（Problem Details、ETag、Idempotency）
│   ├── controllers/grpc      # gRPC Server（ProfileService）
│   ├── services/             # 用例：Profile、Preference、Favorite、WatchHistory
│   ├── repositories/         # PG 访问（sqlc）+ Outbox/Inbox 状态存取
│   ├── models/{po,vo}        # 数据库存储对象、视图对象
│   ├── views                 # Problem、分页、ETag 封装
│   ├── clients/              # 调用 Telemetry/Support（数据删除）、Catalog（批量查询）
│   ├── infrastructure/       # configloader、pgxpool、idempotency store、cache
│   └── tasks/                # Outbox publisher、Catalog Inbox 同步、watch log pruning、read model刷新
├── api/openapi/              # REST 契约（Spectral）
├── api/proto/                # gRPC 契约（buf）
├── migrations/               # Supabase schema 迁移
├── sqlc/                     # sqlc 生成产物
└── test/                     # Service/Repository/任务测试
```

- **缓存策略**：`services/internal/infrastructure/cache` 提供基于本地 LRU / future Redis 的可插拔缓存；收藏状态缓存 TTL ≤ 60s，写入后立即失效。
- **Idempotency**：写接口复用模板中的 `pkg/idempotency`，键格式 `profile:<user_id>:<action>:<resource_id>`。

---

## 5. API 契约

### 5.1 gRPC `profile.v1.ProfileService`

| 方法 | 用途 | 备注 |
| --- | --- | --- |
| `GetProfile(GetProfileRequest) returns (GetProfileResponse)` | 返回用户档案与偏好；支持 `If-None-Match`（ETag 基于 `profile_version` & `preferences_version`） | 只允许本人或服务身份；匿名调用返回 401 |
| `UpdateProfile(UpdateProfileRequest) returns (UpdateProfileResponse)` | 更新基础信息与通知偏好；要求 `Idempotency-Key` 与 `expected_profile_version` | 幂等：重复请求返回最新版本 |
| `UpdatePreferences(UpdatePreferencesRequest)` | 局部更新学习偏好；`fields_mask` 控制更新字段（事件推送留待后续） | 超时 500ms |
| `GetFavorites(GetFavoritesRequest)` | 游标分页返回收藏视频 ID 列表 | 支持 `page_size`、`cursor` |
| `MutateFavorite(MutateFavoriteRequest)` | 新增/取消收藏或点赞；操作类型 `ADD`/`REMOVE`; 支持 `favorite_type` | 响应包含 `favorite_state`，并返回最新 `like_count`/`favorite_count`（来自 `profile.video_stats`） |
| `BatchQueryFavorite(BatchQueryFavoriteRequest)` | 批量获取给定 video_id 对应的收藏/点赞布尔值及统计 | Catalog 在详情页补数使用；返回字段含 `has_liked`、`has_bookmarked`、`like_count`、`favorite_count`、`unique_watchers` |
| `UpsertWatchProgress(UpsertWatchProgressRequest)` | 写入观看进度；带 `session_id` 与播放位置 | 由 Telemetry 或客户端调用 |
| `ListWatchHistory(ListWatchHistoryRequest)` | 分页返回最近观看列表 | `cursor` 基于 `last_watched_at`；每项含视频全局统计（调用 `profile.video_stats`） |
| `PurgeUserData(PurgeUserDataRequest)` | Support 数据删除流程调用；触发异步清理并返回任务 ID | 受限于服务角色 |

### 5.2 REST 映射（Gateway 暴露 `/api/v1`）

| REST | 说明 | gRPC 映射 | 特殊要求 |
| --- | --- | --- | --- |
| `GET /api/v1/user/me` | 返回本人档案与偏好 | `GetProfile` | MVP 先返回最新数据，`ETag`/`If-None-Match` 留待后续版本 |
| `PATCH /api/v1/user/me` | 更新档案/偏好 | `UpdateProfile` + `UpdatePreferences` | MVP 阶段仅做幂等性说明，不强制 `Idempotency-Key`；正式支持放入 post-MVP |
| `GET /api/v1/user/me/favorites` | 分页获取收藏列表 | `GetFavorites` | 通过 `profile.videos_projection` 补全视频摘要，MVP 需同步维护该投影 |
| `POST /api/v1/video/{id}/like` | 点赞（favorite_type=like） | `MutateFavorite` (`ADD`) | 幂等；返回 Problem 429 on rate limit |
| `DELETE /api/v1/video/{id}/like` | 取消点赞 | `MutateFavorite` (`REMOVE`) | 重复删除返回 204 |
| `POST /api/v1/video/{id}/favorite` | 收藏（favorite_type=bookmark） | 同上 | 同上 |
| `DELETE /api/v1/video/{id}/favorite` | 取消收藏 | 同上 | 同上 |
| `GET /api/v1/user/me/watch-history` | 观看历史 | `ListWatchHistory` | 支持 `cursor`；默认 20 条；视频元数据同样来自 `profile.videos_projection` |

- **限流与配额**：点赞/收藏接口限制 `10 req/s`（滑动窗口）与 `每日 5k`；偏好更新限制 `100 req/day`。
- **错误语义**：统一 Problem 类型（例：`profile.errors.preference_conflict`、`profile.errors.favorite_limit_reached`）。

---

## 6. 领域事件与 Outbox

- **事件流摘要（MVP）**
  - Profile 发布：`profile.engagement.added`、`profile.engagement.removed`、`profile.watch.progressed`（通过 Outbox 实时推送）。
  - Profile 订阅：`catalog.video.*`（通过 Inbox / `profile.videos_projection` 同步视频元数据）。

  **实施方式（与 kratos-template 保持一致）**
  1. **领域事件构造 + Outbox 写入**：Service 在同一事务内完成业务表写入后，沿用模板 `internal/services/video_command_service.go` 的模式，使用 `internal/models/outbox_events` 生成 protobuf 事件，调用 `repositories.OutboxRepository.Enqueue` 写入 `profile.outbox_events`。
  2. **Outbox Publisher**：复用 `internal/tasks/outbox` 的 Runner，在 `cmd/grpc` 注册为后台 worker。Runner 按 `messaging.outbox` 配置扫描 Outbox，发布至 Pub/Sub Topic，成功 `MarkPublished`，失败 `Reschedule`。
  3. **Inbox + 投影消费**：复制模板 `internal/tasks/projection`，订阅 `catalog.video.events`。Runner 先把事件写入 `profile.inbox_events`（幂等），再 `UPSERT` `profile.videos_projection`，最后在同一事务内标记处理成功并 Ack。
  4. **配置**：Profile 的 `config.yaml` 保留模板 `messaging` 节点，修改 schema（`profile`）、Topic/Subscription 名称等，Wire 通过 `ProvideOutboxConfig` 与 `ProvidePubSubConfig` 注入依赖。
  5. **迁移脚本**：基于模板 `migrations/002_create_catalog_event_tables.sql` 复制一份，将 schema 改为 `profile`，即可得到标准 `outbox_events`、`inbox_events` 表结构及索引。
  6. **测试/运维**：沿用模板 `test/full_e2e_projection.sh` 与 Outbox/Inbox 集成测试，结合 `outbox_backlog`、`inbox_lag` 等指标监控链路健康。

| 事件名 | 触发条件 | 关键字段 | 消费方 |
| --- | --- | --- | --- |
| `profile.engagement.added` | 收藏/点赞等互动新增 | `user_id`, `video_id`, `engagement_type`, `created_at`, `source` | Feed（推荐权重）、Catalog（异步写 user_state_view）、Telemetry（行为对账） |
| `profile.engagement.removed` | 收藏/点赞等互动删除 | 同上 + `deleted_at` | 同上 |
| `profile.watch.progressed` | 观看记录更新（进度变化 ≥5% 或状态从无到有） | `user_id`, `video_id`, `progress_ratio`, `position_seconds`, `last_watched_at`, `session_id` | Feed（继续看推荐）、Report（活跃度统计）；MVP 仅在进度首次记录或变更 ≥5% 时发出，避免播放心跳产生过量事件 |
| `profile.user.deletion.scheduled` | 用户提交删除申请 | `user_id`, `scheduled_at`, `delete_after` | Support（协调删除）、Telemetry（停止继续采集） |
| `profile.user.deletion.completed` | 清理任务完成 | `user_id`, `completed_at` | Support、Gateway（登出） |

- Outbox 模式：与业务事务共享 tx；`tasks/outbox_publisher` 每 100ms 扫描，失败重试使用指数退避（最多 5 次）。
- Inbox 模式：`catalog.video.*` 事件通过 `tasks/catalog_inbox_consumer` 消费，先写 `profile.inbox_events` 去重，再 `UPSERT` `profile.videos_projection`。偏移记录（`profile.subscription_offsets`）留待 post-MVP 引入。
- 事件 payload 遵循 JSON Schema（放置于 `api/events`，供 Spectral 校验）。

---

## 7. 集成契约

### 7.1 Gateway ↔ Profile

- Gateway 负责 Supabase JWT 验签并透传 `user_id`；Profile 进行资源级授权。
- Gateway 在 PATCH 档案时拆分请求：基础信息 → `UpdateProfile`，偏好字段 → `UpdatePreferences`。
- 统一 Problem 映射（`profile.errors.*` → HTTP 状态码）。

### 7.2 Catalog ↔ Profile

- Catalog 在详情页调用 `BatchQueryFavorite` 获取用户对当前视频的点赞/收藏状态；超时 200ms。
- Catalog 可订阅 `profile.engagement.*` 事件异步刷新自己的 `video_user_states` 投影。
- Profile 通过 Inbox 消费 `catalog.video.*` 事件维护 `profile.videos_projection` 表，使“收藏/观看历史列表”无需跨服务查询即可附带基础视频元数据。

### 7.3 Feed ↔ Profile

- Feed 消费 `profile.engagement.*` 调整推荐权重。
- Feed 首次为用户生成推荐时，调用 `GetProfile` 获取偏好（带 500ms 超时，允许降级）。
- Profile 提供 `views/recommendation_seed.go` 将偏好转换为 feed 输入结构。
- Feed 可选订阅 `profile.watch.progressed`（按需），用于“继续观看”列表刷新。

### 7.4 Progress ↔ Profile

- Progress 使用 `preferred_difficulty_band`、`daily_quota_minutes`、`reminder_schedule` 来调整 FSRS 队列。
- Progress 与 Profile 之间仍通过 gRPC 获取偏好；偏好更新事件留待后续引入。

### 7.5 Telemetry ↔ Profile

- Telemetry 可直接调用 `UpsertWatchProgress` 或将观看事件写入队列，由 Profile 背景任务消费。
- Watch log 的 session_id 与 Telemetry 事件保持一致，便于追踪。

### 7.6 Support / Compliance ↔ Profile

- Support 触发 `PurgeUserData`，Profile 负责软删除档案并发布 `profile.user.deletion.*`。
- Profile 需暴露 `ExportUserSnapshot`（后续扩展）供数据导出流程使用。

### 7.7 Analytics / Report ↔ Profile

- Report 消费 `profile.engagement.*` 与 `profile.watch.progressed` 填充报表。MVP 阶段 Profile 内部维护 `profile.video_stats` 提供点赞/收藏/观看计数，后续可将聚合职责迁移至 Report。
- Profile 订阅 `catalog.video.*` 事件（通过 `profile.inbox_events` + `videos_projection`）保持视频元数据与 Catalog 同步。
- Profile 提供只读视图（或 gRPC `ListUserActivity`）供运营后台查看个体行为。

---

## 8. 非功能需求

| 项目 | 要求 |
| --- | --- |
| 可用性 | 99.9%；关键读接口（`GetProfile`, `BatchQueryFavorite`）需部署多实例，内建健康检查。 |
| 延迟 | 读接口 P95 < 120ms；写接口 P95 < 200ms；收藏写入完成后 500ms 内事件可达。 |
| 隐私 | 所有表启用 RLS；PII（email）仅在服务级调用、响应中默认省略；支持 GDPR 删除/导出。 |
| 审计 | Post-MVP 引入 `profile.audit_trail` 记录写操作（含 `actor`/`trace_id`）；MVP 阶段可用结构化日志代替。 |
| 缓存 | Favorite 状态使用本地 LRU；跨实例后可切换 Redis。缓存命中率目标 ≥ 85%。 |
| 指标 | 暴露 `profile_engagement_total`, `profile_watch_progress_total`, `profile_preferences_update_total`, `profile_outbox_lag_seconds`, `profile_inbox_lag_seconds`, `profile_cache_hit_ratio`。 |
| 日志 | `log/slog` JSON；字段 `user_id`, `video_id`, `action`, `trace_id`, `source`; 对 PII 脱敏。 |
| 超时 | 外部调用默认 500ms；数据库查询 200ms；UpsertWatchProgress 允许 800ms（批量）。 |
| 重试 | Outbox 发布 5 次；写接口客户端重试建议 3 次带指数退避。 |
| 数据保留 | Watch log 默认保留 180 天，可配置；收藏与偏好长期保留，删除用户时清理。 |

---

## 9. 开发路线图

1. **契约起草**：定义 `api/proto/profile/v1/profile.proto` 与 REST OpenAPI，运行 `buf lint`、`buf breaking`、`spectral lint`。
2. **领域建模**：实现 `internal/domain/profile`、`internal/domain/engagement`，编写状态/幂等单测（覆盖率 ≥ 90%）。
3. **仓储实现**：使用 `sqlc` 生成 DAO；实现 `ProfileRepository`、`FavoriteRepository`、`WatchLogRepository`，并提供事务接口。
4. **Service 层**：实现 `ProfileService`, `PreferenceService`, `FavoriteService`, `WatchHistoryService`，使用 gomock mock 仓储做单测（覆盖率 ≥ 80%）。
5. **Controller 层**：实现 HTTP/gRPC handler，集成 Problem Details、Idempotency、ETag。
6. **Outbox/Inbox 与任务**：实现 `outbox_publisher`、`catalog_inbox_consumer`（维护 `videos_projection`，subscription offset 记录延后至 post-MVP）；`watch_log_pruner` 标记为 post-MVP，可先手动清理。编写集成测试（Testcontainers）。
7. **缓存层**：实现收藏状态缓存与失效策略，提供接口给 Controller 注入。
8. **观测性**：配置 OTel exporter、Prometheus 指标、结构化日志。
9. **集成测试**：覆盖流程“注册档案 → 更新偏好 → 收藏视频 → 观看进度 → 删除收藏”；使用 Supabase Dev DB。
10. **文档与示例**：更新 `README.md`，提供 `grpcurl`、`curl` 示例；说明数据导出/删除流程。

---

## 10. 风险与缓解

| 风险 | 描述 | 缓解措施 |
| --- | --- | --- |
| 缓存一致性 | 本地缓存导致收藏状态短暂不一致 | 写操作后主动失效缓存；设置短 TTL；提供批量查询保证最终一致。 |
| 观看日志膨胀 | 高频事件导致表快速增长 | 设置 `expires_at` + 后台裁剪；可选将冷数据导出至冷存储。 |
| 偏好冲突 | 客户端多端并发修改偏好 | 使用 `preferences_version` 乐观锁；冲突返回 Problem `profile.errors.preference_conflict`。 |
| 隐私违规 | 未授权服务读取用户数据 | 强制服务身份认证 + RLS；审计日志定期巡检。 |
| Outbox 堵塞 | 大量事件导致延迟 | 增加并行发布 worker；监控 `profile_outbox_lag_seconds`；必要时分 topic。 |
| 批量查询压力 | Catalog 批量查询点赞导致热点 | 支持批量接口 + 限制最大请求数（默认 100）；对热点用户启用缓存。 |
| 投影滞后 | Catalog 事件延迟导致收藏/历史返回旧元数据 | Inbox 消费提供重试与监控 `profile_inbox_lag_seconds`，严重时回退到实时 gRPC 查询（降级路径）。 |

---

## 11. 后续扩展

- **多端同步**：记录设备类型、播放模式，支持“继续播放”跨设备同步。
- **社交信号**：扩展 `favorite_type` 支持 `share`、`comment`; 与 Support 协同审核。
- **成就系统**：基于观看/收藏事件触发奖励，需新增事件 `profile.achievement.unlocked`。
- **多租户**：增加 `tenant_id` 字段，并在所有索引中包含；事件 payload 同步带租户信息。
- **边缘缓存**：对公开 `GetProfile`（匿名字段）构建 CDN 缓存，用于自定义主页。

---

## 12. 版本记录

- **v0.1（2025-10-27）**：首版草案，覆盖领域模型、数据结构、契约、事件、非功能与路线图。

> TODO：补充实际迁移脚本、sqlc 输出路径说明、事件 JSON Schema。
- `source` (enum `manual`/`recommendation`/`system`)：触发来源，便于分析策略。
- `metadata` (jsonb, post-MVP)：可选上下文（终端、入口等），MVP 暂不存储，待需要更多分析维度时再启用。
- `created_at` / `updated_at` (timestamptz)：创建与最近更新时间。
- `deleted_at` (timestamptz, nullable)：软删除标记，表示互动被撤销。
- （主键备注）MVP 使用 `(user_id, video_id, engagement_type)` 作为复合主键，同时保留唯一索引，后续若启用 `engagement_id` 会调整为单主键。
