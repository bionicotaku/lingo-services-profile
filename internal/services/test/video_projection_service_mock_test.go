package services_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/bionicotaku/lingo-services-profile/internal/services/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVideoProjectionService_UpsertProjection_PropagatesError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoProjectionRepository(ctrl)
	svc := services.NewVideoProjectionService(repo, log.NewStdLogger(io.Discard))

	videoID := uuid.New()
	expectedErr := errors.New("db failure")
	repo.EXPECT().Upsert(gomock.Any(), gomock.Nil(), gomock.Any()).Return(expectedErr)

	err := svc.UpsertProjection(context.Background(), repositories.UpsertVideoProjectionInput{VideoID: videoID, Title: "title"})
	require.ErrorIs(t, err, expectedErr)
}

func TestVideoProjectionService_ListProjections_UsesRepository(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockVideoProjectionRepository(ctrl)
	svc := services.NewVideoProjectionService(repo, log.NewStdLogger(io.Discard))

	videoID := uuid.New()
	repo.EXPECT().ListByIDs(gomock.Any(), gomock.Nil(), []uuid.UUID{videoID}).Return(nil, errors.New("boom"))

	_, err := svc.ListProjections(context.Background(), []uuid.UUID{videoID})
	require.Error(t, err)
}
