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
