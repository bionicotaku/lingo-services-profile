package repositories_test

import (
	"context"
	"io"
	"testing"

	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestProfileUsersRepositoryIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, terminate := startPostgres(ctx, t)
	defer terminate()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	repo := repositories.NewProfileUsersRepository(pool, log.NewStdLogger(io.Discard))
	txMgr := newTxManager(t, pool)

	userID := uuid.New()
	prefs := map[string]any{"learning_goal": "fluency", "daily_quota_minutes": 30}
	input := repositories.UpsertProfileUserInput{
		UserID:         userID,
		DisplayName:    "Test User",
		AvatarURL:      stringPtr("https://example.com/avatar.png"),
		ProfileVersion: 1,
		Preferences:    prefs,
	}

	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		_, err := repo.Upsert(txCtx, sess, input)
		return err
	})
	require.NoError(t, err)

	record, err := repo.Get(ctx, nil, userID)
	require.NoError(t, err)
	require.Equal(t, "Test User", record.DisplayName)
	require.Equal(t, int64(1), record.ProfileVersion)
	require.Equal(t, prefs["learning_goal"], record.PreferencesJSON["learning_goal"])
	require.Equal(t, prefs["daily_quota_minutes"], record.PreferencesJSON["daily_quota_minutes"])

	input.DisplayName = "Updated User"
	input.ProfileVersion = 2

	err = txMgr.WithinTx(ctx, txmanager.TxOptions{}, func(txCtx context.Context, sess txmanager.Session) error {
		_, err := repo.Upsert(txCtx, sess, input)
		return err
	})
	require.NoError(t, err)

	updated, err := repo.Get(ctx, nil, userID)
	require.NoError(t, err)
	require.Equal(t, "Updated User", updated.DisplayName)
	require.Equal(t, int64(2), updated.ProfileVersion)
}
