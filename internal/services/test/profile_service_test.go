package services_test

import (
	"context"
	"io"
	"testing"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestProfileService_UpdateProfile_CreatesAndUpdates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, cleanup := startPostgres(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileUsersRepository(pool, log.NewStdLogger(io.Discard))
	txMgr, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: log.NewStdLogger(io.Discard)})
	require.NoError(t, err)

	svc := services.NewProfileService(repo, txMgr, log.NewStdLogger(io.Discard))

	userID := uuid.New()

	profile, err := svc.UpdateProfile(ctx, services.UpdateProfileInput{
		UserID:          userID,
		DisplayName:     stringPtr("Alice"),
		AvatarURL:       stringPtr("https://cdn/avatar.png"),
		ExpectedVersion: nil,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), profile.ProfileVersion)
	require.Equal(t, "Alice", profile.DisplayName)

	profile, err = svc.UpdateProfile(ctx, services.UpdateProfileInput{
		UserID:          userID,
		DisplayName:     stringPtr("Alice Prime"),
		ExpectedVersion: int64Ptr(1),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), profile.ProfileVersion)
	require.Equal(t, "Alice Prime", profile.DisplayName)
}

func TestProfileService_UpdateProfile_VersionConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, cleanup := startPostgres(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileUsersRepository(pool, log.NewStdLogger(io.Discard))
	txMgr, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: log.NewStdLogger(io.Discard)})
	require.NoError(t, err)

	svc := services.NewProfileService(repo, txMgr, log.NewStdLogger(io.Discard))

	userID := uuid.New()
	_, err = svc.UpdateProfile(ctx, services.UpdateProfileInput{UserID: userID, DisplayName: stringPtr("Alice")})
	require.NoError(t, err)

	_, err = svc.UpdateProfile(ctx, services.UpdateProfileInput{UserID: userID, DisplayName: stringPtr("Alice"), ExpectedVersion: int64Ptr(5)})
	require.ErrorIs(t, err, services.ErrProfileVersionConflict)
}

func TestProfileService_UpdatePreferences_ProfileNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, cleanup := startPostgres(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileUsersRepository(pool, log.NewStdLogger(io.Discard))
	txMgr, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: log.NewStdLogger(io.Discard)})
	require.NoError(t, err)

	svc := services.NewProfileService(repo, txMgr, log.NewStdLogger(io.Discard))

	_, err = svc.UpdatePreferences(ctx, services.UpdatePreferencesInput{UserID: uuid.New(), LearningGoal: stringPtr("fluency")})
	require.ErrorIs(t, err, services.ErrProfileNotFound)
}

func stringPtr(v string) *string { return &v }
func int64Ptr(v int64) *int64    { return &v }
