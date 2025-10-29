package mappers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	profiledb "github.com/bionicotaku/lingo-services-profile/internal/repositories/profiledb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// BuildUpsertProfileUserParams 构造 UpsertProfileUserParams。
func BuildUpsertProfileUserParams(userID uuid.UUID, displayName string, avatarURL *string, version int64, preferences map[string]any) (profiledb.UpsertProfileUserParams, error) {
	if preferences == nil {
		preferences = map[string]any{}
	}
	payload, err := json.Marshal(preferences)
	if err != nil {
		return profiledb.UpsertProfileUserParams{}, fmt.Errorf("marshal preferences: %w", err)
	}
	return profiledb.UpsertProfileUserParams{
		UserID:          userID,
		DisplayName:     displayName,
		AvatarUrl:       ToPgText(avatarURL),
		ProfileVersion:  version,
		PreferencesJson: payload,
	}, nil
}

// ProfileUserFromRow 将 sqlc ProfileUser 转换为领域对象。
func ProfileUserFromRow(row profiledb.ProfileUser) (*po.ProfileUser, error) {
	prefs := map[string]any{}
	if len(row.PreferencesJson) > 0 {
		if err := json.Unmarshal(row.PreferencesJson, &prefs); err != nil {
			return nil, fmt.Errorf("unmarshal preferences: %w", err)
		}
	}
	return &po.ProfileUser{
		UserID:          row.UserID,
		DisplayName:     row.DisplayName,
		AvatarURL:       textPtr(row.AvatarUrl),
		ProfileVersion:  row.ProfileVersion,
		PreferencesJSON: prefs,
		CreatedAt:       mustTimestamp(row.CreatedAt),
		UpdatedAt:       mustTimestamp(row.UpdatedAt),
	}, nil
}

// ProfileEngagementFromRow 转换互动记录。
func ProfileEngagementFromRow(row profiledb.ProfileEngagement) *po.ProfileEngagement {
	return &po.ProfileEngagement{
		UserID:         row.UserID,
		VideoID:        row.VideoID,
		EngagementType: row.EngagementType,
		CreatedAt:      mustTimestamp(row.CreatedAt),
		UpdatedAt:      mustTimestamp(row.UpdatedAt),
		DeletedAt:      timestampPtr(row.DeletedAt),
	}
}

// ProfileWatchLogFromRow 转换观看记录。
func ProfileWatchLogFromRow(row profiledb.ProfileWatchLog) *po.ProfileWatchLog {
	return &po.ProfileWatchLog{
		UserID:            row.UserID,
		VideoID:           row.VideoID,
		PositionSeconds:   numericToFloat64(row.PositionSeconds),
		ProgressRatio:     numericToFloat64(row.ProgressRatio),
		TotalWatchSeconds: numericToFloat64(row.TotalWatchSeconds),
		FirstWatchedAt:    mustTimestamp(row.FirstWatchedAt),
		LastWatchedAt:     mustTimestamp(row.LastWatchedAt),
		ExpiresAt:         timestampPtr(row.ExpiresAt),
		RedactedAt:        timestampPtr(row.RedactedAt),
		CreatedAt:         mustTimestamp(row.CreatedAt),
		UpdatedAt:         mustTimestamp(row.UpdatedAt),
	}
}

// ProfileVideoProjectionFromRow 转换视频投影。
func ProfileVideoProjectionFromRow(row profiledb.ProfileVideosProjection) *po.ProfileVideoProjection {
	return &po.ProfileVideoProjection{
		VideoID:           row.VideoID,
		Title:             row.Title,
		Description:       textPtr(row.Description),
		DurationMicros:    int8Ptr(row.DurationMicros),
		ThumbnailURL:      textPtr(row.ThumbnailUrl),
		HLSMasterPlaylist: textPtr(row.HlsMasterPlaylist),
		Status:            textPtr(row.Status),
		VisibilityStatus:  textPtr(row.VisibilityStatus),
		PublishedAt:       timestampPtr(row.PublishedAt),
		Version:           row.Version,
		UpdatedAt:         mustTimestamp(row.UpdatedAt),
	}
}

// ProfileVideoStatsFromRow 转换聚合统计。
func ProfileVideoStatsFromRow(row profiledb.ProfileVideoStat) *po.ProfileVideoStats {
	return &po.ProfileVideoStats{
		VideoID:           row.VideoID,
		LikeCount:         row.LikeCount,
		BookmarkCount:     row.BookmarkCount,
		UniqueWatchers:    row.UniqueWatchers,
		TotalWatchSeconds: row.TotalWatchSeconds,
		UpdatedAt:         mustTimestamp(row.UpdatedAt),
	}
}

// ToPgNumeric 将 float64 转换为 pgtype.Numeric。
func ToPgNumeric(value float64) pgtype.Numeric {
	var num pgtype.Numeric
	if err := num.Scan(value); err != nil {
		return pgtype.Numeric{}
	}
	return num
}

// numericToFloat64 将 pgtype.Numeric 转换为 float64。
func numericToFloat64(num pgtype.Numeric) float64 {
	if !num.Valid {
		return 0
	}
	if val, err := num.Float64Value(); err == nil && val.Valid {
		return val.Float64
	}
	if val, err := num.Int64Value(); err == nil && val.Valid {
		return float64(val.Int64)
	}
	return 0
}

// ToPgTimestamptzPtr 将 *time.Time 转换为 pgtype.Timestamptz。
func ToPgTimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// ToPgInt8 将 *int64 转换为 pgtype.Int8。
func ToPgInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

// ToPgText 将 *string 转换为 pgtype.Text。
func ToPgText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func mustTimestamp(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

func timestampPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func int8Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}
