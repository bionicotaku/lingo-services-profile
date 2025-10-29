package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories/mappers"
	profiledb "github.com/bionicotaku/lingo-services-profile/internal/repositories/profiledb"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrProfileUserNotFound 表示档案不存在。
var ErrProfileUserNotFound = errors.New("profile user not found")

// ProfileUsersRepository 提供访问 profile.users 的接口。
type ProfileUsersRepository struct {
	db      *pgxpool.Pool
	queries *profiledb.Queries
	log     *log.Helper
}

// NewProfileUsersRepository 构造仓储实例。
func NewProfileUsersRepository(db *pgxpool.Pool, logger log.Logger) *ProfileUsersRepository {
	return &ProfileUsersRepository{
		db:      db,
		queries: profiledb.New(db),
		log:     log.NewHelper(logger),
	}
}

// UpsertProfileUserInput 描述档案写入参数。
type UpsertProfileUserInput struct {
	UserID         uuid.UUID
	DisplayName    string
	AvatarURL      *string
	ProfileVersion int64
	Preferences    map[string]any
}

// Upsert 写入或更新档案记录。
func (r *ProfileUsersRepository) Upsert(ctx context.Context, sess txmanager.Session, input UpsertProfileUserInput) (*po.ProfileUser, error) {
	params, err := mappers.BuildUpsertProfileUserParams(input.UserID, input.DisplayName, input.AvatarURL, input.ProfileVersion, input.Preferences)
	if err != nil {
		r.log.WithContext(ctx).Errorf("upsert profile user: marshal preferences failed: user=%s err=%v", input.UserID, err)
		return nil, fmt.Errorf("marshal preferences: %w", err)
	}

	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}

	row, err := queries.UpsertProfileUser(ctx, params)
	if err != nil {
		r.log.WithContext(ctx).Errorf("upsert profile user failed: user=%s err=%v", input.UserID, err)
		return nil, fmt.Errorf("upsert profile user: %w", err)
	}

	profile, err := mappers.ProfileUserFromRow(row)
	if err != nil {
		r.log.WithContext(ctx).Errorf("convert profile user failed: user=%s err=%v", input.UserID, err)
		return nil, err
	}
	return profile, nil
}

// Get 返回档案记录。
func (r *ProfileUsersRepository) Get(ctx context.Context, sess txmanager.Session, userID uuid.UUID) (*po.ProfileUser, error) {
	queries := r.queries
	if sess != nil {
		queries = queries.WithTx(sess.Tx())
	}

	row, err := queries.GetProfileUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileUserNotFound
		}
		return nil, fmt.Errorf("get profile user: %w", err)
	}
	profile, err := mappers.ProfileUserFromRow(row)
	if err != nil {
		return nil, err
	}
	return profile, nil
}
