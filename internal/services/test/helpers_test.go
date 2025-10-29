package services_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgres(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_DB":       "profile",
		},
		WaitingFor: wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
			return fmt.Sprintf("postgres://postgres:postgres@%s:%s/profile?sslmode=disable", host, port.Port())
		}).WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("skip integration test: cannot start postgres container: %v", err)
	}

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://postgres:postgres@%s:%s/profile?sslmode=disable", host, port.Port())
	cleanup := func() {
		termCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(termCtx)
	}
	return dsn, cleanup
}

func applyMigrations(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	entries, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	require.NoError(t, err)
	sort.Strings(entries)

	for _, path := range entries {
		sqlBytes, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		_, execErr := pool.Exec(ctx, string(sqlBytes))
		require.NoErrorf(t, execErr, "apply migration %s", filepath.Base(path))
	}
}
