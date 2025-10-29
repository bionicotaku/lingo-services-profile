package services

import (
	"context"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/models/vo"
	"github.com/google/uuid"
)

// ProfileServiceInterface 抽象 Profile 档案/偏好用例，便于测试替换。
type ProfileServiceInterface interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*vo.Profile, error)
	UpdateProfile(ctx context.Context, input UpdateProfileInput) (*vo.Profile, error)
	UpdatePreferences(ctx context.Context, input UpdatePreferencesInput) (*vo.Profile, error)
}

// EngagementServiceInterface 抽象互动用例。
type EngagementServiceInterface interface {
	Mutate(ctx context.Context, input MutateEngagementInput) error
	GetFavoriteState(ctx context.Context, userID, videoID uuid.UUID) (FavoriteState, error)
	ListFavorites(ctx context.Context, input ListFavoritesInput) ([]*po.ProfileEngagement, error)
}

// WatchHistoryServiceInterface 抽象观看历史用例。
type WatchHistoryServiceInterface interface {
	UpsertProgress(ctx context.Context, input UpsertWatchProgressInput) (*po.ProfileWatchLog, error)
	ListWatchHistory(ctx context.Context, input ListWatchHistoryInput) ([]*po.ProfileWatchLog, error)
}

// VideoProjectionServiceInterface 抽象视频投影读取。
type VideoProjectionServiceInterface interface {
	ListProjections(ctx context.Context, videoIDs []uuid.UUID) ([]*po.ProfileVideoProjection, error)
}

// VideoStatsServiceInterface 抽象视频统计读取。
type VideoStatsServiceInterface interface {
	GetStats(ctx context.Context, videoID uuid.UUID) (*po.ProfileVideoStats, error)
	ListStats(ctx context.Context, videoIDs []uuid.UUID) ([]*po.ProfileVideoStats, error)
}

var (
	_ ProfileServiceInterface         = (*ProfileService)(nil)
	_ EngagementServiceInterface      = (*EngagementService)(nil)
	_ WatchHistoryServiceInterface    = (*WatchHistoryService)(nil)
	_ VideoProjectionServiceInterface = (*VideoProjectionService)(nil)
	_ VideoStatsServiceInterface      = (*VideoStatsService)(nil)
)
