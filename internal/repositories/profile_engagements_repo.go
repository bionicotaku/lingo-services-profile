package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories/mappers"
	profiledb "github.com/bionicotaku/lingo-services-profile/internal/repositories/profiledb"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileEngagementsRepository 维护用户互动记录。
type ProfileEngagementsRepository struct {
	db      *pgxpool.Pool
	queries *profiledb.Queries
	log     *log.Helper
}

// NewProfileEngagementsRepository 构造仓储实例。
func NewProfileEngagementsRepository(db *pgxpool.Pool, logger log.Logger) *ProfileEngagementsRepository {
	return &ProfileEngagementsRepository{
		db:      db,
		queries: profiledb.New(db),
		log:     log.NewHelper(logger),
	}
}

// UpsertProfileEngagementInput 描述互动写入参数。
type UpsertProfileEngagementInput struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string
	OccurredAt     *time.Time
}

// SoftDeleteProfileEngagementInput 描述互动删除参数。
type SoftDeleteProfileEngagementInput struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string
	DeletedAt      *time.Time
}

// Upsert 插入或恢复互动记录。
func (r *ProfileEngagementsRepository) Upsert(ctx context.Context, sess txmanager.Session, input UpsertProfileEngagementInput) error {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	var occurred interface{}
	if input.OccurredAt != nil {
		occurred = *input.OccurredAt
	}
	params := profiledb.UpsertEngagementParams{
		UserID:         input.UserID,
		VideoID:        input.VideoID,
		EngagementType: input.EngagementType,
		Column4:        occurred,
	}
	if err := queries.UpsertEngagement(ctx, params); err != nil {
		r.log.WithContext(ctx).Errorf("upsert engagement failed: user=%s video=%s type=%s err=%v", input.UserID, input.VideoID, input.EngagementType, err)
		return fmt.Errorf("upsert engagement: %w", err)
	}
	return nil
}

// SoftDelete 将互动标记为删除。
func (r *ProfileEngagementsRepository) SoftDelete(ctx context.Context, sess txmanager.Session, input SoftDeleteProfileEngagementInput) error {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	params := profiledb.SoftDeleteEngagementParams{
		UserID:         input.UserID,
		VideoID:        input.VideoID,
		EngagementType: input.EngagementType,
		DeletedAt:      mappers.ToPgTimestamptzPtr(input.DeletedAt),
	}
	if err := queries.SoftDeleteEngagement(ctx, params); err != nil {
		r.log.WithContext(ctx).Errorf("soft delete engagement failed: user=%s video=%s type=%s err=%v", input.UserID, input.VideoID, input.EngagementType, err)
		return fmt.Errorf("soft delete engagement: %w", err)
	}
	return nil
}

// Get 返回互动记录。
func (r *ProfileEngagementsRepository) Get(ctx context.Context, sess txmanager.Session, userID, videoID uuid.UUID, engagementType string) (*po.ProfileEngagement, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	row, err := queries.GetEngagement(ctx, profiledb.GetEngagementParams{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: engagementType,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileEngagementNotFound
		}
		return nil, fmt.Errorf("get engagement: %w", err)
	}
	return mappers.ProfileEngagementFromRow(row), nil
}

// ListByUser 返回用户互动列表。
func (r *ProfileEngagementsRepository) ListByUser(ctx context.Context, sess txmanager.Session, userID uuid.UUID, engagementType *string, includeDeleted bool, limit, offset int32) ([]*po.ProfileEngagement, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	filterType := ""
	if engagementType != nil {
		filterType = *engagementType
	}
	params := profiledb.ListEngagementsByUserParams{
		UserID:  userID,
		Column2: filterType,
		Column3: !includeDeleted,
		Limit:   limit,
		Offset:  offset,
	}
	rows, err := queries.ListEngagementsByUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list engagements: %w", err)
	}
	result := make([]*po.ProfileEngagement, 0, len(rows))
	for _, row := range rows {
		result = append(result, mappers.ProfileEngagementFromRow(row))
	}
	return result, nil
}

// ErrProfileEngagementNotFound 表示互动不存在。
var ErrProfileEngagementNotFound = errors.New("profile engagement not found")
