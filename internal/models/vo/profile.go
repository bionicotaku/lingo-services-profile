// Package vo 定义 Profile 服务在控制器与外部交互使用的视图对象。
package vo

import (
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
)

// Profile 表示向上层返回的档案视图。
type Profile struct {
	UserID         string
	DisplayName    string
	AvatarURL      *string
	ProfileVersion int64
	Preferences    Preferences
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Preferences 表示结构化的偏好设置。
type Preferences struct {
	LearningGoal      *string
	DailyQuotaMinutes *int32
	Extra             map[string]any
}

// FavoriteState 表示收藏/点赞状态。
type FavoriteState struct {
	HasLiked      bool
	HasBookmarked bool
	LikedAt       *time.Time
	BookmarkedAt  *time.Time
}

// ProfileVideoStats 表示聚合统计。
type ProfileVideoStats struct {
	LikeCount         int64
	BookmarkCount     int64
	UniqueWatchers    int64
	TotalWatchSeconds int64
	UpdatedAt         time.Time
}

// FavoriteItem 表示收藏列表项。
type FavoriteItem struct {
	VideoID      string
	FavoriteType string
	State        FavoriteState
	Video        *ProfileVideoMetadata
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FavoriteSummary 表示批量查询摘要。
type FavoriteSummary struct {
	VideoID string
	State   FavoriteState
	Stats   *ProfileVideoStats
}

// WatchProgress 表示观看进度。
type WatchProgress struct {
	PositionSeconds   float64
	ProgressRatio     float64
	TotalWatchSeconds float64
	FirstWatchedAt    time.Time
	LastWatchedAt     time.Time
	ExpiresAt         *time.Time
	SessionID         string
}

// WatchHistoryItem 表示观看历史条目。
type WatchHistoryItem struct {
	VideoID  string
	Progress WatchProgress
	Video    *ProfileVideoMetadata
}

// ProfileVideoMetadata 表示从投影补水的元数据。
type ProfileVideoMetadata struct {
	VideoID           string
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

// NewProfileFromPO 将档案 PO 转换为 VO。
func NewProfileFromPO(poProfile *po.ProfileUser, prefs Preferences) *Profile {
	if poProfile == nil {
		return nil
	}
	return &Profile{
		UserID:         poProfile.UserID.String(),
		DisplayName:    poProfile.DisplayName,
		AvatarURL:      poProfile.AvatarURL,
		ProfileVersion: poProfile.ProfileVersion,
		Preferences:    prefs,
		CreatedAt:      poProfile.CreatedAt,
		UpdatedAt:      poProfile.UpdatedAt,
	}
}
