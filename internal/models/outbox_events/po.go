package outboxevents

import (
	"fmt"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/google/uuid"
)

// Kind 标识领域事件类型。
type Kind int

// 领域事件类型常量。
const (
	// KindUnknown 表示未识别的事件类型。
	KindUnknown Kind = iota
	// KindProfileEngagementAdded 表示用户互动新增事件。
	KindProfileEngagementAdded
	// KindProfileEngagementRemoved 表示用户互动删除事件。
	KindProfileEngagementRemoved
	// KindProfileWatchProgressed 表示观看进度更新事件。
	KindProfileWatchProgressed
)

func (k Kind) String() string {
	switch k {
	case KindProfileEngagementAdded:
		return "profile.engagement.added"
	case KindProfileEngagementRemoved:
		return "profile.engagement.removed"
	case KindProfileWatchProgressed:
		return "profile.watch.progressed"
	default:
		return "profile.event.unknown"
	}
}

// DomainEvent 表示领域层生成的标准事件。
type DomainEvent struct {
	EventID       uuid.UUID
	Kind          Kind
	AggregateID   uuid.UUID
	AggregateType string
	Version       int64
	OccurredAt    time.Time
	Payload       any
}

// ProfileEngagementAdded 描述互动新增事件载荷。
type ProfileEngagementAdded struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string
	OccurredAt     time.Time
	Source         *string
	Stats          *po.ProfileVideoStats
}

// ProfileEngagementRemoved 描述互动删除事件载荷。
type ProfileEngagementRemoved struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string
	OccurredAt     time.Time
	DeletedAt      *time.Time
	Source         *string
	Stats          *po.ProfileVideoStats
}

// ProfileWatchProgressed 描述观看进度更新事件载荷。
type ProfileWatchProgressed struct {
	UserID    uuid.UUID
	VideoID   uuid.UUID
	Progress  *po.ProfileWatchLog
	SessionID string
	Context   map[string]any
}

const (
	// AggregateTypeProfileUser 标识档案聚合类型。
	AggregateTypeProfileUser = "profile.user"
	// AggregateTypeProfileEngagement 标识互动聚合类型。
	AggregateTypeProfileEngagement = "profile.engagement"
	// AggregateTypeProfileWatchLog 标识观看记录聚合类型。
	AggregateTypeProfileWatchLog = "profile.watch_log"
	// SchemaVersionV1 描述事件载荷的当前 schema 版本。
	SchemaVersionV1 = "v1"
)

var (
	// ErrInvalidEventID 表示未提供合法的事件 ID。
	ErrInvalidEventID = fmt.Errorf("event builder: event id is required")
	// ErrUnknownEventKind 表示未识别的事件类型。
	ErrUnknownEventKind = fmt.Errorf("event builder: unknown event kind")
)
