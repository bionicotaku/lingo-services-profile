package services_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/bionicotaku/lingo-services-profile/internal/services/mocks"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProfileService_UpdateProfile_VersionConflictWithoutGet(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockProfileUsersRepository(ctrl)
	repo.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&po.ProfileUser{ProfileVersion: 1, DisplayName: "Alice"}, nil)

	svc := services.NewProfileService(repo, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	_, err := svc.UpdateProfile(context.Background(), services.UpdateProfileInput{
		UserID:          userID,
		DisplayName:     ptrString("Alice"),
		ExpectedVersion: ptrInt64(2),
	})
	require.ErrorIs(t, err, services.ErrProfileVersionConflict)
}

func TestProfileService_UpdateProfile_PropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockProfileUsersRepository(ctrl)
	repo.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, repositories.ErrProfileUserNotFound)
	repo.EXPECT().Upsert(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(repositories.UpsertProfileUserInput{})).Return(nil, errors.New("db err"))

	svc := services.NewProfileService(repo, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	_, err := svc.UpdateProfile(context.Background(), services.UpdateProfileInput{
		UserID:      uuid.New(),
		DisplayName: ptrString("Alice"),
	})
	require.Error(t, err)
}

func TestProfileService_UpdatePreferences_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockProfileUsersRepository(ctrl)
	repo.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, repositories.ErrProfileUserNotFound)

	svc := services.NewProfileService(repo, &fakeTxManager{}, log.NewStdLogger(io.Discard))

	_, err := svc.UpdatePreferences(context.Background(), services.UpdatePreferencesInput{
		UserID:       uuid.New(),
		LearningGoal: ptrString("fluency"),
	})
	require.ErrorIs(t, err, services.ErrProfileNotFound)
}
