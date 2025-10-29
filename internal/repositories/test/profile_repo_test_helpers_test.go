package repositories_test

import (
	"io"
	"testing"
	"time"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func newTxManager(t *testing.T, pool *pgxpool.Pool) txmanager.Manager {
	t.Helper()
	mgr, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: log.NewStdLogger(io.Discard)})
	require.NoError(t, err)
	return mgr
}

func stringPtr(val string) *string {
	return &val
}

func timePtr(val time.Time) *time.Time {
	return &val
}
