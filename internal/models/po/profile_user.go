package po

import (
	"time"

	"github.com/google/uuid"
)

// ProfileUser 表示 profile.users 表中的档案记录。
type ProfileUser struct {
	UserID          uuid.UUID
	DisplayName     string
	AvatarURL       *string
	ProfileVersion  int64
	PreferencesJSON map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ProfileEngagement 表示 profile.engagements 表的行。
type ProfileEngagement struct {
	UserID         uuid.UUID
	VideoID        uuid.UUID
	EngagementType string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// ProfileWatchLog 表示 profile.watch_logs 表的行。
type ProfileWatchLog struct {
	UserID            uuid.UUID
	VideoID           uuid.UUID
	PositionSeconds   float64
	ProgressRatio     float64
	TotalWatchSeconds float64
	FirstWatchedAt    time.Time
	LastWatchedAt     time.Time
	ExpiresAt         *time.Time
	RedactedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProfileVideoProjection 表示 profile.videos_projection 投影表。
type ProfileVideoProjection struct {
	VideoID           uuid.UUID
	Title             string
	Description       *string
	DurationMicros    *int64
	ThumbnailURL      *string
	HLSMasterPlaylist *string
	Status            *string
	VisibilityStatus  *string
	PublishedAt       *time.Time
	Version           int64
	UpdatedAt         time.Time
}

// ProfileVideoStats 表示 profile.video_stats 聚合表。
type ProfileVideoStats struct {
	VideoID           uuid.UUID
	LikeCount         int64
	BookmarkCount     int64
	UniqueWatchers    int64
	TotalWatchSeconds int64
	UpdatedAt         time.Time
}
