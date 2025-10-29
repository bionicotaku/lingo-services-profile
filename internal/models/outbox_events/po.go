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
	// KindVideoCreated 表示视频创建事件。
	KindVideoCreated
	// KindVideoUpdated 表示视频更新事件。
	KindVideoUpdated
	// KindVideoDeleted 表示视频删除事件。
	KindVideoDeleted
	// KindVideoMediaReady 表示媒体阶段完成事件。
	KindVideoMediaReady
	// KindVideoAIEnriched 表示 AI 阶段完成事件。
	KindVideoAIEnriched
	// KindVideoProcessingFailed 表示处理阶段失败事件。
	KindVideoProcessingFailed
	// KindVideoVisibilityChanged 表示可见性变更事件。
	KindVideoVisibilityChanged
	// KindProfileEngagementAdded 表示用户互动新增事件。
	KindProfileEngagementAdded
	// KindProfileEngagementRemoved 表示用户互动删除事件。
	KindProfileEngagementRemoved
	// KindProfileWatchProgressed 表示观看进度更新事件。
	KindProfileWatchProgressed
)

func (k Kind) String() string {
	switch k {
	case KindVideoCreated:
		return "catalog.video.created"
	case KindVideoUpdated:
		return "catalog.video.updated"
	case KindVideoDeleted:
		return "catalog.video.deleted"
	case KindVideoMediaReady:
		return "catalog.video.media_ready"
	case KindVideoAIEnriched:
		return "catalog.video.ai_enriched"
	case KindVideoProcessingFailed:
		return "catalog.video.processing_failed"
	case KindVideoVisibilityChanged:
		return "catalog.video.visibility_changed"
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

// VideoCreated 描述视频创建事件的业务载荷。
type VideoCreated struct {
	VideoID        uuid.UUID
	UploaderID     uuid.UUID
	Title          string
	Description    *string
	DurationMicros *int64
	PublishedAt    *time.Time
	Status         string
	MediaStatus    string
	AnalysisStatus string
}

// VideoUpdated 描述视频更新事件的业务载荷。
type VideoUpdated struct {
	VideoID           uuid.UUID
	Title             *string
	Description       *string
	Status            *string
	MediaStatus       *string
	AnalysisStatus    *string
	DurationMicros    *int64
	ThumbnailURL      *string
	HLSMasterPlaylist *string
	Difficulty        *string
	Summary           *string
	Tags              []string
	RawSubtitleURL    *string
	VisibilityStatus  *string
	PublishedAt       *time.Time
}

// VideoDeleted 描述视频删除事件的业务载荷。
type VideoDeleted struct {
	VideoID   uuid.UUID
	DeletedAt *time.Time
	Reason    *string
}

// VideoMediaReady 描述媒体阶段完成事件载荷。
type VideoMediaReady struct {
	VideoID           uuid.UUID
	Status            po.VideoStatus
	MediaStatus       po.StageStatus
	AnalysisStatus    po.StageStatus
	DurationMicros    *int64
	EncodedResolution *string
	EncodedBitrate    *int32
	ThumbnailURL      *string
	HLSMasterPlaylist *string
	JobID             *string
	EmittedAt         *time.Time
}

// VideoAIEnriched 描述 AI 阶段完成事件载荷。
type VideoAIEnriched struct {
	VideoID        uuid.UUID
	Status         po.VideoStatus
	AnalysisStatus po.StageStatus
	MediaStatus    po.StageStatus
	Difficulty     *string
	Summary        *string
	Tags           []string
	RawSubtitleURL *string
	JobID          *string
	EmittedAt      *time.Time
	ErrorMessage   *string
}

// VideoProcessingFailed 描述处理阶段失败事件载荷。
type VideoProcessingFailed struct {
	VideoID        uuid.UUID
	Status         po.VideoStatus
	MediaStatus    po.StageStatus
	AnalysisStatus po.StageStatus
	Stage          string
	ErrorMessage   *string
	JobID          *string
	EmittedAt      *time.Time
}

// VideoVisibilityChanged 描述可见性变更事件载荷。
type VideoVisibilityChanged struct {
	VideoID          uuid.UUID
	Status           po.VideoStatus
	VisibilityStatus string
	PreviousStatus   *po.VideoStatus
	PublishedAt      *time.Time
	Reason           *string
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
	// AggregateTypeVideo 标识视频聚合类型，供 Outbox headers / attributes 使用。
	AggregateTypeVideo = "video"
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
	// ErrNilVideo 在构建事件时视频实体为空。
	ErrNilVideo = fmt.Errorf("event builder: video is nil")
	// ErrInvalidEventID 表示未提供合法的事件 ID。
	ErrInvalidEventID = fmt.Errorf("event builder: event id is required")
	// ErrUnknownEventKind 表示未识别的事件类型。
	ErrUnknownEventKind = fmt.Errorf("event builder: unknown event kind")
	// ErrInvalidStage 表示阶段名称非法。
	ErrInvalidStage = fmt.Errorf("event builder: invalid stage")
)
