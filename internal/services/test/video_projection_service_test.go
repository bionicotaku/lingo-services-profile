package services_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestVideoProjectionService_UpsertAndList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, cleanup := startPostgres(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileVideoProjectionRepository(pool, log.NewStdLogger(io.Discard))
	svc := services.NewVideoProjectionService(repo, log.NewStdLogger(io.Discard))

	videoID := uuid.New()
	publishedAt := time.Now().UTC()
	updatedAt := time.Now().UTC()

	err = svc.UpsertProjection(ctx, repositories.UpsertVideoProjectionInput{
		VideoID:           videoID,
		Title:             "Sample Title",
		Description:       stringPtr("Sample Desc"),
		DurationMicros:    int64Ptr(180_000_000),
		ThumbnailURL:      stringPtr("https://cdn/thumb.jpg"),
		HLSMasterPlaylist: stringPtr("https://cdn/master.m3u8"),
		Status:            stringPtr("published"),
		VisibilityStatus:  stringPtr("public"),
		PublishedAt:       &publishedAt,
		Version:           1,
		UpdatedAt:         &updatedAt,
	})
	require.NoError(t, err)

	projections, err := svc.ListProjections(ctx, []uuid.UUID{videoID})
	require.NoError(t, err)
	require.Len(t, projections, 1)
	require.Equal(t, "Sample Title", projections[0].Title)
	require.Equal(t, int64(1), projections[0].Version)

	// second upsert with higher version should overwrite fields
	updatedAt2 := time.Now().Add(time.Minute).UTC()
	err = svc.UpsertProjection(ctx, repositories.UpsertVideoProjectionInput{
		VideoID:        videoID,
		Title:          "Updated Title",
		DurationMicros: int64Ptr(200_000_000),
		Version:        2,
		UpdatedAt:      &updatedAt2,
	})
	require.NoError(t, err)

	record, err := repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, "Updated Title", record.Title)
	require.Equal(t, int64(2), record.Version)
	require.Equal(t, int64(200_000_000), derefInt64(record.DurationMicros))
}

func TestVideoProjectionService_UpsertRequiresVideoID(t *testing.T) {
	t.Parallel()

	svc := services.NewVideoProjectionService(nil, log.NewStdLogger(io.Discard))
	ctx := context.Background()

	err := svc.UpsertProjection(ctx, repositories.UpsertVideoProjectionInput{})
	require.Error(t, err)
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
