package services_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/bionicotaku/lingo-services-profile/internal/services/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVideoStatsService_GetStats_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoStatsRepository(ctrl)
	svc := services.NewVideoStatsService(repo, log.NewStdLogger(io.Discard))

	videoID := uuid.New()
	expected := &po.ProfileVideoStats{VideoID: videoID, LikeCount: 2}

	repo.EXPECT().Get(gomock.Any(), gomock.Nil(), videoID).Return(expected, nil)

	stats, err := svc.GetStats(context.Background(), videoID)
	require.NoError(t, err)
	require.Equal(t, expected, stats)
}

func TestVideoStatsService_GetStats_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoStatsRepository(ctrl)
	svc := services.NewVideoStatsService(repo, log.NewStdLogger(io.Discard))

	videoID := uuid.New()
	repo.EXPECT().Get(gomock.Any(), gomock.Nil(), videoID).Return(nil, errors.New("boom"))

	_, err := svc.GetStats(context.Background(), videoID)
	require.Error(t, err)
}

func TestVideoStatsService_GetStats_MissingVideoID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoStatsRepository(ctrl)
	svc := services.NewVideoStatsService(repo, log.NewStdLogger(io.Discard))

	_, err := svc.GetStats(context.Background(), uuid.Nil)
	require.Error(t, err)
}

func TestVideoStatsService_ListStats_EmptyInput(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoStatsRepository(ctrl)
	svc := services.NewVideoStatsService(repo, log.NewStdLogger(io.Discard))

	stats, err := svc.ListStats(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, stats)
}

func TestVideoStatsService_ListStats_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoStatsRepository(ctrl)
	svc := services.NewVideoStatsService(repo, log.NewStdLogger(io.Discard))

	ids := []uuid.UUID{uuid.New()}
	repo.EXPECT().ListByIDs(gomock.Any(), gomock.Nil(), ids).Return(nil, errors.New("query failed"))

	_, err := svc.ListStats(context.Background(), ids)
	require.Error(t, err)
}

func TestVideoStatsService_ListStats_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoStatsRepository(ctrl)
	svc := services.NewVideoStatsService(repo, log.NewStdLogger(io.Discard))

	ids := []uuid.UUID{uuid.New(), uuid.New()}
	expected := []*po.ProfileVideoStats{
		{VideoID: ids[0], LikeCount: 1},
		{VideoID: ids[1], BookmarkCount: 2},
	}

	repo.EXPECT().ListByIDs(gomock.Any(), gomock.Nil(), ids).Return(expected, nil)

	stats, err := svc.ListStats(context.Background(), ids)
	require.NoError(t, err)
	require.Equal(t, expected, stats)
}
