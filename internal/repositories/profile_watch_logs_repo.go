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

// ErrProfileWatchLogNotFound 表示观看记录不存在。
var ErrProfileWatchLogNotFound = errors.New("profile watch log not found")

// ProfileWatchLogsRepository 访问 profile.watch_logs。
type ProfileWatchLogsRepository struct {
	db      *pgxpool.Pool
	queries *profiledb.Queries
	log     *log.Helper
}

// NewProfileWatchLogsRepository 构造仓储实例。
func NewProfileWatchLogsRepository(db *pgxpool.Pool, logger log.Logger) *ProfileWatchLogsRepository {
	return &ProfileWatchLogsRepository{
		db:      db,
		queries: profiledb.New(db),
		log:     log.NewHelper(logger),
	}
}

// UpsertWatchLogInput 描述观看记录写入参数。
type UpsertWatchLogInput struct {
	UserID              uuid.UUID
	VideoID             uuid.UUID
	PositionSeconds     float64
	ProgressRatio       float64
	TotalWatchSeconds   float64
	FirstWatchedAt      *time.Time
	LastWatchedAt       *time.Time
	ExpiresAt           *time.Time
	RedactedAt          *time.Time
	IncrementWatchDelta float64
}

// Upsert 插入或更新观看记录。
func (r *ProfileWatchLogsRepository) Upsert(ctx context.Context, sess txmanager.Session, input UpsertWatchLogInput) error {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	params := profiledb.UpsertWatchLogParams{
		UserID:              input.UserID,
		VideoID:             input.VideoID,
		PositionSeconds:     mappers.ToPgNumeric(input.PositionSeconds),
		ProgressRatio:       mappers.ToPgNumeric(input.ProgressRatio),
		TotalWatchSeconds:   mappers.ToPgNumeric(input.TotalWatchSeconds),
		Column6:             input.FirstWatchedAt,
		Column7:             input.LastWatchedAt,
		ExpiresAt:           mappers.ToPgTimestamptzPtr(input.ExpiresAt),
		RedactedAt:          mappers.ToPgTimestamptzPtr(input.RedactedAt),
		TotalWatchSeconds_2: mappers.ToPgNumeric(input.IncrementWatchDelta),
	}
	if err := queries.UpsertWatchLog(ctx, params); err != nil {
		r.log.WithContext(ctx).Errorf("upsert watch log failed: user=%s video=%s err=%v", input.UserID, input.VideoID, err)
		return fmt.Errorf("upsert watch log: %w", err)
	}
	return nil
}

// Get 返回观看记录。
func (r *ProfileWatchLogsRepository) Get(ctx context.Context, sess txmanager.Session, userID, videoID uuid.UUID) (*po.ProfileWatchLog, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	row, err := queries.GetWatchLog(ctx, profiledb.GetWatchLogParams{UserID: userID, VideoID: videoID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileWatchLogNotFound
		}
		return nil, fmt.Errorf("get watch log: %w", err)
	}
	return mappers.ProfileWatchLogFromRow(row), nil
}

// ListByUser 返回观看历史。
func (r *ProfileWatchLogsRepository) ListByUser(ctx context.Context, sess txmanager.Session, userID uuid.UUID, includeRedacted bool, limit, offset int32) ([]*po.ProfileWatchLog, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}
	params := profiledb.ListWatchLogsByUserParams{
		UserID:  userID,
		Column2: !includeRedacted,
		Limit:   limit,
		Offset:  offset,
	}
	rows, err := queries.ListWatchLogsByUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list watch logs: %w", err)
	}
	result := make([]*po.ProfileWatchLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, mappers.ProfileWatchLogFromRow(row))
	}
	return result, nil
}
