package controllers_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	profilev1 "github.com/bionicotaku/lingo-services-profile/api/profile/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/controllers"
	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/models/vo"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type profileServiceStub struct {
	getProfileFn        func(context.Context, uuid.UUID) (*vo.Profile, error)
	updateProfileFn     func(context.Context, services.UpdateProfileInput) (*vo.Profile, error)
	updatePreferencesFn func(context.Context, services.UpdatePreferencesInput) (*vo.Profile, error)
}

func (s *profileServiceStub) GetProfile(ctx context.Context, userID uuid.UUID) (*vo.Profile, error) {
	if s.getProfileFn != nil {
		return s.getProfileFn(ctx, userID)
	}
	return nil, nil
}

func (s *profileServiceStub) UpdateProfile(ctx context.Context, input services.UpdateProfileInput) (*vo.Profile, error) {
	if s.updateProfileFn != nil {
		return s.updateProfileFn(ctx, input)
	}
	return nil, nil
}

func (s *profileServiceStub) UpdatePreferences(ctx context.Context, input services.UpdatePreferencesInput) (*vo.Profile, error) {
	if s.updatePreferencesFn != nil {
		return s.updatePreferencesFn(ctx, input)
	}
	return nil, nil
}

type engagementServiceStub struct {
	mutateFn        func(context.Context, services.MutateEngagementInput) error
	getStateFn      func(context.Context, uuid.UUID, uuid.UUID) (services.FavoriteState, error)
	listFavoritesFn func(context.Context, services.ListFavoritesInput) ([]*po.ProfileEngagement, error)
	lastMutateInput services.MutateEngagementInput
}

func (s *engagementServiceStub) Mutate(ctx context.Context, input services.MutateEngagementInput) error {
	s.lastMutateInput = input
	if s.mutateFn != nil {
		return s.mutateFn(ctx, input)
	}
	return nil
}

func (s *engagementServiceStub) GetFavoriteState(ctx context.Context, userID, videoID uuid.UUID) (services.FavoriteState, error) {
	if s.getStateFn != nil {
		return s.getStateFn(ctx, userID, videoID)
	}
	return services.FavoriteState{}, nil
}

func (s *engagementServiceStub) ListFavorites(ctx context.Context, input services.ListFavoritesInput) ([]*po.ProfileEngagement, error) {
	if s.listFavoritesFn != nil {
		return s.listFavoritesFn(ctx, input)
	}
	return nil, nil
}

type watchHistoryServiceStub struct {
	upsertFn func(context.Context, services.UpsertWatchProgressInput) (*po.ProfileWatchLog, error)
	listFn   func(context.Context, services.ListWatchHistoryInput) ([]*po.ProfileWatchLog, error)
}

func (s *watchHistoryServiceStub) UpsertProgress(ctx context.Context, input services.UpsertWatchProgressInput) (*po.ProfileWatchLog, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, input)
	}
	return nil, nil
}

func (s *watchHistoryServiceStub) ListWatchHistory(ctx context.Context, input services.ListWatchHistoryInput) ([]*po.ProfileWatchLog, error) {
	if s.listFn != nil {
		return s.listFn(ctx, input)
	}
	return nil, nil
}

type videoProjectionServiceStub struct {
	listFn func(context.Context, []uuid.UUID) ([]*po.ProfileVideoProjection, error)
}

func (s *videoProjectionServiceStub) ListProjections(ctx context.Context, ids []uuid.UUID) ([]*po.ProfileVideoProjection, error) {
	if s.listFn != nil {
		return s.listFn(ctx, ids)
	}
	return nil, nil
}

type videoStatsServiceStub struct {
	getFn  func(context.Context, uuid.UUID) (*po.ProfileVideoStats, error)
	listFn func(context.Context, []uuid.UUID) ([]*po.ProfileVideoStats, error)
}

func (s *videoStatsServiceStub) GetStats(ctx context.Context, id uuid.UUID) (*po.ProfileVideoStats, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, nil
}

func (s *videoStatsServiceStub) ListStats(ctx context.Context, ids []uuid.UUID) ([]*po.ProfileVideoStats, error) {
	if s.listFn != nil {
		return s.listFn(ctx, ids)
	}
	return nil, nil
}

func metadataContextWithUser(t *testing.T, userID uuid.UUID) context.Context {
	t.Helper()
	claims := []byte(`{"sub":"` + userID.String() + `"}`)
	encoded := base64.RawURLEncoding.EncodeToString(claims)
	md := metadata.New(map[string]string{"x-apigateway-api-userinfo": encoded})
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestProfileHandler_GetProfile_UsesMetadataUser(t *testing.T) {
	t.Parallel()

	expectedID := uuid.New()
	called := false
	profile := &vo.Profile{UserID: expectedID.String(), DisplayName: "Alice"}
	handler := controllers.NewProfileHandler(
		&profileServiceStub{getProfileFn: func(_ context.Context, userID uuid.UUID) (*vo.Profile, error) {
			require.Equal(t, expectedID, userID)
			called = true
			return profile, nil
		}},
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, expectedID)
	resp, err := handler.GetProfile(ctx, &profilev1.GetProfileRequest{})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "Alice", resp.GetProfile().GetDisplayName())
	require.Equal(t, expectedID.String(), resp.GetProfile().GetUserId())
}

func TestProfileHandler_MutateFavorite_MapsInput(t *testing.T) {
	t.Parallel()

	expectedUser := uuid.New()
	expectedVideo := uuid.New()
	engagement := &engagementServiceStub{
		mutateFn: func(_ context.Context, input services.MutateEngagementInput) error {
			require.Equal(t, expectedUser, input.UserID)
			require.Equal(t, expectedVideo, input.VideoID)
			require.Equal(t, "bookmark", input.EngagementType)
			require.Equal(t, services.EngagementActionAdd, input.Action)
			return nil
		},
		getStateFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (services.FavoriteState, error) {
			return services.FavoriteState{HasBookmarked: true}, nil
		},
	}
	statsSvc := &videoStatsServiceStub{
		getFn: func(_ context.Context, id uuid.UUID) (*po.ProfileVideoStats, error) {
			require.Equal(t, expectedVideo, id)
			return &po.ProfileVideoStats{BookmarkCount: 3, UpdatedAt: time.Now()}, nil
		},
	}

	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		engagement,
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		statsSvc,
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, expectedUser)
	req := &profilev1.MutateFavoriteRequest{
		VideoId:      expectedVideo.String(),
		FavoriteType: profilev1.FavoriteType_FAVORITE_TYPE_BOOKMARK,
		Action:       profilev1.FavoriteAction_FAVORITE_ACTION_ADD,
	}
	resp, err := handler.MutateFavorite(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.GetState().GetHasBookmarked())
	require.EqualValues(t, 3, resp.GetStats().GetBookmarkCount())
}

func TestProfileHandler_UpdateProfile_ConflictMapsToAborted(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	profiles := &profileServiceStub{
		updateProfileFn: func(context.Context, services.UpdateProfileInput) (*vo.Profile, error) {
			return nil, services.ErrProfileVersionConflict
		},
	}
	handler := controllers.NewProfileHandler(
		profiles,
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, userID)
	_, err := handler.UpdateProfile(ctx, &profilev1.UpdateProfileRequest{
		Profile:    &profilev1.Profile{DisplayName: "Alice"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	require.Equal(t, codes.Aborted, st.Code())
}

func TestProfileHandler_MutateFavorite_UnsupportedTypeReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	videoID := uuid.New()
	engSvc := &engagementServiceStub{
		mutateFn: func(context.Context, services.MutateEngagementInput) error {
			return services.ErrUnsupportedEngagementType
		},
	}
	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		engSvc,
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, userID)
	_, err := handler.MutateFavorite(ctx, &profilev1.MutateFavoriteRequest{
		VideoId:      videoID.String(),
		FavoriteType: profilev1.FavoriteType_FAVORITE_TYPE_LIKE,
		Action:       profilev1.FavoriteAction_FAVORITE_ACTION_ADD,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestProfileHandler_GetProfile_MissingUserID(t *testing.T) {
	t.Parallel()

	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	_, err := handler.GetProfile(context.Background(), &profilev1.GetProfileRequest{})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestProfileHandler_MutateFavorite_InvalidVideoID(t *testing.T) {
	t.Parallel()

	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, uuid.New())
	_, err := handler.MutateFavorite(ctx, &profilev1.MutateFavoriteRequest{VideoId: "not-a-uuid"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestProfileHandler_MutateFavorite_UnsupportedType(t *testing.T) {
	t.Parallel()

	engagement := &engagementServiceStub{
		mutateFn: func(context.Context, services.MutateEngagementInput) error {
			return services.ErrUnsupportedEngagementType
		},
	}
	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		engagement,
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, uuid.New())
	req := &profilev1.MutateFavoriteRequest{
		VideoId:      uuid.NewString(),
		FavoriteType: profilev1.FavoriteType_FAVORITE_TYPE_BOOKMARK,
		Action:       profilev1.FavoriteAction_FAVORITE_ACTION_ADD,
	}
	_, err := handler.MutateFavorite(ctx, req)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestProfileHandler_ListFavorites_InvalidPageToken(t *testing.T) {
	t.Parallel()

	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, uuid.New())
	_, err := handler.ListFavorites(ctx, &profilev1.ListFavoritesRequest{PageToken: "abc"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestProfileHandler_ListFavorites_MissingUserMetadata(t *testing.T) {
	t.Parallel()

	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	_, err := handler.ListFavorites(context.Background(), &profilev1.ListFavoritesRequest{})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestProfileHandler_ListFavorites_PaginationAndStats(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now().UTC()
	videoIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	engagements := &engagementServiceStub{
		listFavoritesFn: func(_ context.Context, input services.ListFavoritesInput) ([]*po.ProfileEngagement, error) {
			require.Equal(t, userID, input.UserID)
			require.Equal(t, int32(3), input.Limit)
			require.Equal(t, int32(0), input.Offset)
			return []*po.ProfileEngagement{
				{UserID: userID, VideoID: videoIDs[0], EngagementType: "like", CreatedAt: now, UpdatedAt: now},
				{UserID: userID, VideoID: videoIDs[1], EngagementType: "bookmark", CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
				{UserID: userID, VideoID: videoIDs[2], EngagementType: "bookmark", CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(2 * time.Minute)},
			}, nil
		},
	}

	projections := &videoProjectionServiceStub{
		listFn: func(_ context.Context, ids []uuid.UUID) ([]*po.ProfileVideoProjection, error) {
			require.ElementsMatch(t, videoIDs[:2], ids)
			return []*po.ProfileVideoProjection{
				{VideoID: videoIDs[0], Title: "Lesson 1", UpdatedAt: now},
				{VideoID: videoIDs[1], Title: "Lesson 2", UpdatedAt: now.Add(30 * time.Second)},
			}, nil
		},
	}

	statsCalled := false
	stats := &videoStatsServiceStub{
		listFn: func(_ context.Context, ids []uuid.UUID) ([]*po.ProfileVideoStats, error) {
			statsCalled = true
			require.ElementsMatch(t, videoIDs[:2], ids)
			return []*po.ProfileVideoStats{
				{VideoID: videoIDs[0], LikeCount: 3, BookmarkCount: 1, UpdatedAt: now},
				{VideoID: videoIDs[1], LikeCount: 1, BookmarkCount: 5, UpdatedAt: now},
			}, nil
		},
	}

	handler := controllers.NewProfileHandler(
		&profileServiceStub{},
		engagements,
		&watchHistoryServiceStub{},
		projections,
		stats,
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, userID)
	resp, err := handler.ListFavorites(ctx, &profilev1.ListFavoritesRequest{PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, "2", resp.GetNextPageToken())
	require.Len(t, resp.GetFavorites(), 2)
	require.True(t, statsCalled)

	first := resp.GetFavorites()[0]
	require.Equal(t, videoIDs[0].String(), first.GetVideoId())
	require.True(t, first.GetState().GetHasLiked())
	require.False(t, first.GetState().GetHasBookmarked())
	require.Equal(t, "Lesson 1", first.GetVideo().GetTitle())

	second := resp.GetFavorites()[1]
	require.Equal(t, videoIDs[1].String(), second.GetVideoId())
	require.False(t, second.GetState().GetHasLiked())
	require.True(t, second.GetState().GetHasBookmarked())
	require.Equal(t, "Lesson 2", second.GetVideo().GetTitle())
}
