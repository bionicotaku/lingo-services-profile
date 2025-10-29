package outboxevents

import (
	"fmt"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/google/uuid"
)

// NewProfileEngagementAddedEvent 构造收藏/点赞新增事件。
func NewProfileEngagementAddedEvent(userID, videoID uuid.UUID, engagementType string, occurredAt time.Time, source *string, stats *po.ProfileVideoStats) (*DomainEvent, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("engagement event: user_id required")
	}
	if videoID == uuid.Nil {
		return nil, fmt.Errorf("engagement event: video_id required")
	}
	if engagementType == "" {
		return nil, fmt.Errorf("engagement event: type required")
	}
	evt := &DomainEvent{
		EventID:       uuid.New(),
		Kind:          KindProfileEngagementAdded,
		AggregateID:   videoID,
		AggregateType: AggregateTypeProfileEngagement,
		Version:       VersionFromTime(occurredAt),
		OccurredAt:    occurredAt,
		Payload: &ProfileEngagementAdded{
			UserID:         userID,
			VideoID:        videoID,
			EngagementType: engagementType,
			OccurredAt:     occurredAt,
			Source:         source,
			Stats:          stats,
		},
	}
	return evt, nil
}

// NewProfileEngagementRemovedEvent 构造收藏/点赞撤销事件。
func NewProfileEngagementRemovedEvent(userID, videoID uuid.UUID, engagementType string, occurredAt time.Time, deletedAt *time.Time, source *string, stats *po.ProfileVideoStats) (*DomainEvent, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("engagement event: user_id required")
	}
	if videoID == uuid.Nil {
		return nil, fmt.Errorf("engagement event: video_id required")
	}
	if engagementType == "" {
		return nil, fmt.Errorf("engagement event: type required")
	}
	evt := &DomainEvent{
		EventID:       uuid.New(),
		Kind:          KindProfileEngagementRemoved,
		AggregateID:   videoID,
		AggregateType: AggregateTypeProfileEngagement,
		Version:       VersionFromTime(occurredAt),
		OccurredAt:    occurredAt,
		Payload: &ProfileEngagementRemoved{
			UserID:         userID,
			VideoID:        videoID,
			EngagementType: engagementType,
			OccurredAt:     occurredAt,
			DeletedAt:      deletedAt,
			Source:         source,
			Stats:          stats,
		},
	}
	return evt, nil
}

// NewProfileWatchProgressedEvent 构造观看进度更新事件。
func NewProfileWatchProgressedEvent(userID, videoID uuid.UUID, progress *po.ProfileWatchLog, occurredAt time.Time, sessionID string, ctx map[string]any) (*DomainEvent, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("watch progressed event: user_id required")
	}
	if videoID == uuid.Nil {
		return nil, fmt.Errorf("watch progressed event: video_id required")
	}
	if progress == nil {
		return nil, fmt.Errorf("watch progressed event: progress required")
	}
	eventTime := occurredAt
	if eventTime.IsZero() {
		eventTime = progress.LastWatchedAt
	}
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	} else {
		eventTime = eventTime.UTC()
	}
	evt := &DomainEvent{
		EventID:       uuid.New(),
		Kind:          KindProfileWatchProgressed,
		AggregateID:   videoID,
		AggregateType: AggregateTypeProfileWatchLog,
		Version:       VersionFromTime(eventTime),
		OccurredAt:    eventTime,
		Payload: &ProfileWatchProgressed{
			UserID:    userID,
			VideoID:   videoID,
			Progress:  progress,
			SessionID: sessionID,
			Context:   ctx,
		},
	}
	return evt, nil
}
