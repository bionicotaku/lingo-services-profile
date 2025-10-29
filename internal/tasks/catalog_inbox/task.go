package cataloginbox

import (
	"context"
	"time"

	videov1 "github.com/bionicotaku/lingo-services-catalog/api/video/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/gcpubsub"
	outboxcfg "github.com/bionicotaku/lingo-utils/outbox/config"
	"github.com/bionicotaku/lingo-utils/outbox/inbox"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
)

// Task 封装 Catalog Inbox 消费逻辑。
type Task struct {
	runner *inbox.Runner[videov1.Event]
}

// NewTask 构造 Inbox Runner。
func NewTask(
	subscriber gcpubsub.Subscriber,
	inboxRepo *repositories.InboxRepository,
	projection *repositories.ProfileVideoProjectionRepository,
	tx txmanager.Manager,
	logger log.Logger,
	cfg outboxcfg.InboxConfig,
) *Task {
	if subscriber == nil || inboxRepo == nil || projection == nil || tx == nil {
		return nil
	}

	metrics := newInboxMetrics()
	handler := newEventHandler(projection, logger, metrics)
	dec := newDecoder()

	runner, err := inbox.NewRunner[videov1.Event](inbox.RunnerParams[videov1.Event]{
		Store:      inboxRepo.Shared(),
		Subscriber: subscriber,
		TxManager:  tx,
		Decoder:    dec,
		Handler:    handler,
		Config:     cfg.Normalize(),
		Logger:     logger,
	})
	if err != nil {
		log.NewHelper(logger).Errorw("msg", "catalog inbox: init runner failed", "error", err)
		return nil
	}

	task := &Task{runner: runner}
	task.runner.WithClock(time.Now)
	return task
}

// Run 启动消费循环。
func (t *Task) Run(ctx context.Context) error {
	if t == nil || t.runner == nil {
		return nil
	}
	return t.runner.Run(ctx)
}

// WithClock 提供测试替换时间。
func (t *Task) WithClock(fn func() time.Time) {
	if t == nil || t.runner == nil || fn == nil {
		return
	}
	t.runner.WithClock(fn)
}
