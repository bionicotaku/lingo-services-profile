package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	profilev1 "github.com/bionicotaku/lingo-services-profile/api/profile/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/controllers/dto"
	"github.com/bionicotaku/lingo-services-profile/internal/metadata"
	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/models/vo"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// ProfileHandler 实现 ProfileService gRPC 接口。
type ProfileHandler struct {
	profilev1.UnimplementedProfileServiceServer

	*BaseHandler
	profiles     *services.ProfileService
	engagements  *services.EngagementService
	watchHistory *services.WatchHistoryService
	projections  *services.VideoProjectionService
	stats        *services.VideoStatsService
}

// NewProfileHandler 构造 ProfileHandler。
func NewProfileHandler(
	profiles *services.ProfileService,
	engagements *services.EngagementService,
	watchHistory *services.WatchHistoryService,
	projections *services.VideoProjectionService,
	stats *services.VideoStatsService,
	base *BaseHandler,
) *ProfileHandler {
	if base == nil {
		base = NewBaseHandler(HandlerTimeouts{})
	}
	return &ProfileHandler{
		BaseHandler:  base,
		profiles:     profiles,
		engagements:  engagements,
		watchHistory: watchHistory,
		projections:  projections,
		stats:        stats,
	}
}

// GetProfile 返回档案。
func (h *ProfileHandler) GetProfile(ctx context.Context, req *profilev1.GetProfileRequest) (*profilev1.GetProfileResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeQuery)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	profile, err := h.profiles.GetProfile(timeoutCtx, userID)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return &profilev1.GetProfileResponse{Profile: dto.ToProtoProfile(profile)}, nil
}

// UpdateProfile 更新档案基础信息。
func (h *ProfileHandler) UpdateProfile(ctx context.Context, req *profilev1.UpdateProfileRequest) (*profilev1.UpdateProfileResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input, err := buildUpdateProfileInput(userID, req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeCommand)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	profile, err := h.profiles.UpdateProfile(timeoutCtx, input)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return &profilev1.UpdateProfileResponse{Profile: dto.ToProtoProfile(profile)}, nil
}

// UpdatePreferences 更新偏好字段。
func (h *ProfileHandler) UpdatePreferences(ctx context.Context, req *profilev1.UpdatePreferencesRequest) (*profilev1.UpdatePreferencesResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input, err := buildUpdatePreferencesInput(userID, req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeCommand)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	profile, err := h.profiles.UpdatePreferences(timeoutCtx, input)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return &profilev1.UpdatePreferencesResponse{Profile: dto.ToProtoProfile(profile)}, nil
}

// MutateFavorite 新增或取消收藏/点赞。
func (h *ProfileHandler) MutateFavorite(ctx context.Context, req *profilev1.MutateFavoriteRequest) (*profilev1.MutateFavoriteResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	videoID, err := parseUUID(req.GetVideoId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid video_id: %v", err)
	}
	typeStr, err := favoriteTypeToString(req.GetFavoriteType())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	action, err := favoriteActionToEnum(req.GetAction())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	var occurred *time.Time
	if ts := req.GetOccurredAt(); ts != nil {
		value := ts.AsTime().UTC()
		occurred = &value
	}

	input := services.MutateEngagementInput{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: typeStr,
		Action:         action,
		OccurredAt:     occurred,
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeCommand)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	if err := h.engagements.Mutate(timeoutCtx, input); err != nil {
		return nil, mapEngagementError(err)
	}

	state, err := h.engagements.GetFavoriteState(timeoutCtx, userID, videoID)
	if err != nil && !errors.Is(err, repositories.ErrProfileEngagementNotFound) {
		return nil, mapEngagementError(err)
	}

	stats, err := h.stats.GetStats(timeoutCtx, videoID)
	if err != nil && !isStatsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "query stats: %v", err)
	}

	return &profilev1.MutateFavoriteResponse{
		State: dto.ToProtoFavoriteState(stateToVO(state)),
		Stats: dto.ToProtoVideoStats(statsToVO(stats)),
	}, nil
}

// BatchQueryFavorite 批量查询收藏状态。
func (h *ProfileHandler) BatchQueryFavorite(ctx context.Context, req *profilev1.BatchQueryFavoriteRequest) (*profilev1.BatchQueryFavoriteResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeQuery)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	videoIDs, err := parseUUIDs(req.GetVideoIds())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid video_ids: %v", err)
	}

	statsMap := map[uuid.UUID]*vo.ProfileVideoStats{}
	if req.GetIncludeStats() && len(videoIDs) > 0 {
		statsSlice, err := h.stats.ListStats(timeoutCtx, videoIDs)
		if err != nil && !isStatsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "list stats: %v", err)
		}
		for _, item := range statsSlice {
			statsMap[item.VideoID] = statsToVO(item)
		}
	}

	summaries := make([]*profilev1.FavoriteSummary, 0, len(videoIDs))
	for _, vid := range videoIDs {
		state, err := h.engagements.GetFavoriteState(timeoutCtx, userID, vid)
		if err != nil && !errors.Is(err, repositories.ErrProfileEngagementNotFound) {
			return nil, mapEngagementError(err)
		}
		sum := &profilev1.FavoriteSummary{
			VideoId: vid.String(),
			State:   dto.ToProtoFavoriteState(stateToVO(state)),
		}
		if req.GetIncludeStats() {
			if stats := statsMap[vid]; stats != nil {
				sum.Stats = dto.ToProtoVideoStats(stats)
			}
		}
		summaries = append(summaries, sum)
	}
	return &profilev1.BatchQueryFavoriteResponse{Summaries: summaries}, nil
}

// ListFavorites 返回收藏列表。
func (h *ProfileHandler) ListFavorites(ctx context.Context, req *profilev1.ListFavoritesRequest) (*profilev1.ListFavoritesResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	limit, offset, err := parsePagination(req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeQuery)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	items, err := h.engagements.ListFavorites(timeoutCtx, services.ListFavoritesInput{
		UserID:         userID,
		EngagementType: nil,
		IncludeDeleted: false,
		Limit:          limit + 1,
		Offset:         int32(offset),
	})
	if err != nil {
		return nil, mapEngagementError(err)
	}

	nextToken := ""
	if len(items) > int(limit) {
		nextToken = strconv.Itoa(offset + int(limit))
		items = items[:limit]
	}

	videoIDs := uniqueVideoIDs(items)
	metaMap := map[uuid.UUID]*vo.ProfileVideoMetadata{}
	if len(videoIDs) > 0 {
		proj, err := h.projections.ListProjections(timeoutCtx, videoIDs)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list projections: %v", err)
		}
		for _, p := range proj {
			metaMap[p.VideoID] = projectionToMetadataVO(p)
		}
	}

	statsMap := map[uuid.UUID]*vo.ProfileVideoStats{}
	if len(videoIDs) > 0 {
		statsSlice, err := h.stats.ListStats(timeoutCtx, videoIDs)
		if err != nil && !isStatsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "list stats: %v", err)
		}
		for _, s := range statsSlice {
			statsMap[s.VideoID] = statsToVO(s)
		}
	}

	favorites := make([]*profilev1.FavoriteItem, 0, len(items))
	for _, item := range items {
		state := vo.FavoriteState{}
		if item.EngagementType == "like" && item.DeletedAt == nil {
			state.HasLiked = true
		}
		if item.EngagementType == "bookmark" && item.DeletedAt == nil {
			state.HasBookmarked = true
		}

		favType, _ := favoriteTypeFromString(item.EngagementType)
		favorites = append(favorites, &profilev1.FavoriteItem{
			VideoId:      item.VideoID.String(),
			FavoriteType: favType,
			State:        dto.ToProtoFavoriteState(state),
			Video:        dto.ToProtoVideoMetadata(metaMap[item.VideoID]),
			CreatedAt:    timestamppb.New(item.CreatedAt.UTC()),
			UpdatedAt:    timestamppb.New(item.UpdatedAt.UTC()),
		})
	}

	return &profilev1.ListFavoritesResponse{
		Favorites:     favorites,
		NextPageToken: nextToken,
	}, nil
}

// UpsertWatchProgress 写入观看进度。
func (h *ProfileHandler) UpsertWatchProgress(ctx context.Context, req *profilev1.UpsertWatchProgressRequest) (*profilev1.UpsertWatchProgressResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	videoID, err := parseUUID(req.GetVideoId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid video_id: %v", err)
	}

	progress := req.GetProgress()
	if progress == nil {
		return nil, status.Errorf(codes.InvalidArgument, "progress is required")
	}

	input := services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   float64(progress.GetPositionSeconds()),
		ProgressRatio:     progress.GetProgressRatio(),
		TotalWatchSeconds: float64(progress.GetTotalWatchSeconds()),
		FirstWatchedAt:    tsToPointer(progress.GetFirstWatchedAt()),
		LastWatchedAt:     tsToPointer(progress.GetLastWatchedAt()),
		ExpiresAt:         tsToPointer(progress.GetExpiresAt()),
		SessionID:         progress.GetSessionId(),
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeCommand)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	logRecord, err := h.watchHistory.UpsertProgress(timeoutCtx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert watch log: %v", err)
	}

	stats, err := h.stats.GetStats(timeoutCtx, videoID)
	if err != nil && !isStatsNotFound(err) {
		return nil, status.Errorf(codes.Internal, "query stats: %v", err)
	}

	return &profilev1.UpsertWatchProgressResponse{
		Progress: dto.ToProtoWatchProgress(watchLogToVO(logRecord, input.SessionID)),
		Stats:    dto.ToProtoVideoStats(statsToVO(stats)),
	}, nil
}

// ListWatchHistory 返回观看历史。
func (h *ProfileHandler) ListWatchHistory(ctx context.Context, req *profilev1.ListWatchHistoryRequest) (*profilev1.ListWatchHistoryResponse, error) {
	meta := h.ExtractMetadata(ctx)
	userID, err := h.resolveUserID(req.GetUserId(), meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	limit, offset, err := parsePagination(req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	timeoutCtx, cancel := h.WithTimeout(ctx, HandlerTypeQuery)
	defer cancel()
	timeoutCtx = InjectHandlerMetadata(timeoutCtx, meta)

	items, err := h.watchHistory.ListWatchHistory(timeoutCtx, services.ListWatchHistoryInput{
		UserID:          userID,
		IncludeRedacted: false,
		Limit:           limit + 1,
		Offset:          int32(offset),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list watch history: %v", err)
	}

	nextToken := ""
	if len(items) > int(limit) {
		nextToken = strconv.Itoa(offset + int(limit))
		items = items[:limit]
	}

	videoIDs := uniqueWatchVideoIDs(items)
	metaMap := map[uuid.UUID]*vo.ProfileVideoMetadata{}
	if len(videoIDs) > 0 {
		proj, err := h.projections.ListProjections(timeoutCtx, videoIDs)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list projections: %v", err)
		}
		for _, p := range proj {
			metaMap[p.VideoID] = projectionToMetadataVO(p)
		}
	}

	entries := make([]*profilev1.WatchHistoryEntry, 0, len(items))
	for _, item := range items {
		entries = append(entries, &profilev1.WatchHistoryEntry{
			VideoId:  item.VideoID.String(),
			Progress: dto.ToProtoWatchProgress(watchLogToVO(item, "")),
			Video:    dto.ToProtoVideoMetadata(metaMap[item.VideoID]),
		})
	}

	return &profilev1.ListWatchHistoryResponse{
		Items:         entries,
		NextPageToken: nextToken,
	}, nil
}

// PurgeUserData 暂未实现，返回未实现错误。
func (h *ProfileHandler) PurgeUserData(context.Context, *profilev1.PurgeUserDataRequest) (*profilev1.PurgeUserDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "purge user data not implemented")
}

// 辅助函数

func (h *ProfileHandler) resolveUserID(requestUserID string, meta metadata.HandlerMetadata) (uuid.UUID, error) {
	if strings.TrimSpace(requestUserID) != "" {
		return parseUUID(requestUserID)
	}
	if strings.TrimSpace(meta.UserID) != "" {
		return parseUUID(meta.UserID)
	}
	return uuid.Nil, fmt.Errorf("user_id required")
}

func buildUpdateProfileInput(userID uuid.UUID, req *profilev1.UpdateProfileRequest) (services.UpdateProfileInput, error) {
	var displayName *string
	var avatarURL *string
	var prefsPatch *vo.Preferences

	mask := maskSet(req.GetUpdateMask().GetPaths())
	should := func(path string) bool {
		if len(mask) == 0 {
			return true
		}
		return mask[path]
	}

	if profile := req.GetProfile(); profile != nil {
		if should("profile.display_name") {
			displayName = stringPtr(profile.GetDisplayName())
		}
		if should("profile.avatar_url") {
			avatar := profile.GetAvatarUrl()
			avatarURL = &avatar
		}
		if should("profile.preferences") || should("profile.preferences.learning_goal") || should("profile.preferences.daily_quota_minutes") || should("profile.preferences.extra") {
			prefs := vo.Preferences{Extra: map[string]any{}}
			if prefsPB := profile.GetPreferences(); prefsPB != nil {
				if should("profile.preferences.learning_goal") && strings.TrimSpace(prefsPB.GetLearningGoal()) != "" {
					lg := prefsPB.GetLearningGoal()
					prefs.LearningGoal = &lg
				}
				if should("profile.preferences.daily_quota_minutes") && prefsPB.GetDailyQuotaMinutes() != nil {
					val := prefsPB.GetDailyQuotaMinutes().GetValue()
					prefs.DailyQuotaMinutes = &val
				}
				if extra := prefsPB.GetExtra(); extra != nil {
					prefs.Extra = extra.AsMap()
				}
			}
			prefsPatch = &prefs
		}
	}

	var expectedVersion *int64
	if v := req.GetExpectedProfileVersion(); v != nil {
		value := v.GetValue()
		expectedVersion = &value
	}

	return services.UpdateProfileInput{
		UserID:           userID,
		DisplayName:      displayName,
		AvatarURL:        avatarURL,
		ExpectedVersion:  expectedVersion,
		PreferencesPatch: prefsPatch,
	}, nil
}

func buildUpdatePreferencesInput(userID uuid.UUID, req *profilev1.UpdatePreferencesRequest) (services.UpdatePreferencesInput, error) {
	mask := maskSet(req.GetUpdateMask().GetPaths())
	should := func(path string) bool {
		if len(mask) == 0 {
			return true
		}
		return mask[path]
	}

	prefs := req.GetPreferences()
	if prefs == nil {
		return services.UpdatePreferencesInput{}, fmt.Errorf("preferences is required")
	}

	var learningGoal *string
	var quota *int32
	extra := map[string]any{}
	if should("learning_goal") && strings.TrimSpace(prefs.GetLearningGoal()) != "" {
		lg := prefs.GetLearningGoal()
		learningGoal = &lg
	}
	if should("daily_quota_minutes") && prefs.GetDailyQuotaMinutes() != nil {
		val := prefs.GetDailyQuotaMinutes().GetValue()
		quota = &val
	}
	if should("extra") && prefs.GetExtra() != nil {
		extra = prefs.GetExtra().AsMap()
	}

	var expectedVersion *int64
	if v := req.GetExpectedProfileVersion(); v != nil {
		value := v.GetValue()
		expectedVersion = &value
	}

	return services.UpdatePreferencesInput{
		UserID:          userID,
		LearningGoal:    learningGoal,
		DailyQuotaMins:  quota,
		Extra:           extra,
		ExpectedVersion: expectedVersion,
	}, nil
}

func parsePagination(pageSize int32, pageToken string) (int32, int, error) {
	limit := pageSize
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	offset := 0
	if strings.TrimSpace(pageToken) != "" {
		val, err := strconv.Atoi(pageToken)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid page_token")
		}
		if val < 0 {
			val = 0
		}
		offset = val
	}
	return limit, offset, nil
}

func parseUUID(id string) (uuid.UUID, error) {
	if strings.TrimSpace(id) == "" {
		return uuid.Nil, fmt.Errorf("empty id")
	}
	return uuid.Parse(id)
}

func parseUUIDs(ids []string) ([]uuid.UUID, error) {
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		uid, err := parseUUID(id)
		if err != nil {
			return nil, err
		}
		result = append(result, uid)
	}
	return result, nil
}

func maskSet(paths []string) map[string]bool {
	set := map[string]bool{}
	for _, p := range paths {
		set[strings.ToLower(p)] = true
	}
	return set
}

func stateToVO(state services.FavoriteState) vo.FavoriteState {
	return vo.FavoriteState{
		HasLiked:      state.HasLiked,
		HasBookmarked: state.HasBookmarked,
	}
}

func statsToVO(stats *po.ProfileVideoStats) *vo.ProfileVideoStats {
	if stats == nil {
		return nil
	}
	return &vo.ProfileVideoStats{
		LikeCount:         stats.LikeCount,
		BookmarkCount:     stats.BookmarkCount,
		UniqueWatchers:    stats.UniqueWatchers,
		TotalWatchSeconds: stats.TotalWatchSeconds,
		UpdatedAt:         stats.UpdatedAt,
	}
}

func projectionToMetadataVO(p *po.ProfileVideoProjection) *vo.ProfileVideoMetadata {
	if p == nil {
		return nil
	}
	return &vo.ProfileVideoMetadata{
		VideoID:           p.VideoID.String(),
		Title:             p.Title,
		Description:       p.Description,
		DurationMicros:    p.DurationMicros,
		ThumbnailURL:      p.ThumbnailURL,
		HLSMasterPlaylist: p.HLSMasterPlaylist,
		Status:            p.Status,
		VisibilityStatus:  p.VisibilityStatus,
		PublishedAt:       p.PublishedAt,
		Version:           p.Version,
		UpdatedAt:         p.UpdatedAt,
	}
}

func watchLogToVO(log *po.ProfileWatchLog, sessionID string) *vo.WatchProgress {
	if log == nil {
		return nil
	}
	return &vo.WatchProgress{
		PositionSeconds:   log.PositionSeconds,
		ProgressRatio:     log.ProgressRatio,
		TotalWatchSeconds: log.TotalWatchSeconds,
		FirstWatchedAt:    log.FirstWatchedAt,
		LastWatchedAt:     log.LastWatchedAt,
		ExpiresAt:         log.ExpiresAt,
		SessionID:         sessionID,
	}
}

func uniqueVideoIDs(items []*po.ProfileEngagement) []uuid.UUID {
	set := map[uuid.UUID]struct{}{}
	for _, item := range items {
		set[item.VideoID] = struct{}{}
	}
	result := make([]uuid.UUID, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	return result
}

func uniqueWatchVideoIDs(items []*po.ProfileWatchLog) []uuid.UUID {
	set := map[uuid.UUID]struct{}{}
	for _, item := range items {
		set[item.VideoID] = struct{}{}
	}
	result := make([]uuid.UUID, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	return result
}

func favoriteTypeToString(t profilev1.FavoriteType) (string, error) {
	switch t {
	case profilev1.FavoriteType_FAVORITE_TYPE_LIKE:
		return "like", nil
	case profilev1.FavoriteType_FAVORITE_TYPE_BOOKMARK:
		return "bookmark", nil
	default:
		return "", fmt.Errorf("unsupported favorite_type")
	}
}

func favoriteTypeFromString(v string) (profilev1.FavoriteType, error) {
	switch strings.ToLower(v) {
	case "like":
		return profilev1.FavoriteType_FAVORITE_TYPE_LIKE, nil
	case "bookmark":
		return profilev1.FavoriteType_FAVORITE_TYPE_BOOKMARK, nil
	default:
		return profilev1.FavoriteType_FAVORITE_TYPE_UNSPECIFIED, fmt.Errorf("unsupported favorite_type")
	}
}

func favoriteActionToEnum(a profilev1.FavoriteAction) (services.EngagementAction, error) {
	switch a {
	case profilev1.FavoriteAction_FAVORITE_ACTION_ADD:
		return services.EngagementActionAdd, nil
	case profilev1.FavoriteAction_FAVORITE_ACTION_REMOVE:
		return services.EngagementActionRemove, nil
	default:
		return "", fmt.Errorf("unsupported favorite action")
	}
}

func tsToPointer(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	value := ts.AsTime().UTC()
	return &value
}

func isStatsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no rows")
}

func stringPtr(v string) *string { return &v }

func mapProfileError(err error) error {
	switch {
	case errors.Is(err, services.ErrProfileNotFound):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, services.ErrProfileVersionConflict):
		return status.Errorf(codes.Aborted, "%v", err)
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}

func mapEngagementError(err error) error {
	switch {
	case errors.Is(err, services.ErrUnsupportedEngagementType):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}
