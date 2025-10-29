package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	outboxevents "github.com/bionicotaku/lingo-services-profile/internal/models/outbox_events"
	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// WatchLogsRepository 抽象 watch_logs 仓储行为，便于测试替换。
type WatchLogsRepository interface {
	Get(ctx context.Context, sess txmanager.Session, userID, videoID uuid.UUID) (*po.ProfileWatchLog, error)
	Upsert(ctx context.Context, sess txmanager.Session, input repositories.UpsertWatchLogInput) error
	ListByUser(ctx context.Context, sess txmanager.Session, userID uuid.UUID, includeRedacted bool, limit, offset int32) ([]*po.ProfileWatchLog, error)
}

// WatchStatsRepository 抽象视频统计仓储行为。
type WatchStatsRepository interface {
	Increment(ctx context.Context, sess txmanager.Session, videoID uuid.UUID, likeDelta, bookmarkDelta, watcherDelta, secondsDelta int64) error
}

// OutboxEnqueuer 抽象 Outbox 写入行为，供服务层与测试复用。
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, sess txmanager.Session, msg repositories.OutboxMessage) error
}

// WatchHistoryService 负责观看进度写入与查询。
type WatchHistoryService struct {
	logs      WatchLogsRepository
	stats     WatchStatsRepository
	outbox    OutboxEnqueuer
	txManager txmanager.Manager
	log       *log.Helper
	metrics   *outboxMetrics
}

// NewWatchHistoryService 构造 WatchHistoryService。
func NewWatchHistoryService(
	logs WatchLogsRepository,
	stats WatchStatsRepository,
	outbox OutboxEnqueuer,
	tx txmanager.Manager,
	logger log.Logger,
) *WatchHistoryService {
	return &WatchHistoryService{
		logs:      logs,
		stats:     stats,
		outbox:    outbox,
		txManager: tx,
		log:       log.NewHelper(logger),
		metrics:   newOutboxMetrics("watch_history"),
	}
}

// UpsertWatchProgressInput 描述观看进度上报参数。
type UpsertWatchProgressInput struct {
	UserID            uuid.UUID
	VideoID           uuid.UUID
	PositionSeconds   float64
	ProgressRatio     float64
	TotalWatchSeconds float64
	FirstWatchedAt    *time.Time
	LastWatchedAt     *time.Time
	ExpiresAt         *time.Time
	RedactedAt        *time.Time
	SessionID         string
}

// UpsertProgress 写入或更新观看记录，并根据需要更新统计。
func (s *WatchHistoryService) UpsertProgress(ctx context.Context, input UpsertWatchProgressInput) (*po.ProfileWatchLog, error) {
	if input.UserID == uuid.Nil || input.VideoID == uuid.Nil {
		return nil, fmt.Errorf("upsert watch progress: missing identifiers")
	}

	var result *po.ProfileWatchLog
	err := s.txManager.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		existing, err := s.logs.Get(txCtx, sess, input.UserID, input.VideoID)
		if err != nil && !errors.Is(err, repositories.ErrProfileWatchLogNotFound) {
			return err
		}

		deltaSeconds := computeWatchSecondsDelta(existing, input.TotalWatchSeconds)
		if deltaSeconds < 0 {
			deltaSeconds = 0
		}
		increment := repositories.UpsertWatchLogInput{
			UserID:              input.UserID,
			VideoID:             input.VideoID,
			PositionSeconds:     input.PositionSeconds,
			ProgressRatio:       input.ProgressRatio,
			TotalWatchSeconds:   input.TotalWatchSeconds,
			FirstWatchedAt:      input.FirstWatchedAt,
			LastWatchedAt:       input.LastWatchedAt,
			ExpiresAt:           input.ExpiresAt,
			RedactedAt:          input.RedactedAt,
			IncrementWatchDelta: deltaSeconds,
		}
		if err := s.logs.Upsert(txCtx, sess, increment); err != nil {
			return err
		}

		updated, err := s.logs.Get(txCtx, sess, input.UserID, input.VideoID)
		if err != nil {
			return err
		}
		result = updated

		if s.stats != nil {
			watcherDelta := computeWatcherDelta(existing, updated)
			secondsDelta := int64(math.Round(deltaSeconds))
			if secondsDelta != 0 || watcherDelta != 0 {
				if err := s.stats.Increment(txCtx, sess, input.VideoID, 0, 0, watcherDelta, secondsDelta); err != nil {
					return err
				}
			}
		}

		if s.outbox != nil && shouldEmitWatchEvent(existing, updated) {
			occurredAt := updated.LastWatchedAt
			evt, err := outboxevents.NewProfileWatchProgressedEvent(input.UserID, input.VideoID, updated, occurredAt, input.SessionID, nil)
			if err != nil {
				if s.metrics != nil {
					s.metrics.recordFailure(txCtx, outboxevents.KindProfileWatchProgressed.String(), err)
				}
				return err
			}
			msg, err := buildOutboxMessage(evt)
			if err != nil {
				if s.metrics != nil {
					s.metrics.recordFailure(txCtx, evt.Kind.String(), err)
				}
				return err
			}
			if err := s.outbox.Enqueue(txCtx, sess, msg); err != nil {
				if s.metrics != nil {
					s.metrics.recordFailure(txCtx, evt.Kind.String(), err)
				}
				return err
			}
			if s.metrics != nil {
				s.metrics.recordSuccess(txCtx, evt.Kind.String(), evt.OccurredAt)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListWatchHistoryInput 描述观看历史查询参数。
type ListWatchHistoryInput struct {
	UserID          uuid.UUID
	IncludeRedacted bool
	Limit           int32
	Offset          int32
}

// ListWatchHistory 返回观看记录列表。
func (s *WatchHistoryService) ListWatchHistory(ctx context.Context, input ListWatchHistoryInput) ([]*po.ProfileWatchLog, error) {
	if input.UserID == uuid.Nil {
		return nil, fmt.Errorf("list watch history: user_id required")
	}
	items, err := s.logs.ListByUser(ctx, nil, input.UserID, input.IncludeRedacted, input.Limit, input.Offset)
	if err != nil {
		return nil, fmt.Errorf("list watch history: %w", err)
	}
	return items, nil
}

const (
	progressQualifiedThreshold = 0.05
	progressDeltaThreshold     = 0.05
)

func computeWatchSecondsDelta(existing *po.ProfileWatchLog, newTotal float64) float64 {
	if existing == nil {
		if newTotal < 0 {
			return 0
		}
		return newTotal
	}
	delta := newTotal - existing.TotalWatchSeconds
	if delta < 0 {
		return 0
	}
	return delta
}

func computeWatcherDelta(existing, updated *po.ProfileWatchLog) int64 {
	if updated == nil {
		return 0
	}
	newQualified := updated.ProgressRatio >= progressQualifiedThreshold
	oldQualified := existing != nil && existing.ProgressRatio >= progressQualifiedThreshold
	switch {
	case !oldQualified && newQualified:
		return 1
	case oldQualified && !newQualified:
		return -1
	default:
		return 0
	}
}

func shouldEmitWatchEvent(existing, updated *po.ProfileWatchLog) bool {
	if updated == nil {
		return false
	}
	if updated.ProgressRatio < progressQualifiedThreshold {
		return false
	}
	if existing == nil {
		return true
	}
	if existing.ProgressRatio < progressQualifiedThreshold {
		return true
	}
	delta := updated.ProgressRatio - existing.ProgressRatio
	return delta >= progressDeltaThreshold
}
