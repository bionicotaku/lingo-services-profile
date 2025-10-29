package services

import (
	"context"
	"fmt"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// VideoProjectionService 负责维护和查询视频投影。
type VideoProjectionService struct {
	repo *repositories.ProfileVideoProjectionRepository
	log  *log.Helper
}

// NewVideoProjectionService 构造 VideoProjectionService。
func NewVideoProjectionService(repo *repositories.ProfileVideoProjectionRepository, logger log.Logger) *VideoProjectionService {
	return &VideoProjectionService{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// UpsertProjection 写入投影记录。
func (s *VideoProjectionService) UpsertProjection(ctx context.Context, input repositories.UpsertVideoProjectionInput) error {
	if input.VideoID == uuid.Nil {
		return fmt.Errorf("upsert projection: video_id required")
	}
	if err := s.repo.Upsert(ctx, nil, input); err != nil {
		return fmt.Errorf("upsert projection: %w", err)
	}
	return nil
}

// GetProjection 返回视频投影。
func (s *VideoProjectionService) GetProjection(ctx context.Context, videoID uuid.UUID) (*po.ProfileVideoProjection, error) {
	if videoID == uuid.Nil {
		return nil, fmt.Errorf("get projection: video_id required")
	}
	record, err := s.repo.Get(ctx, nil, videoID)
	if err != nil {
		return nil, fmt.Errorf("get projection: %w", err)
	}
	return record, nil
}

// ListProjections 批量查询投影。
func (s *VideoProjectionService) ListProjections(ctx context.Context, videoIDs []uuid.UUID) ([]*po.ProfileVideoProjection, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}
	items, err := s.repo.ListByIDs(ctx, nil, videoIDs)
	if err != nil {
		return nil, fmt.Errorf("list projections: %w", err)
	}
	return items, nil
}
