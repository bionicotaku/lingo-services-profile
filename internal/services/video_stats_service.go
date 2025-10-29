package services

import (
	"context"
	"fmt"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-utils/txmanager"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// VideoStatsRepository 抽象视频统计仓储接口，便于测试替换。
type VideoStatsRepository interface {
	Get(ctx context.Context, sess txmanager.Session, videoID uuid.UUID) (*po.ProfileVideoStats, error)
	ListByIDs(ctx context.Context, sess txmanager.Session, videoIDs []uuid.UUID) ([]*po.ProfileVideoStats, error)
}

// VideoStatsService 提供视频聚合统计的读取能力。
type VideoStatsService struct {
	repo VideoStatsRepository
	log  *log.Helper
}

// NewVideoStatsService 构造 VideoStatsService。
func NewVideoStatsService(repo VideoStatsRepository, logger log.Logger) *VideoStatsService {
	return &VideoStatsService{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// GetStats 返回单个视频的统计。
func (s *VideoStatsService) GetStats(ctx context.Context, videoID uuid.UUID) (*po.ProfileVideoStats, error) {
	if videoID == uuid.Nil {
		return nil, fmt.Errorf("get stats: video_id required")
	}
	stats, err := s.repo.Get(ctx, nil, videoID)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	return stats, nil
}

// ListStats 批量返回统计。
func (s *VideoStatsService) ListStats(ctx context.Context, ids []uuid.UUID) ([]*po.ProfileVideoStats, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	items, err := s.repo.ListByIDs(ctx, nil, ids)
	if err != nil {
		return nil, fmt.Errorf("list stats: %w", err)
	}
	return items, nil
}
