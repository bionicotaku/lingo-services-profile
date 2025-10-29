// Package dto 提供控制层与 protobuf 之间的转换工具。
package dto

import (
	"time"

	profilev1 "github.com/bionicotaku/lingo-services-profile/api/profile/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/models/vo"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ToProtoProfile 将领域视图对象转换为 gRPC Profile。
func ToProtoProfile(profile *vo.Profile) *profilev1.Profile {
	if profile == nil {
		return nil
	}
	return &profilev1.Profile{
		UserId:         profile.UserID,
		DisplayName:    profile.DisplayName,
		AvatarUrl:      valueOrEmpty(profile.AvatarURL),
		ProfileVersion: profile.ProfileVersion,
		Preferences:    toProtoPreferences(profile.Preferences),
		CreatedAt:      unixTime(profile.CreatedAt),
		UpdatedAt:      unixTime(profile.UpdatedAt),
	}
}

func toProtoPreferences(prefs vo.Preferences) *profilev1.Preferences {
	extra, _ := structpb.NewStruct(prefs.Extra)
	proto := &profilev1.Preferences{Extra: extra}
	if prefs.LearningGoal != nil {
		proto.LearningGoal = *prefs.LearningGoal
	}
	if prefs.DailyQuotaMinutes != nil {
		proto.DailyQuotaMinutes = wrapperspb.Int32(*prefs.DailyQuotaMinutes)
	}
	return proto
}

// ToProtoFavoriteState 转换收藏状态。
func ToProtoFavoriteState(state vo.FavoriteState) *profilev1.FavoriteState {
	return &profilev1.FavoriteState{
		HasLiked:      state.HasLiked,
		HasBookmarked: state.HasBookmarked,
		LikedAt:       timePtr(state.LikedAt),
		BookmarkedAt:  timePtr(state.BookmarkedAt),
	}
}

// ToProtoVideoStats 转换视频聚合统计。
func ToProtoVideoStats(stats *vo.ProfileVideoStats) *profilev1.VideoStats {
	if stats == nil {
		return nil
	}
	return &profilev1.VideoStats{
		LikeCount:         stats.LikeCount,
		BookmarkCount:     stats.BookmarkCount,
		UniqueWatchers:    stats.UniqueWatchers,
		TotalWatchSeconds: stats.TotalWatchSeconds,
		UpdatedAt:         unixTime(stats.UpdatedAt),
	}
}

// ToProtoVideoMetadata 转换视频元数据补水信息。
func ToProtoVideoMetadata(meta *vo.ProfileVideoMetadata) *profilev1.VideoMetadata {
	if meta == nil {
		return nil
	}
	return &profilev1.VideoMetadata{
		VideoId:           meta.VideoID,
		Title:             meta.Title,
		Description:       valueOrEmpty(meta.Description),
		DurationMicros:    derefInt64(meta.DurationMicros),
		ThumbnailUrl:      valueOrEmpty(meta.ThumbnailURL),
		HlsMasterPlaylist: valueOrEmpty(meta.HLSMasterPlaylist),
		Status:            valueOrEmpty(meta.Status),
		VisibilityStatus:  valueOrEmpty(meta.VisibilityStatus),
		PublishedAt:       timePtr(meta.PublishedAt),
		Version:           meta.Version,
		UpdatedAt:         unixTime(meta.UpdatedAt),
	}
}

// ToProtoWatchProgress 转换观看进度信息。
func ToProtoWatchProgress(progress *vo.WatchProgress) *profilev1.WatchProgress {
	if progress == nil {
		return nil
	}
	return &profilev1.WatchProgress{
		PositionSeconds:   int64(progress.PositionSeconds),
		ProgressRatio:     progress.ProgressRatio,
		TotalWatchSeconds: int64(progress.TotalWatchSeconds),
		FirstWatchedAt:    unixTime(progress.FirstWatchedAt),
		LastWatchedAt:     unixTime(progress.LastWatchedAt),
		ExpiresAt:         timePtr(progress.ExpiresAt),
		SessionId:         progress.SessionID,
	}
}

func valueOrEmpty(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func derefInt64(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func timePtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(t.UTC())
}

func unixTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t.UTC())
}
