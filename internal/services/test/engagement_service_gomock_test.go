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

func TestEngagementService_Mutate_StatsError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	engRepo := mocks.NewMockEngagementsRepository(ctrl)
	statsRepo := mocks.NewMockEngagementStatsRepository(ctrl)
	outbox := mocks.NewMockOutboxEnqueuer(ctrl)
	svc := services.NewEngagementService(engRepo, statsRepo, outbox, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()

	engRepo.EXPECT().Upsert(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.UpsertProfileEngagementInput{})).Return(nil)
	statsRepo.EXPECT().Increment(gomock.Any(), gomock.Any(), videoID, int64(1), int64(0), int64(0), int64(0)).Return(errors.New("stats failure"))

	err := svc.Mutate(context.Background(), services.MutateEngagementInput{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: "like",
		Action:         services.EngagementActionAdd,
	})
	require.Error(t, err)
}

func TestEngagementService_Mutate_RemoveOutboxError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	engRepo := mocks.NewMockEngagementsRepository(ctrl)
	statsRepo := mocks.NewMockEngagementStatsRepository(ctrl)
	outbox := mocks.NewMockOutboxEnqueuer(ctrl)
	svc := services.NewEngagementService(engRepo, statsRepo, outbox, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()
	deletedAt := time.Now().UTC()

	engRepo.EXPECT().SoftDelete(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.SoftDeleteProfileEngagementInput{})).Return(nil)
	statsRepo.EXPECT().Increment(gomock.Any(), gomock.Any(), videoID, int64(-1), int64(0), int64(0), int64(0)).Return(nil)
	statsRepo.EXPECT().Get(gomock.Any(), gomock.Any(), videoID).Return(&po.ProfileVideoStats{}, nil)
	outbox.EXPECT().Enqueue(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("outbox failure"))

	err := svc.Mutate(context.Background(), services.MutateEngagementInput{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: "like",
		Action:         services.EngagementActionRemove,
		OccurredAt:     ptrTime(deletedAt),
	})
	require.Error(t, err)
}

func TestEngagementService_GetFavoriteState_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	engRepo := mocks.NewMockEngagementsRepository(ctrl)
	svc := services.NewEngagementService(engRepo, nil, nil, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	videoID := uuid.New()
	gomock.InOrder(
		engRepo.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID, "like").Return(nil, repositories.ErrProfileEngagementNotFound),
		engRepo.EXPECT().Get(gomock.Any(), gomock.Any(), userID, videoID, "bookmark").Return(nil, repositories.ErrProfileEngagementNotFound),
	)

	state, err := svc.GetFavoriteState(context.Background(), userID, videoID)
	require.NoError(t, err)
	require.False(t, state.HasLiked)
	require.False(t, state.HasBookmarked)
}
