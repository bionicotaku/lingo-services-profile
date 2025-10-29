package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// WatchHistoryService 负责观看进度写入与查询。
type WatchHistoryService struct {
	logs      *repositories.ProfileWatchLogsRepository
	stats     *repositories.ProfileVideoStatsRepository
	txManager txmanager.Manager
	log       *log.Helper
}

// NewWatchHistoryService 构造 WatchHistoryService。
func NewWatchHistoryService(
	logs *repositories.ProfileWatchLogsRepository,
	stats *repositories.ProfileVideoStatsRepository,
	tx txmanager.Manager,
	logger log.Logger,
) *WatchHistoryService {
	return &WatchHistoryService{
		logs:      logs,
		stats:     stats,
		txManager: tx,
		log:       log.NewHelper(logger),
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

		deltaSeconds := input.TotalWatchSeconds
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
			watcherDelta := int64(0)
			if existing == nil {
				watcherDelta = 1
			}
			secondsDelta := int64(deltaSeconds)
			if secondsDelta != 0 || watcherDelta != 0 {
				if err := s.stats.Increment(txCtx, sess, input.VideoID, 0, 0, watcherDelta, secondsDelta); err != nil {
					return err
				}
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

func errorsIsWatchLogNotFound(err error) bool {
	return err != nil && errors.Is(err, repositories.ErrProfileWatchLogNotFound)
}
