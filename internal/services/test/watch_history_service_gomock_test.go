package services_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/bionicotaku/lingo-services-profile/internal/services/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWatchHistoryService_UpsertProgress_EmitsEvent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logs := mocks.NewMockWatchLogsRepository(ctrl)
	stats := mocks.NewMockWatchStatsRepository(ctrl)
	outbox := mocks.NewMockOutboxEnqueuer(ctrl)
	svc := services.NewWatchHistoryService(logs, stats, outbox, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()
	firstWatched := time.Now().UTC().Add(-time.Hour)
	lastWatched := time.Now().UTC()

	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(nil, repositories.ErrProfileWatchLogNotFound)
	logs.EXPECT().Upsert(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.UpsertWatchLogInput{})).Return(nil)
	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(&po.ProfileWatchLog{
		UserID:            userID,
		VideoID:           videoID,
		ProgressRatio:     0.50,
		TotalWatchSeconds: 180,
		FirstWatchedAt:    firstWatched,
		LastWatchedAt:     lastWatched,
	}, nil)
	stats.EXPECT().Increment(gomock.Any(), gomock.Any(), videoID, int64(0), int64(0), int64(1), int64(180)).Return(nil)
	outbox.EXPECT().Enqueue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	result, err := svc.UpsertProgress(context.Background(), services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   90,
		ProgressRatio:     0.50,
		TotalWatchSeconds: 180,
		FirstWatchedAt:    ptrTime(firstWatched),
		LastWatchedAt:     ptrTime(lastWatched),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestWatchHistoryService_UpsertProgress_StatsError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logs := mocks.NewMockWatchLogsRepository(ctrl)
	stats := mocks.NewMockWatchStatsRepository(ctrl)
	outbox := mocks.NewMockOutboxEnqueuer(ctrl)
	svc := services.NewWatchHistoryService(logs, stats, outbox, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()

	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(nil, repositories.ErrProfileWatchLogNotFound)
	logs.EXPECT().Upsert(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.UpsertWatchLogInput{})).Return(nil)
	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(&po.ProfileWatchLog{
		UserID:            userID,
		VideoID:           videoID,
		ProgressRatio:     0.60,
		TotalWatchSeconds: 240,
		FirstWatchedAt:    time.Now().UTC(),
		LastWatchedAt:     time.Now().UTC(),
	}, nil)
	stats.EXPECT().Increment(gomock.Any(), gomock.Any(), videoID, int64(0), int64(0), int64(1), int64(240)).Return(errors.New("stats failure"))

	_, err := svc.UpsertProgress(context.Background(), services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   120,
		ProgressRatio:     0.60,
		TotalWatchSeconds: 240,
	})
	require.Error(t, err)
}

func TestWatchHistoryService_UpsertProgress_NoEventWhenDeltaBelowThreshold(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logs := mocks.NewMockWatchLogsRepository(ctrl)
	stats := mocks.NewMockWatchStatsRepository(ctrl)
	outbox := mocks.NewMockOutboxEnqueuer(ctrl)
	svc := services.NewWatchHistoryService(logs, stats, outbox, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()
	existing := &po.ProfileWatchLog{
		UserID:            userID,
		VideoID:           videoID,
		ProgressRatio:     0.30,
		TotalWatchSeconds: 180,
		FirstWatchedAt:    time.Now().UTC().Add(-time.Hour),
		LastWatchedAt:     time.Now().UTC().Add(-time.Minute),
	}
	updated := &po.ProfileWatchLog{
		UserID:            userID,
		VideoID:           videoID,
		ProgressRatio:     0.33,
		TotalWatchSeconds: 200,
		FirstWatchedAt:    existing.FirstWatchedAt,
		LastWatchedAt:     time.Now().UTC(),
	}

	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(existing, nil)
	logs.EXPECT().Upsert(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.UpsertWatchLogInput{})).Return(nil)
	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(updated, nil)
	stats.EXPECT().Increment(gomock.Any(), gomock.Any(), videoID, int64(0), int64(0), int64(0), int64(20)).Return(nil)
	// No Outbox enqueue expected

	_, err := svc.UpsertProgress(context.Background(), services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   150,
		ProgressRatio:     0.33,
		TotalWatchSeconds: 200,
	})
	require.NoError(t, err)
}

func TestWatchHistoryService_UpsertProgress_OutboxError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logs := mocks.NewMockWatchLogsRepository(ctrl)
	stats := mocks.NewMockWatchStatsRepository(ctrl)
	outbox := mocks.NewMockOutboxEnqueuer(ctrl)
	svc := services.NewWatchHistoryService(logs, stats, outbox, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()
	existing := &po.ProfileWatchLog{
		UserID:            userID,
		VideoID:           videoID,
		ProgressRatio:     0.60,
		TotalWatchSeconds: 120,
		FirstWatchedAt:    time.Now().UTC().Add(-time.Hour),
		LastWatchedAt:     time.Now().UTC().Add(-time.Minute),
	}
	updated := &po.ProfileWatchLog{
		UserID:            userID,
		VideoID:           videoID,
		ProgressRatio:     0.80,
		TotalWatchSeconds: 180,
		FirstWatchedAt:    existing.FirstWatchedAt,
		LastWatchedAt:     time.Now().UTC(),
	}

	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(existing, nil)
	logs.EXPECT().Upsert(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.UpsertWatchLogInput{})).Return(nil)
	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(updated, nil)
	stats.EXPECT().Increment(gomock.Any(), gomock.Any(), videoID, int64(0), int64(0), int64(0), int64(60)).Return(nil)
	outbox.EXPECT().Enqueue(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("outbox failure"))

	_, err := svc.UpsertProgress(context.Background(), services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   160,
		ProgressRatio:     0.80,
		TotalWatchSeconds: 180,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "outbox failure")
}

func TestWatchHistoryService_UpsertProgress_GetError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logs := mocks.NewMockWatchLogsRepository(ctrl)
	svc := services.NewWatchHistoryService(logs, nil, nil, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()
	logs.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID).Return(nil, errors.New("db error"))

	_, err := svc.UpsertProgress(context.Background(), services.UpsertWatchProgressInput{UserID: userID, VideoID: videoID})
	require.Error(t, err)
}
