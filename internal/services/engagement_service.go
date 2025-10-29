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

// EngagementAction 指定互动动作。
type EngagementAction string

const (
	// EngagementActionAdd 表示新增互动。
	EngagementActionAdd EngagementAction = "add"
	// EngagementActionRemove 表示取消互动。
	EngagementActionRemove EngagementAction = "remove"
)

// ErrUnsupportedEngagementType 表示互动类型不受支持。
var ErrUnsupportedEngagementType = errors.New("unsupported engagement type")

// EngagementService 处理收藏/点赞等互动逻辑。
type EngagementService struct {
	engagements *repositories.ProfileEngagementsRepository
	stats       *repositories.ProfileVideoStatsRepository
	txManager   txmanager.Manager
	log         *log.Helper
}

// NewEngagementService 构造 EngagementService。
func NewEngagementService(
	engagements *repositories.ProfileEngagementsRepository,
	stats *repositories.ProfileVideoStatsRepository,
	tx txmanager.Manager,
	logger log.Logger,
) *EngagementService {
	return &EngagementService{
		engagements: engagements,
		stats:       stats,
		txManager:   tx,
		log:         log.NewHelper(logger),
	}
}

// MutateEngagementInput 描述互动变更参数。
type MutateEngagementInput struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string // like | bookmark
	Action         EngagementAction
	OccurredAt     *time.Time
}

// Mutate 执行点赞/收藏新增或移除，并更新统计聚合。
func (s *EngagementService) Mutate(ctx context.Context, input MutateEngagementInput) error {
	if !isSupportedEngagement(input.EngagementType) {
		return ErrUnsupportedEngagementType
	}
	if input.UserID == uuid.Nil || input.VideoID == uuid.Nil {
		return fmt.Errorf("mutate engagement: missing identifiers")
	}

	return s.txManager.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		switch input.Action {
		case EngagementActionAdd:
			if err := s.engagements.Upsert(txCtx, sess, repositories.UpsertProfileEngagementInput{
				UserID:         input.UserID,
				VideoID:        input.VideoID,
				EngagementType: input.EngagementType,
				OccurredAt:     input.OccurredAt,
			}); err != nil {
				return err
			}
			return s.bumpStats(txCtx, sess, input.VideoID, input.EngagementType, 1)
		case EngagementActionRemove:
			if err := s.engagements.SoftDelete(txCtx, sess, repositories.SoftDeleteProfileEngagementInput{
				UserID:         input.UserID,
				VideoID:        input.VideoID,
				EngagementType: input.EngagementType,
				DeletedAt:      input.OccurredAt,
			}); err != nil {
				return err
			}
			return s.bumpStats(txCtx, sess, input.VideoID, input.EngagementType, -1)
		default:
			return fmt.Errorf("mutate engagement: invalid action %q", input.Action)
		}
	})
}

func (s *EngagementService) bumpStats(ctx context.Context, sess txmanager.Session, videoID uuid.UUID, engagementType string, delta int64) error {
	if s.stats == nil {
		return nil
	}
	likeDelta, bookmarkDelta := int64(0), int64(0)
	switch engagementType {
	case "like":
		likeDelta = delta
	case "bookmark":
		bookmarkDelta = delta
	default:
		return ErrUnsupportedEngagementType
	}
	if err := s.stats.Increment(ctx, sess, videoID, likeDelta, bookmarkDelta, 0, 0); err != nil {
		return fmt.Errorf("update stats: %w", err)
	}
	return nil
}

func isSupportedEngagement(kind string) bool {
	return kind == "like" || kind == "bookmark"
}

// FavoriteState 描述用户对单个视频的互动状态。
type FavoriteState struct {
	HasLiked      bool
	HasBookmarked bool
}

// GetFavoriteState 返回用户对视频的互动状态。
func (s *EngagementService) GetFavoriteState(ctx context.Context, userID, videoID uuid.UUID) (FavoriteState, error) {
	state := FavoriteState{}
	like, err := s.engagements.Get(ctx, nil, userID, videoID, "like")
	if err == nil && like.DeletedAt == nil {
		state.HasLiked = true
	}
	if err != nil && !errors.Is(err, repositories.ErrProfileEngagementNotFound) {
		return state, fmt.Errorf("get like: %w", err)
	}

	bookmark, err := s.engagements.Get(ctx, nil, userID, videoID, "bookmark")
	if err == nil && bookmark.DeletedAt == nil {
		state.HasBookmarked = true
	}
	if err != nil && !errors.Is(err, repositories.ErrProfileEngagementNotFound) {
		return state, fmt.Errorf("get bookmark: %w", err)
	}

	return state, nil
}

// ListFavorites 返回用户收藏/点赞列表。
type ListFavoritesInput struct {
	UserID         uuid.UUID
	EngagementType *string
	IncludeDeleted bool
	Limit          int32
	Offset         int32
}

func (s *EngagementService) ListFavorites(ctx context.Context, input ListFavoritesInput) ([]*po.ProfileEngagement, error) {
	items, err := s.engagements.ListByUser(ctx, nil, input.UserID, input.EngagementType, input.IncludeDeleted, input.Limit, input.Offset)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	return items, nil
}
