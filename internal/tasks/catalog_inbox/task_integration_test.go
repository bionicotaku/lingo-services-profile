package cataloginbox_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	videov1 "github.com/bionicotaku/lingo-services-catalog/api/video/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-services-profile/internal/tasks/catalog_inbox"
	"github.com/bionicotaku/lingo-utils/gcpubsub"
	outboxcfg "github.com/bionicotaku/lingo-utils/outbox/config"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/docker/go-connections/nat"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
)

func TestCatalogInboxTask_UpsertsProjection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, terminate := startPostgres(ctx, t)
	defer terminate()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	applyMigrations(ctx, t, pool)

	logger := log.NewStdLogger(io.Discard)
	inboxRepo := repositories.NewInboxRepository(pool, logger, outboxcfg.Config{Schema: "profile"})
	projectionRepo := repositories.NewProfileVideoProjectionRepository(pool, logger)
	manager, err := txmanager.NewManager(pool, txmanager.Config{}, txmanager.Dependencies{Logger: logger})
	require.NoError(t, err)

	videoID := uuid.New()
	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	event := &videov1.Event{
		EventId:       uuid.NewString(),
		EventType:     videov1.EventType_EVENT_TYPE_VIDEO_CREATED,
		AggregateId:   videoID.String(),
		AggregateType: "video",
		Version:       1,
		OccurredAt:    occurredAt.Format(time.RFC3339Nano),
		Payload: &videov1.Event_Created{Created: &videov1.Event_VideoCreated{
			VideoId:     videoID.String(),
			Title:       "Sample Title",
			Description: optionalString("Sample Description"),
			Version:     1,
			OccurredAt:  occurredAt.Format(time.RFC3339Nano),
			Status:      "published",
		}},
	}

	msg := buildMessage(t, event)
	stub := &stubSubscriber{messages: []*gcpubsub.Message{msg}}

	cfg := outboxcfg.Config{Schema: "profile", Inbox: outboxcfg.InboxConfig{SourceService: "catalog", MaxConcurrency: 1}}
	task := cataloginbox.NewTask(stub, inboxRepo, projectionRepo, manager, logger, cfg.Inbox)
	require.NotNil(t, task)

	require.NoError(t, task.Run(ctx))

	record, err := projectionRepo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, "Sample Title", record.Title)
	require.NotNil(t, record.Description)
	require.Equal(t, int64(1), record.Version)
	require.Equal(t, "published", deref(record.Status))

	// send stale update should be ignored
	updateEvent := &videov1.Event{
		EventId:       uuid.NewString(),
		EventType:     videov1.EventType_EVENT_TYPE_VIDEO_UPDATED,
		AggregateId:   videoID.String(),
		AggregateType: "video",
		Version:       1,
		OccurredAt:    occurredAt.Add(time.Minute).Format(time.RFC3339Nano),
		Payload: &videov1.Event_Updated{Updated: &videov1.Event_VideoUpdated{
			VideoId: videoID.String(),
			Title:   optionalString("Stale Title"),
			Version: 1,
		}},
	}

	freshEvent := &videov1.Event{
		EventId:       uuid.NewString(),
		EventType:     videov1.EventType_EVENT_TYPE_VIDEO_UPDATED,
		AggregateId:   videoID.String(),
		AggregateType: "video",
		Version:       2,
		OccurredAt:    occurredAt.Add(2 * time.Minute).Format(time.RFC3339Nano),
		Payload: &videov1.Event_Updated{Updated: &videov1.Event_VideoUpdated{
			VideoId:     videoID.String(),
			Title:       optionalString("Fresh Title"),
			Description: optionalString("Fresh Description"),
			Version:     2,
		}},
	}

	stub.messages = []*gcpubsub.Message{buildMessage(t, updateEvent), buildMessage(t, freshEvent)}
	require.NoError(t, task.Run(ctx))

	record, err = projectionRepo.Get(ctx, nil, videoID)
	require.NoError(t, err)
	require.Equal(t, int64(2), record.Version)
	require.Equal(t, "Fresh Title", record.Title)
	require.Equal(t, "Fresh Description", deref(record.Description))
}

// stubSubscriber delivers queued messages synchronously.
type stubSubscriber struct {
	messages []*gcpubsub.Message
}

func (s *stubSubscriber) Receive(ctx context.Context, handler func(context.Context, *gcpubsub.Message) error) error {
	for _, msg := range s.messages {
		if err := handler(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *stubSubscriber) Stop() {}

func buildMessage(t *testing.T, evt *videov1.Event) *gcpubsub.Message {
	data, err := proto.Marshal(evt)
	require.NoError(t, err)
	return &gcpubsub.Message{
		ID:   uuid.NewString(),
		Data: data,
		Attributes: map[string]string{
			"event_id":       evt.GetEventId(),
			"event_type":     evt.GetEventType().String(),
			"aggregate_id":   evt.GetAggregateId(),
			"aggregate_type": evt.GetAggregateType(),
		},
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

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
		t.Skipf("skip catalog inbox tests: cannot start postgres container: %v", err)
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
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		_, execErr := pool.Exec(ctx, string(content))
		require.NoErrorf(t, execErr, "apply migration %s", filepath.Base(path))
	}
}
