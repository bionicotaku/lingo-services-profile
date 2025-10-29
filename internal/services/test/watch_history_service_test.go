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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestWatchHistoryService_OutboxAndStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, cleanup := startPostgres(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	logger := log.NewStdLogger(io.Discard)
	txMgr, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: logger})
	require.NoError(t, err)

	watchRepo := repositories.NewProfileWatchLogsRepository(pool, logger)
	statsRepo := repositories.NewProfileVideoStatsRepository(pool, logger)
	outboxRepo := repositories.NewOutboxRepository(pool, logger, outboxcfg.Config{Schema: "profile"})

	svc := services.NewWatchHistoryService(watchRepo, statsRepo, outboxRepo, txMgr, logger)

	userID := uuid.New()
	videoID := uuid.New()
	firstWatched := time.Now().UTC().Add(-time.Hour)
	lastWatched := time.Now().UTC()

	_, err = svc.UpsertProgress(ctx, services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   90,
		ProgressRatio:     0.30,
		TotalWatchSeconds: 180,
		FirstWatchedAt:    &firstWatched,
		LastWatchedAt:     &lastWatched,
	})
	require.NoError(t, err)

    verifyWatchLog(ctx, t, watchRepo, userID, videoID, 0.30, 180)
    verifyStats(ctx, t, statsRepo, videoID, 1, 180)
    require.Equal(t, int64(1), countOutboxEvents(ctx, t, pool), "expected watch progressed event enqueued")

	// Progress increase below threshold: no additional event, but stats accumulate seconds delta.
	lastWatched2 := time.Now().UTC().Add(2 * time.Minute)
	_, err = svc.UpsertProgress(ctx, services.UpsertWatchProgressInput{
		UserID:            userID,
		VideoID:           videoID,
		PositionSeconds:   140,
		ProgressRatio:     0.33,
		TotalWatchSeconds: 240,
		LastWatchedAt:     &lastWatched2,
	})
	require.NoError(t, err)

    verifyWatchLog(ctx, t, watchRepo, userID, videoID, 0.33, 240)
    verifyStats(ctx, t, statsRepo, videoID, 1, 240)
    require.Equal(t, int64(1), countOutboxEvents(ctx, t, pool), "progress delta below threshold should not emit new event")
}

func verifyWatchLog(ctx context.Context, t *testing.T, repo *repositories.ProfileWatchLogsRepository, userID, videoID uuid.UUID, expectedRatio float64, expectedSeconds float64) {
	t.Helper()
	rec, err := repo.Get(ctx, nil, userID, videoID)
	require.NoError(t, err)
	require.InDelta(t, expectedRatio, rec.ProgressRatio, 1e-6)
	require.InDelta(t, expectedSeconds, rec.TotalWatchSeconds, 1e-6)
}

func verifyStats(ctx context.Context, t *testing.T, repo *repositories.ProfileVideoStatsRepository, videoID uuid.UUID, expectedWatchers, expectedSeconds int64) {
	t.Helper()
	rec, err := repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, expectedWatchers, rec.UniqueWatchers)
	require.Equal(t, expectedSeconds, rec.TotalWatchSeconds)
}

func countOutboxEvents(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var cnt int64
	err := pool.QueryRow(ctx, `select count(*) from profile.outbox_events where event_type = 'profile.watch.progressed'`).Scan(&cnt)
	require.NoError(t, err)
	return cnt
}
