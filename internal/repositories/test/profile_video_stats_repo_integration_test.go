package repositories_test

import (
	"context"
	"io"
	"testing"

	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestProfileVideoStatsRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, terminate := startPostgres(ctx, t)
	defer terminate()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileVideoStatsRepository(pool, log.NewStdLogger(io.Discard))
	txMgr := newTxManager(t, pool)

	videoID := uuid.New()

	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Increment(txCtx, sess, videoID, 1, 1, 1, 120)
	})
	require.NoError(t, err)

	stats, err := repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.LikeCount)
	require.Equal(t, int64(1), stats.BookmarkCount)
	require.Equal(t, int64(1), stats.UniqueWatchers)
	require.Equal(t, int64(120), stats.TotalWatchSeconds)

	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Increment(txCtx, sess, videoID, 2, 0, 0, 30)
	})
	require.NoError(t, err)

	stats, err = repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.LikeCount)
	require.Equal(t, int64(1), stats.BookmarkCount)
	require.Equal(t, int64(1), stats.UniqueWatchers)
	require.Equal(t, int64(150), stats.TotalWatchSeconds)

	override := po.ProfileVideoStats{
		VideoID:           videoID,
		LikeCount:         10,
		BookmarkCount:     2,
		UniqueWatchers:    5,
		TotalWatchSeconds: 600,
		UpdatedAt:         stats.UpdatedAt,
	}
	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Set(txCtx, sess, override)
	})
	require.NoError(t, err)

	stats, err = repo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, int64(10), stats.LikeCount)
	require.Equal(t, int64(2), stats.BookmarkCount)
	require.Equal(t, int64(5), stats.UniqueWatchers)
	require.Equal(t, int64(600), stats.TotalWatchSeconds)

	list, err := repo.ListByIDs(ctx, nil, []uuid.UUID{videoID})
	require.NoError(t, err)
	require.Len(t, list, 1)
}
