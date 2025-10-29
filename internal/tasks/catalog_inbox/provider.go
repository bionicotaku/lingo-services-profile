package cataloginbox

import (
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/gcpubsub"
	outboxcfg "github.com/bionicotaku/lingo-utils/outbox/config"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
)

// ProvideTask 根据配置和依赖构造 Catalog Inbox 任务。
func ProvideTask(
	subscriber gcpubsub.Subscriber,
	inboxRepo *repositories.InboxRepository,
	projectionRepo *repositories.ProfileVideoProjectionRepository,
	tx txmanager.Manager,
	cfg outboxcfg.Config,
	logger log.Logger,
) *Task {
	normalized := cfg.Normalize()
	if normalized.Inbox.SourceService == "" {
		log.NewHelper(logger).Warn("catalog inbox: skip initialization, source_service not configured")
		return nil
	}
	return NewTask(subscriber, inboxRepo, projectionRepo, tx, logger, normalized.Inbox)
}
