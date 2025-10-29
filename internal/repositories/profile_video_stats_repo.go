package repositories

import (
	"context"
	"fmt"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories/mappers"
	profiledb "github.com/bionicotaku/lingo-services-profile/internal/repositories/profiledb"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileVideoStatsRepository 维护 profile.video_stats。
type ProfileVideoStatsRepository struct {
	db      *pgxpool.Pool
	queries *profiledb.Queries
	log     *log.Helper
}

// NewProfileVideoStatsRepository 构造仓储实例。
func NewProfileVideoStatsRepository(db *pgxpool.Pool, logger log.Logger) *ProfileVideoStatsRepository {
	return &ProfileVideoStatsRepository{
		db:      db,
		queries: profiledb.New(db),
		log:     log.NewHelper(logger),
	}
}

// Increment 以增量方式更新统计。
func (r *ProfileVideoStatsRepository) Increment(ctx context.Context, sess txmanager.Session, videoID uuid.UUID, likeDelta, bookmarkDelta, watcherDelta, watchSecondsDelta int64) error {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	params := profiledb.UpsertVideoStatsParams{
		VideoID:           videoID,
		LikeCount:         likeDelta,
		BookmarkCount:     bookmarkDelta,
		UniqueWatchers:    watcherDelta,
		TotalWatchSeconds: watchSecondsDelta,
		Column6:           nil,
	}
	if err := queries.UpsertVideoStats(ctx, params); err != nil {
		r.log.WithContext(ctx).Errorf("increment video stats failed: video=%s err=%v", videoID, err)
		return fmt.Errorf("increment video stats: %w", err)
	}
	return nil
}

// Set 覆盖写统计。
func (r *ProfileVideoStatsRepository) Set(ctx context.Context, sess txmanager.Session, stats po.ProfileVideoStats) error {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	params := profiledb.SetVideoStatsParams{
		VideoID:           stats.VideoID,
		LikeCount:         stats.LikeCount,
		BookmarkCount:     stats.BookmarkCount,
		UniqueWatchers:    stats.UniqueWatchers,
		TotalWatchSeconds: stats.TotalWatchSeconds,
		UpdatedAt:         mappers.ToPgTimestamptzPtr(&stats.UpdatedAt),
	}
	if err := queries.SetVideoStats(ctx, params); err != nil {
		r.log.WithContext(ctx).Errorf("set video stats failed: video=%s err=%v", stats.VideoID, err)
		return fmt.Errorf("set video stats: %w", err)
	}
	return nil
}

// Get 返回统计。
func (r *ProfileVideoStatsRepository) Get(ctx context.Context, sess txmanager.Session, videoID uuid.UUID) (*po.ProfileVideoStats, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	row, err := queries.GetVideoStats(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("get video stats: %w", err)
	}
	return mappers.ProfileVideoStatsFromRow(row), nil
}

// ListByIDs 批量获取统计。
func (r *ProfileVideoStatsRepository) ListByIDs(ctx context.Context, sess txmanager.Session, ids []uuid.UUID) ([]*po.ProfileVideoStats, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	rows, err := queries.ListVideoStats(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list video stats: %w", err)
	}
	result := make([]*po.ProfileVideoStats, 0, len(rows))
	for _, row := range rows {
		result = append(result, mappers.ProfileVideoStatsFromRow(row))
	}
	return result, nil
}
