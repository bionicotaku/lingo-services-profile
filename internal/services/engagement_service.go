package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	outboxevents "github.com/bionicotaku/lingo-services-profile/internal/models/outbox_events"
	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// EngagementsRepository 抽象互动仓储行为。
type EngagementsRepository interface {
	Upsert(ctx context.Context, sess txmanager.Session, input repositories.UpsertProfileEngagementInput) error
	SoftDelete(ctx context.Context, sess txmanager.Session, input repositories.SoftDeleteProfileEngagementInput) error
	Get(ctx context.Context, sess txmanager.Session, userID, videoID uuid.UUID, engagementType string) (*po.ProfileEngagement, error)
	ListByUser(ctx context.Context, sess txmanager.Session, userID uuid.UUID, engagementType *string, includeDeleted bool, limit, offset int32) ([]*po.ProfileEngagement, error)
}

// EngagementStatsRepository 抽象视频统计增量行为。
type EngagementStatsRepository interface {
	Increment(ctx context.Context, sess txmanager.Session, videoID uuid.UUID, likeDelta, bookmarkDelta, watcherDelta, secondsDelta int64) error
	Get(ctx context.Context, sess txmanager.Session, videoID uuid.UUID) (*po.ProfileVideoStats, error)
}

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
	engagements EngagementsRepository
	stats       EngagementStatsRepository
	outbox      OutboxEnqueuer
	txManager   txmanager.Manager
	log         *log.Helper
	metrics     *outboxMetrics
}

// NewEngagementService 构造 EngagementService。
func NewEngagementService(
	engagements EngagementsRepository,
	stats EngagementStatsRepository,
	outbox OutboxEnqueuer,
	tx txmanager.Manager,
	logger log.Logger,
) *EngagementService {
	return &EngagementService{
		engagements: engagements,
		stats:       stats,
		outbox:      outbox,
		txManager:   tx,
		log:         log.NewHelper(logger),
		metrics:     newOutboxMetrics("engagement"),
	}
}

// MutateEngagementInput 描述互动变更参数。
type MutateEngagementInput struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string // like | bookmark
	Action         EngagementAction
	OccurredAt     *time.Time
	Source         *string
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
		occurredAt := time.Now().UTC()
		if input.OccurredAt != nil {
			occurredAt = input.OccurredAt.UTC()
		}

		var event *outboxevents.DomainEvent
		var statsSnapshot *po.ProfileVideoStats

		fetchStats := func() {
			if s.stats == nil {
				return
			}
			stats, err := s.stats.Get(txCtx, sess, input.VideoID)
			if err != nil {
				s.log.WithContext(txCtx).Warnf("fetch video stats failed: video=%s err=%v", input.VideoID, err)
				return
			}
			statsSnapshot = stats
		}

		switch input.Action {
		case EngagementActionAdd:
			if err := s.engagements.Upsert(txCtx, sess, repositories.UpsertProfileEngagementInput{
				UserID:         input.UserID,
				VideoID:        input.VideoID,
				EngagementType: input.EngagementType,
				OccurredAt:     &occurredAt,
			}); err != nil {
				return err
			}
			if err := s.bumpStats(txCtx, sess, input.VideoID, input.EngagementType, 1); err != nil {
				return err
			}
			fetchStats()
			var err error
			event, err = outboxevents.NewProfileEngagementAddedEvent(input.UserID, input.VideoID, input.EngagementType, occurredAt, input.Source, statsSnapshot)
			if err != nil {
				return err
			}
		case EngagementActionRemove:
			if err := s.engagements.SoftDelete(txCtx, sess, repositories.SoftDeleteProfileEngagementInput{
				UserID:         input.UserID,
				VideoID:        input.VideoID,
				EngagementType: input.EngagementType,
				DeletedAt:      &occurredAt,
			}); err != nil {
				return err
			}
			if err := s.bumpStats(txCtx, sess, input.VideoID, input.EngagementType, -1); err != nil {
				return err
			}
			fetchStats()
			var err error
			event, err = outboxevents.NewProfileEngagementRemovedEvent(input.UserID, input.VideoID, input.EngagementType, occurredAt, &occurredAt, input.Source, statsSnapshot)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("mutate engagement: invalid action %q", input.Action)
		}

		return s.enqueueEvent(txCtx, sess, event)
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

func (s *EngagementService) enqueueEvent(ctx context.Context, sess txmanager.Session, evt *outboxevents.DomainEvent) error {
	if evt == nil || s.outbox == nil {
		return nil
	}
	msg, err := buildOutboxMessage(evt)
	if err != nil {
		if s.metrics != nil {
			s.metrics.recordFailure(ctx, evt.Kind.String(), err)
		}
		return err
	}
	if err := s.outbox.Enqueue(ctx, sess, msg); err != nil {
		if s.metrics != nil {
			s.metrics.recordFailure(ctx, evt.Kind.String(), err)
		}
		return err
	}
	if s.metrics != nil {
		s.metrics.recordSuccess(ctx, evt.Kind.String(), evt.OccurredAt)
	}
	return nil
}

func buildOutboxMessage(evt *outboxevents.DomainEvent) (repositories.OutboxMessage, error) {
	payloadMsg, err := outboxevents.ToProfileProto(evt)
	if err != nil {
		return repositories.OutboxMessage{}, err
	}
	data, err := proto.Marshal(payloadMsg)
	if err != nil {
		return repositories.OutboxMessage{}, fmt.Errorf("marshal event payload: %w", err)
	}
	return repositories.OutboxMessage{
		EventID:       evt.EventID,
		AggregateType: evt.AggregateType,
		AggregateID:   evt.AggregateID,
		EventType:     evt.Kind.String(),
		Payload:       data,
		Headers: map[string]string{
			"schema_version": outboxevents.SchemaVersionV1,
		},
		AvailableAt: evt.OccurredAt,
	}, nil
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

// ListFavoritesInput 描述收藏/点赞列表查询参数。
type ListFavoritesInput struct {
	UserID         uuid.UUID
	EngagementType *string
	IncludeDeleted bool
	Limit          int32
	Offset         int32
}

// ListFavorites 返回用户收藏/点赞列表。
func (s *EngagementService) ListFavorites(ctx context.Context, input ListFavoritesInput) ([]*po.ProfileEngagement, error) {
	items, err := s.engagements.ListByUser(ctx, nil, input.UserID, input.EngagementType, input.IncludeDeleted, input.Limit, input.Offset)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	return items, nil
}
