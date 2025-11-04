# Profile Engagement 事件：Pub/Sub 配置指引

> 目标：`services-profile` 在用户互动（点赞/收藏/观看进度）变更时，通过 Outbox 将领域事件投递到 `profile.engagement.events`，供 Catalog 等下游消费。
> 示例项目：`smiling-landing-472320-q0`。执行命令前请替换为实际项目 ID。

---

## 一、资源创建与权限配置

1. **启用 Pub/Sub API、设置默认项目**
   ```bash
   gcloud services enable pubsub.googleapis.com \
       --project=smiling-landing-472320-q0
   gcloud config set project smiling-landing-472320-q0
   ```

2. **创建服务账号（最小权限）**
   ```bash
   # 发布端：Profile Outbox Publisher
   gcloud iam service-accounts create sa-profile-publisher \
       --display-name="Profile Engagement Publisher"

   # 订阅端：Catalog 或其他消费侧
   gcloud iam service-accounts create sa-catalog-engagement-reader \
       --display-name="Catalog Engagement Consumer"
   ```

3. **注册事件 Schema（Proto）**
   ```bash
   gcloud pubsub schemas create profile-engagement-events \
       --project=smiling-landing-472320-q0 \
       --type=protocol-buffer \
       --definition-file=api/profile/v1/events.proto
   ```
   Schema 载荷覆盖三类事件：
   - `profile.engagement.added`
   - `profile.engagement.removed`
   - `profile.watch.progressed`

4. **创建 Topic 与 DLQ**
   ```bash
   gcloud pubsub topics create profile.engagement.events \
       --project=smiling-landing-472320-q0 \
       --schema=profile-engagement-events \
       --message-encoding=binary

   gcloud pubsub topics create profile.engagement.events.dlq \
       --project=smiling-landing-472320-q0
   ```

5. **为下游消费者创建订阅（示例）**
   ```bash
   gcloud pubsub subscriptions create profile.engagement.events.catalog-runner \
       --project=smiling-landing-472320-q0 \
       --topic=profile.engagement.events \
       --ack-deadline=60 \
       --message-retention-duration=7d \
       --enable-message-ordering \
       --dead-letter-topic=profile.engagement.events.dlq \
       --max-delivery-attempts=5 \
       --min-retry-delay=10s \
       --max-retry-delay=600s
   ```

6. **绑定 IAM 权限**
   ```bash
   gcloud pubsub topics add-iam-policy-binding profile.engagement.events \
       --member=serviceAccount:sa-profile-publisher@smiling-landing-472320-q0.iam.gserviceaccount.com \
       --role=roles/pubsub.publisher

   gcloud pubsub subscriptions add-iam-policy-binding profile.engagement.events.catalog-runner \
       --member=serviceAccount:sa-catalog-engagement-reader@smiling-landing-472320-q0.iam.gserviceaccount.com \
       --role=roles/pubsub.subscriber
   ```

---

## 二、发布端（services-profile）配置

1. **`configs/config.yaml` 示例**
   ```yaml
   messaging:
     schema: profile
     topics:
       default:
         project_id: smiling-landing-472320-q0
         topic_id: profile.engagement.events
         subscription_id: profile.engagement.events.catalog-runner # 仅用于本地测试，可换成任意占位
         dead_letter_topic_id: profile.engagement.events.dlq
         ordering_key_enabled: true
         logging_enabled: true
         metrics_enabled: true
         emulator_endpoint: ""          # 使用 Pub/Sub emulator 时指向 localhost:8085
         publish_timeout: 5s
         receive:
           num_goroutines: 4
           max_outstanding_messages: 500
           max_outstanding_bytes: 67108864
           max_extension: 60s
           max_extension_period: 600s
         exactly_once_delivery: true
     outbox:
       batch_size: 100
       tick_interval: 1s
       initial_backoff: 2s
       max_backoff: 120s
       max_attempts: 20
       publish_timeout: 10s
       workers: 4
       lock_ttl: 120s
       logging_enabled: true
       metrics_enabled: true
   ```

2. **凭据与运行时要求**
   - `services-profile` 在生成 GCS Signed URL 时会读取 ADC 凭据；推荐在本地和 Cloud Run 上设置 `GOOGLE_APPLICATION_CREDENTIALS` 指向包含私钥的 Service Account JSON。
   - 若使用 Base64 形式的凭据，可在启动脚本中解码到临时文件后再设置上述环境变量。

3. **事件发布链路**
   - `EngagementService` 触发互动新增/删除或观看进度更新 → 写入 `profile.outbox_events`。
   - Outbox Runner（`cmd/tasks/outbox`）将 `DomainEvent` 转换为 `profilev1.*Event` Proto，调用 `topic.Publish`。
   - 消息属性包含：`event_type`、`aggregate_id`、`aggregate_type`、`schema_version` 等，OrderingKey 使用 `profile.engagement` 的业务主键（通常是 `user_id` 或 `video_id`）。
   - 发布失败时按配置重试，失败过多会进入 DLQ，需配合监控巡检。

---

## 三、订阅端（示例：services-catalog）配置

1. **为消费侧服务账号授权**
   - 使用上文创建的 `sa-catalog-engagement-reader`，或为现有服务账号授予 `roles/pubsub.subscriber`。

2. **在 `services-catalog` 中的配置片段**（参考）
   ```yaml
   messaging:
     topics:
       engagement:
         project_id: smiling-landing-472320-q0
         topic_id: profile.engagement.events
         subscription_id: profile.engagement.events.catalog-runner
         logging_enabled: true
         metrics_enabled: true
         emulator_endpoint: ""
         publish_timeout: 5s
         receive:
           num_goroutines: 4
           max_outstanding_messages: 1000
           max_outstanding_bytes: 67108864
           max_extension: 60s
           max_extension_period: 600s
   ```

3. **消费流程概述**
   - Catalog 的 Engagement Runner（或其他下游服务）使用 StreamingPull 处理事件：
     1. 反序列化 `profilev1.EngagementAdded/Removed/WatchProgressedEvent`。
     2. 按 `video_id`、`user_id` 更新自身投影（如 `catalog.video_user_engagements_projection`）。
     3. 处理成功后 Ack，失败则 Nack 触发重试，必要时将消息转入 DLQ。
   - 可开启 Exactly-once Delivery + Inbox 幂等，避免重复消费造成的脏数据。

4. **常用运维命令**
   ```bash
   # 临时抽样查看最新事件
   gcloud pubsub subscriptions pull profile.engagement.events.catalog-runner \
       --project=smiling-landing-472320-q0 --limit=5 --auto-ack

   # DLQ 消息巡检
   gcloud pubsub subscriptions pull profile.engagement.events.dlq.monitor \
       --project=smiling-landing-472320-q0 --limit=5 --auto-ack
   ```

---

完成以上配置后，Profile 服务即可安全地将互动类事件发布到 `profile.engagement.events`，并由 Catalog 等下游消费实现跨服务投影。请根据环境需求调整 topic/subscription 命名以及订阅端消费逻辑，并确保在 Cloud Run 或本地环境中正确加载 Service Account 凭据。EOF
