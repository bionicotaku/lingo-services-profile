package controllers_test

import (
	"context"
	"testing"

	profilev1 "github.com/bionicotaku/lingo-services-profile/api/profile/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/controllers"
	"github.com/bionicotaku/lingo-services-profile/internal/models/vo"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProfileHandler_GetProfile_ProfileNotFound(t *testing.T) {
	t.Parallel()

	handler := controllers.NewProfileHandler(
		&profileServiceStub{getProfileFn: func(context.Context, uuid.UUID) (*vo.Profile, error) {
			return nil, services.ErrProfileNotFound
		}},
		&engagementServiceStub{},
		&watchHistoryServiceStub{},
		&videoProjectionServiceStub{},
		&videoStatsServiceStub{},
		controllers.NewBaseHandler(controllers.HandlerTimeouts{}),
	)

	ctx := metadataContextWithUser(t, uuid.New())
	_, err := handler.GetProfile(ctx, &profilev1.GetProfileRequest{})
	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
