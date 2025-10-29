package repositories_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestProfileVideoProjectionRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, terminate := startPostgres(ctx, t)
	defer terminate()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileVideoProjectionRepository(pool, log.NewStdLogger(io.Discard))

	videoID := uuid.New()
	publishedAt := time.Now().Add(-time.Hour).UTC()

	input := repositories.UpsertVideoProjectionInput{
		VideoID:           videoID,
		Title:             "Title",
		Description:       stringPtr("Description"),
		DurationMicros:    int64Ptr(120_000_000),
		ThumbnailURL:      stringPtr("https://example.com/thumb.jpg"),
		HLSMasterPlaylist: stringPtr("https://example.com/master.m3u8"),
		Status:            stringPtr("ready"),
		VisibilityStatus:  stringPtr("public"),
		PublishedAt:       &publishedAt,
		Version:           3,
		UpdatedAt:         timePtr(time.Now().UTC()),
	}

	require.NoError(t, repo.Upsert(ctx, nil, input))

	record, err := repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, "Title", record.Title)
	require.NotNil(t, record.Description)
	require.Equal(t, int64(120_000_000), derefInt64(record.DurationMicros))
	require.Equal(t, int64(3), record.Version)

	input.Title = "Updated"
	input.Description = stringPtr("Updated Desc")
	input.Version = 4
	require.NoError(t, repo.Upsert(ctx, nil, input))

	record, err = repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, "Updated", record.Title)
	require.Equal(t, int64(4), record.Version)
	require.Equal(t, "Updated Desc", derefString(record.Description))

	list, err := repo.ListByIDs(ctx, nil, []uuid.UUID{videoID})
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func int64Ptr(v int64) *int64 {
	return &v
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
