package services_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/services"
	outboxcfg "github.com/bionicotaku/lingo-utils/outbox/config"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestEngagementService_MutateLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, cleanup := startPostgres(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	logger := log.NewStdLogger(io.Discard)
	engRepo := repositories.NewProfileEngagementsRepository(pool, logger)
	statsRepo := repositories.NewProfileVideoStatsRepository(pool, logger)
	outboxRepo := repositories.NewOutboxRepository(pool, logger, outboxcfg.Config{Schema: "profile"})
	txMgr, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: logger})
	require.NoError(t, err)

	svc := services.NewEngagementService(engRepo, statsRepo, outboxRepo, txMgr, logger)

	userID := uuid.New()
	videoID := uuid.New()

	err = svc.Mutate(ctx, services.MutateEngagementInput{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: "like",
		Action:         services.EngagementActionAdd,
	})
	require.NoError(t, err)

    verifyEngagementExists(ctx, t, pool, userID, videoID, true)
    verifyVideoStats(ctx, t, pool, videoID, 1, 0, 0, 0)
    verifyOutboxCount(ctx, t, pool, "profile.engagement.added", 1)

	err = svc.Mutate(ctx, services.MutateEngagementInput{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: "like",
		Action:         services.EngagementActionRemove,
	})
	require.NoError(t, err)

    verifyEngagementExists(ctx, t, pool, userID, videoID, false)
    verifyVideoStats(ctx, t, pool, videoID, 0, 0, 0, 0)
    verifyOutboxCount(ctx, t, pool, "profile.engagement.removed", 2)

	err = svc.Mutate(ctx, services.MutateEngagementInput{
		UserID:         userID,
		VideoID:        videoID,
		EngagementType: "unsupported",
		Action:         services.EngagementActionAdd,
	})
	require.ErrorIs(t, err, services.ErrUnsupportedEngagementType)
}

func verifyEngagementExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, videoID uuid.UUID, expectActive bool) {
	t.Helper()
	var deletedAt *time.Time
	err := pool.QueryRow(ctx, `select deleted_at from profile.engagements where user_id = $1 and video_id = $2 and engagement_type = 'like'`, userID, videoID).Scan(&deletedAt)
	if expectActive {
		require.NoError(t, err)
		require.Nil(t, deletedAt)
		return
	}
	if err != nil {
		require.ErrorIs(t, err, pgx.ErrNoRows)
		return
	}
	require.NotNil(t, deletedAt)
}

func verifyVideoStats(ctx context.Context, t *testing.T, pool *pgxpool.Pool, videoID uuid.UUID, like, bookmark, watchers, seconds int64) {
	t.Helper()
	var likeCount, bookmarkCount, uniqueWatchers, totalWatchSeconds int64
	err := pool.QueryRow(ctx, `select like_count, bookmark_count, unique_watchers, total_watch_seconds from profile.video_stats where video_id = $1`, videoID).
		Scan(&likeCount, &bookmarkCount, &uniqueWatchers, &totalWatchSeconds)
	require.NoError(t, err)
	require.Equal(t, like, likeCount)
	require.Equal(t, bookmark, bookmarkCount)
	require.Equal(t, watchers, uniqueWatchers)
	require.Equal(t, seconds, totalWatchSeconds)
}

func verifyOutboxCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool, eventType string, expected int64) {
	t.Helper()
	var count int64
	err := pool.QueryRow(ctx, `select count(*) from profile.outbox_events where event_type = $1`, eventType).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, expected, count)
}
