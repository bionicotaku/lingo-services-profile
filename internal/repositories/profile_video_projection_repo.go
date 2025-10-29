package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories/mappers"
	profiledb "github.com/bionicotaku/lingo-services-profile/internal/repositories/profiledb"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileVideoProjectionRepository 维护 profile.videos_projection。
type ProfileVideoProjectionRepository struct {
	db      *pgxpool.Pool
	queries *profiledb.Queries
	log     *log.Helper
}

// NewProfileVideoProjectionRepository 构造仓储实例。
func NewProfileVideoProjectionRepository(db *pgxpool.Pool, logger log.Logger) *ProfileVideoProjectionRepository {
	return &ProfileVideoProjectionRepository{
		db:      db,
		queries: profiledb.New(db),
		log:     log.NewHelper(logger),
	}
}

// UpsertVideoProjectionInput 描述投影写入参数。
type UpsertVideoProjectionInput struct {
	VideoID           uuid.UUID
	Title             string
	Description       *string
	DurationMicros    *int64
	ThumbnailURL      *string
	HLSMasterPlaylist *string
	Status            *string
	VisibilityStatus  *string
	PublishedAt       *time.Time
	Version           int64
	UpdatedAt         *time.Time
}

// Upsert 写入投影记录。
func (r *ProfileVideoProjectionRepository) Upsert(ctx context.Context, sess txmanager.Session, input UpsertVideoProjectionInput) error {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	params := profiledb.UpsertVideoProjectionParams{
		VideoID:           input.VideoID,
		Title:             input.Title,
		Description:       mappers.ToPgText(input.Description),
		DurationMicros:    mappers.ToPgInt8(input.DurationMicros),
		ThumbnailUrl:      mappers.ToPgText(input.ThumbnailURL),
		HlsMasterPlaylist: mappers.ToPgText(input.HLSMasterPlaylist),
		Status:            mappers.ToPgText(input.Status),
		VisibilityStatus:  mappers.ToPgText(input.VisibilityStatus),
		PublishedAt:       mappers.ToPgTimestamptzPtr(input.PublishedAt),
		Version:           input.Version,
		Column11:          mappers.ToPgTimestamptzPtr(input.UpdatedAt),
	}
	if err := queries.UpsertVideoProjection(ctx, params); err != nil {
		r.log.WithContext(ctx).Errorf("upsert video projection failed: video=%s err=%v", input.VideoID, err)
		return fmt.Errorf("upsert video projection: %w", err)
	}
	return nil
}

// Get 返回单个投影。
func (r *ProfileVideoProjectionRepository) Get(ctx context.Context, sess txmanager.Session, videoID uuid.UUID) (*po.ProfileVideoProjection, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	row, err := queries.GetVideoProjection(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("get video projection: %w", err)
	}
	return mappers.ProfileVideoProjectionFromRow(row), nil
}

// ListByIDs 批量读取投影。
func (r *ProfileVideoProjectionRepository) ListByIDs(ctx context.Context, sess txmanager.Session, ids []uuid.UUID) ([]*po.ProfileVideoProjection, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	rows, err := queries.ListVideoProjections(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list video projections: %w", err)
	}
	result := make([]*po.ProfileVideoProjection, 0, len(rows))
	for _, row := range rows {
		result = append(result, mappers.ProfileVideoProjectionFromRow(row))
	}
	return result, nil
}
