-- ============================================
-- Profile Schema 初始化（基于 Profile ARCHITECTURE.md）
-- ============================================

create schema if not exists profile;

-- 通用触发器：更新 updated_at 字段
create or replace function profile.tg_set_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at := now();
  return new;
end;
$$;
comment on function profile.tg_set_updated_at() is '触发器：在 UPDATE 时自动写入 updated_at';

-- ============================================
-- 1) 档案主表：profile.users
-- ============================================
create table if not exists profile.users (
  user_id         uuid primary key,                                  -- 用户主键，复用 Supabase sub
  display_name    text not null,                                     -- 展示昵称
  avatar_url      text,                                              -- 头像 URL
  profile_version bigint not null default 1,                         -- 乐观锁版本号
  preferences_json jsonb not null default jsonb_build_object(
    'learning_goal', null,
    'daily_quota_minutes', null
  ),                                                                 -- 偏好 JSON（MVP 仅含 learning_goal、daily_quota_minutes）
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

do $$
begin
  if not exists (
    select 1 from pg_trigger where tgname = 'set_updated_at_on_profile_users'
  ) then
    create trigger set_updated_at_on_profile_users
      before update on profile.users
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;

-- ============================================
-- 2) 互动表：profile.engagements
-- ============================================
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
comment on index profile.profile_engagements_video_idx is '按视频查询互动用户，过滤已删除记录';

do $$
begin
  if not exists (
    select 1 from pg_trigger where tgname = 'set_updated_at_on_profile_engagements'
  ) then
    create trigger set_updated_at_on_profile_engagements
      before update on profile.engagements
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;

-- ============================================
-- 3) 观看日志：profile.watch_logs
-- ============================================
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
  updated_at          timestamptz not null default now(),            -- 最近更新时间
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
comment on column profile.watch_logs.updated_at is '最近更新时间';

create index if not exists profile_watch_logs_user_last_idx
  on profile.watch_logs (user_id, last_watched_at desc);
comment on index profile.profile_watch_logs_user_last_idx is '按用户查询观看历史，倒序分页';

create index if not exists profile_watch_logs_expired_idx
  on profile.watch_logs (expires_at)
  where expires_at is not null;
comment on index profile.profile_watch_logs_expired_idx is '根据 expires_at 扫描需要裁剪的记录';

do $$
begin
  if not exists (
    select 1 from pg_trigger where tgname = 'set_updated_at_on_profile_watch_logs'
  ) then
    create trigger set_updated_at_on_profile_watch_logs
      before update on profile.watch_logs
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;

-- ============================================
-- 4) 视频投影：profile.videos_projection
-- ============================================
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
comment on index profile.profile_videos_projection_updated_idx is '检测滞后投影的更新时间索引';

-- ============================================
-- 5) 视频统计：profile.video_stats
-- ============================================
create table if not exists profile.video_stats (
  video_id            uuid primary key,                              -- 视频主键
  like_count          bigint not null default 0,                     -- 点赞总数
  bookmark_count      bigint not null default 0,                     -- 收藏总数
  unique_watchers     bigint not null default 0,                     -- 独立观看用户数
  total_watch_seconds bigint not null default 0,                     -- 累计观看时长（秒）
  updated_at          timestamptz not null default now()             -- 统计更新时间
);

comment on table profile.video_stats is '视频全局互动/观看统计（MVP 由 Profile 同步维护）';
comment on column profile.video_stats.video_id is '视频主键';
comment on column profile.video_stats.like_count is '点赞总数';
comment on column profile.video_stats.bookmark_count is '收藏总数';
comment on column profile.video_stats.unique_watchers is '独立观看用户数';
comment on column profile.video_stats.total_watch_seconds is '累计观看时长（秒）';
comment on column profile.video_stats.updated_at is '统计更新时间';

do $$
begin
  if not exists (
    select 1 from pg_trigger where tgname = 'set_updated_at_on_profile_video_stats'
  ) then
    create trigger set_updated_at_on_profile_video_stats
      before update on profile.video_stats
      for each row execute function profile.tg_set_updated_at();
  end if;
end$$;

-- ============================================
-- 6) Outbox 表：profile.outbox_events
-- ============================================
create table if not exists profile.outbox_events (
  event_id            uuid primary key default gen_random_uuid(),  -- 事件唯一标识
  aggregate_type      text not null,                               -- 聚合根类型
  aggregate_id        uuid not null,                               -- 聚合根主键
  event_type          text not null,                               -- 领域事件名，如 profile.engagement.added
  payload             bytea not null,                              -- 事件负载
  headers             jsonb not null default '{}'::jsonb,          -- 追踪/幂等等头信息
  occurred_at         timestamptz not null default now(),          -- 事件产生时间
  available_at        timestamptz not null default now(),          -- 可发布时间（延迟投递时使用）
  published_at        timestamptz,                                 -- 发布成功时间
  delivery_attempts   integer not null default 0 check (delivery_attempts >= 0), -- 投递尝试次数
  last_error          text,                                        -- 最近一次失败原因
  lock_token          text,                                        -- 发布器租约标记
  locked_at           timestamptz                                  -- 租约获取时间
);

comment on table profile.outbox_events is 'Outbox 表：与业务事务同库写入，后台扫描发布到事件总线';
comment on column profile.outbox_events.aggregate_type    is '聚合根类型（profile 服务内实体）';
comment on column profile.outbox_events.aggregate_id      is '聚合根主键';
comment on column profile.outbox_events.event_type        is '事件名，使用过去式（如 profile.watch.progressed）';
comment on column profile.outbox_events.payload           is '事件负载（Protobuf 二进制），包含业务数据快照';
comment on column profile.outbox_events.headers           is '事件头部（JSON），用于 trace/idempotency 等';
comment on column profile.outbox_events.available_at      is '事件可被 Relay 选择的时间';
comment on column profile.outbox_events.published_at      is '事件成功发布的时间戳';
comment on column profile.outbox_events.delivery_attempts is 'Outbox Relay 重试次数';
comment on column profile.outbox_events.last_error        is '最近一次投递失败原因';
comment on column profile.outbox_events.lock_token        is '发布器租约标记';
comment on column profile.outbox_events.locked_at         is '租约获取时间';

create index if not exists profile_outbox_events_available_idx
  on profile.outbox_events (available_at)
  where published_at is null;
comment on index profile.profile_outbox_events_available_idx is '扫描未发布事件时按 available_at 排序';

create index if not exists profile_outbox_events_lock_idx
  on profile.outbox_events (lock_token)
  where lock_token is not null;
comment on index profile.profile_outbox_events_lock_idx is '租约查询，定位长时间锁定的事件';

create index if not exists profile_outbox_events_published_idx
  on profile.outbox_events (published_at);
comment on index profile.profile_outbox_events_published_idx is '按发布状态过滤或审计事件';

-- ============================================
-- 7) Inbox 表：profile.inbox_events
-- ============================================
create table if not exists profile.inbox_events (
  event_id         uuid primary key,                     -- 来源事件唯一标识
  source_service   text not null,                        -- 事件来源服务
  event_type       text not null,                        -- 事件名
  aggregate_type   text,                                 -- 来源聚合根类型
  aggregate_id     text,                                 -- 来源聚合根主键
  payload          bytea not null,                       -- 原始事件载荷快照
  received_at      timestamptz not null default now(),   -- 收到事件时间
  processed_at     timestamptz,                          -- 本服务处理完成时间
  last_error       text                                  -- 最近一次处理失败信息
);

comment on table profile.inbox_events is 'Inbox 表：记录已消费的外部事件，保障处理幂等性';
comment on column profile.inbox_events.event_id       is '来源事件的唯一标识，保证消费幂等';
comment on column profile.inbox_events.source_service is '事件来源服务上下文';
comment on column profile.inbox_events.aggregate_type is '来源聚合根类型';
comment on column profile.inbox_events.aggregate_id   is '来源聚合根标识';
comment on column profile.inbox_events.processed_at   is '事件处理成功的时间戳';

create index if not exists profile_inbox_events_processed_idx
  on profile.inbox_events (processed_at);
comment on index profile.profile_inbox_events_processed_idx is '按处理状态/时间筛选 Inbox 记录';
