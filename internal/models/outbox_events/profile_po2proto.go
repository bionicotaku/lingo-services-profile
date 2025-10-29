package outboxevents

import (
	"fmt"

	profilev1 "github.com/bionicotaku/lingo-services-profile/api/profile/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToProfileProto 将 Profile 领域事件转换为 protobuf 载荷。
func ToProfileProto(evt *DomainEvent) (proto.Message, error) {
	if evt == nil {
		return nil, fmt.Errorf("events: nil domain event")
	}

	switch payload := evt.Payload.(type) {
	case *ProfileEngagementAdded:
		return encodeProfileEngagementAdded(evt, payload), nil
	case *ProfileEngagementRemoved:
		return encodeProfileEngagementRemoved(evt, payload), nil
	case *ProfileWatchProgressed:
		return encodeProfileWatchProgressed(evt, payload), nil
	default:
		return nil, fmt.Errorf("events: unsupported profile payload type %T", payload)
	}
}

func encodeProfileEngagementAdded(evt *DomainEvent, payload *ProfileEngagementAdded) *profilev1.EngagementAddedEvent {
	out := &profilev1.EngagementAddedEvent{
		EventId:      evt.EventID.String(),
		UserId:       payload.UserID.String(),
		VideoId:      payload.VideoID.String(),
		FavoriteType: engagementTypeToProto(payload.EngagementType),
		OccurredAt:   timestamppb.New(payload.OccurredAt.UTC()),
	}
	if payload.Source != nil {
		out.Source = *payload.Source
	}
	if payload.Stats != nil {
		out.Stats = toProfileStatsProto(payload.Stats)
	}
	return out
}

func encodeProfileEngagementRemoved(evt *DomainEvent, payload *ProfileEngagementRemoved) *profilev1.EngagementRemovedEvent {
	out := &profilev1.EngagementRemovedEvent{
		EventId:      evt.EventID.String(),
		UserId:       payload.UserID.String(),
		VideoId:      payload.VideoID.String(),
		FavoriteType: engagementTypeToProto(payload.EngagementType),
		OccurredAt:   timestamppb.New(payload.OccurredAt.UTC()),
	}
	if payload.DeletedAt != nil {
		out.DeletedAt = timestamppb.New(payload.DeletedAt.UTC())
	}
	if payload.Source != nil {
		out.Source = *payload.Source
	}
	if payload.Stats != nil {
		out.Stats = toProfileStatsProto(payload.Stats)
	}
	return out
}

func encodeProfileWatchProgressed(evt *DomainEvent, payload *ProfileWatchProgressed) *profilev1.WatchProgressedEvent {
	out := &profilev1.WatchProgressedEvent{
		EventId: evt.EventID.String(),
		UserId:  payload.UserID.String(),
		VideoId: payload.VideoID.String(),
	}
	if payload.Progress != nil {
		out.Progress = toWatchProgressProto(payload.Progress, payload.SessionID)
	}
	if len(payload.Context) > 0 {
		if ctx, err := structpb.NewStruct(payload.Context); err == nil {
			out.Context = ctx
		}
	}
	return out
}

func toProfileStatsProto(stats *po.ProfileVideoStats) *profilev1.VideoStats {
	if stats == nil {
		return nil
	}
	return &profilev1.VideoStats{
		LikeCount:         stats.LikeCount,
		BookmarkCount:     stats.BookmarkCount,
		UniqueWatchers:    stats.UniqueWatchers,
		TotalWatchSeconds: stats.TotalWatchSeconds,
		UpdatedAt:         timestamppb.New(stats.UpdatedAt.UTC()),
	}
}

func toWatchProgressProto(log *po.ProfileWatchLog, sessionID string) *profilev1.WatchProgress {
	if log == nil {
		return nil
	}
	wp := &profilev1.WatchProgress{
		PositionSeconds:   int64(log.PositionSeconds),
		ProgressRatio:     log.ProgressRatio,
		TotalWatchSeconds: int64(log.TotalWatchSeconds),
		FirstWatchedAt:    timestamppb.New(log.FirstWatchedAt.UTC()),
		LastWatchedAt:     timestamppb.New(log.LastWatchedAt.UTC()),
	}
	if log.ExpiresAt != nil {
		wp.ExpiresAt = timestamppb.New(log.ExpiresAt.UTC())
	}
	if sessionID != "" {
		wp.SessionId = sessionID
	}
	return wp
}

func engagementTypeToProto(v string) profilev1.FavoriteType {
	switch v {
	case "like":
		return profilev1.FavoriteType_FAVORITE_TYPE_LIKE
	case "bookmark":
		return profilev1.FavoriteType_FAVORITE_TYPE_BOOKMARK
	default:
		return profilev1.FavoriteType_FAVORITE_TYPE_UNSPECIFIED
	}
}
