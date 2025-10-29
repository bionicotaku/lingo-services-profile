package repositories_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestProfileWatchLogsRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, terminate := startPostgres(ctx, t)
	defer terminate()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileWatchLogsRepository(pool, log.NewStdLogger(io.Discard))
	txMgr := newTxManager(t, pool)

	userID := uuid.New()
	videoID := uuid.New()
	firstWatched := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	lastWatched := time.Now().UTC().Truncate(time.Second)
	expires := lastWatched.Add(24 * time.Hour)

	input := repositories.UpsertWatchLogInput{
		UserID:              userID,
		VideoID:             videoID,
		PositionSeconds:     120.5,
		ProgressRatio:       0.6,
		TotalWatchSeconds:   300,
		FirstWatchedAt:      &firstWatched,
		LastWatchedAt:       &lastWatched,
		ExpiresAt:           &expires,
		IncrementWatchDelta: 300,
	}

	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Upsert(txCtx, sess, input)
	})
	require.NoError(t, err)

	record, err := repo.Get(ctx, nil, userID, videoID)
	require.NoError(t, err)
	require.InDelta(t, 120.5, record.PositionSeconds, 0.001)
	require.InDelta(t, 0.6, record.ProgressRatio, 0.0001)
	require.InDelta(t, 300, record.TotalWatchSeconds, 0.001)
	require.Nil(t, record.RedactedAt)

	// 增量更新
	newLast := lastWatched.Add(5 * time.Minute)
	inputIncrement := repositories.UpsertWatchLogInput{
		UserID:              userID,
		VideoID:             videoID,
		PositionSeconds:     200,
		ProgressRatio:       0.85,
		TotalWatchSeconds:   0,
		LastWatchedAt:       &newLast,
		IncrementWatchDelta: 60,
	}
	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Upsert(txCtx, sess, inputIncrement)
	})
	require.NoError(t, err)

	record, err = repo.Get(ctx, nil, userID, videoID)
	require.NoError(t, err)
	require.InDelta(t, 200, record.PositionSeconds, 0.001)
	require.InDelta(t, 0.85, record.ProgressRatio, 0.0001)
	require.InDelta(t, 360, record.TotalWatchSeconds, 0.001)

	redactedAt := time.Now().UTC()
	redactInput := repositories.UpsertWatchLogInput{
		UserID:              userID,
		VideoID:             videoID,
		PositionSeconds:     200,
		ProgressRatio:       0.85,
		TotalWatchSeconds:   0,
		RedactedAt:          &redactedAt,
		IncrementWatchDelta: 0,
	}
	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Upsert(txCtx, sess, redactInput)
	})
	require.NoError(t, err)

	record, err = repo.Get(ctx, nil, userID, videoID)
	require.NoError(t, err)
	require.NotNil(t, record.RedactedAt)

	list, err := repo.ListByUser(ctx, nil, userID, false, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 0)

	list, err = repo.ListByUser(ctx, nil, userID, true, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
}
