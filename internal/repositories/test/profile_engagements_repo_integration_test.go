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

func TestProfileEngagementsRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, terminate := startPostgres(ctx, t)
	defer terminate()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileEngagementsRepository(pool, log.NewStdLogger(io.Discard))
	txMgr := newTxManager(t, pool)

	userID := uuid.New()
	videoID := uuid.New()
	occurred := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)

	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Upsert(txCtx, sess, repositories.UpsertProfileEngagementInput{
			UserID:         userID,
			VideoID:        videoID,
			EngagementType: "like",
			OccurredAt:     &occurred,
		})
	})
	require.NoError(t, err)

	record, err := repo.Get(ctx, nil, userID, videoID, "like")
	require.NoError(t, err)
	require.Equal(t, "like", record.EngagementType)
	require.Nil(t, record.DeletedAt)

	list, err := repo.ListByUser(ctx, nil, userID, nil, false, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)

	deletedAt := time.Now().UTC()
	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.SoftDelete(txCtx, sess, repositories.SoftDeleteProfileEngagementInput{
			UserID:         userID,
			VideoID:        videoID,
			EngagementType: "like",
			DeletedAt:      &deletedAt,
		})
	})
	require.NoError(t, err)

	record, err = repo.Get(ctx, nil, userID, videoID, "like")
	require.NoError(t, err)
	require.NotNil(t, record.DeletedAt)

	// 默认不返回已删除记录
	list, err = repo.ListByUser(ctx, nil, userID, nil, false, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 0)

	// includeDeleted = true 返回已删除记录
	list, err = repo.ListByUser(ctx, nil, userID, nil, true, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].DeletedAt)

	// 重新 Upsert 恢复记录
	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		return repo.Upsert(txCtx, sess, repositories.UpsertProfileEngagementInput{
			UserID:         userID,
			VideoID:        videoID,
			EngagementType: "like",
		})
	})
	require.NoError(t, err)

	record, err = repo.Get(ctx, nil, userID, videoID, "like")
	require.NoError(t, err)
	require.Nil(t, record.DeletedAt)
}
