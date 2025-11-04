package cataloginbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	videov1 "github.com/bionicotaku/lingo-services-catalog/api/video/v1"
	"github.com/bionicotaku/lingo-services-profile/internal/models/po"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	"github.com/bionicotaku/lingo-utils/outbox/store"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type eventHandler struct {
	projections *repositories.ProfileVideoProjectionRepository
	log         *log.Helper
	metrics     *inboxMetrics
	clock       func() time.Time
}

func newEventHandler(repo *repositories.ProfileVideoProjectionRepository, logger log.Logger, metrics *inboxMetrics) *eventHandler {
	return &eventHandler{
		projections: repo,
		log:         log.NewHelper(logger),
		metrics:     metrics,
		clock:       time.Now,
	}
}

func (h *eventHandler) Handle(ctx context.Context, sess txmanager.Session, evt *videov1.Event, inboxEvt *store.InboxEvent) error {
	if evt == nil {
		return fmt.Errorf("catalog inbox: nil event")
	}

	aggregateID := evt.GetAggregateId()
	if aggregateID == "" && inboxEvt != nil && inboxEvt.AggregateID != nil {
		aggregateID = *inboxEvt.AggregateID
	}
	videoID, err := uuid.Parse(aggregateID)
	if err != nil {
		return fmt.Errorf("catalog inbox: parse aggregate_id: %w", err)
	}

	occurredAt, err := parseRFC3339(evt.GetOccurredAt())
	if err != nil {
		if h.metrics != nil {
			h.metrics.recordFailure(ctx, evt.GetEventType().String(), err)
		}
		return fmt.Errorf("catalog inbox: parse occurred_at: %w", err)
	}
	if occurredAt.IsZero() {
		occurredAt = h.clock().UTC()
	}

	var handleErr error
	switch evt.GetEventType() {
	case videov1.EventType_EVENT_TYPE_VIDEO_CREATED:
		handleErr = h.handleCreated(ctx, sess, evt, videoID, occurredAt)
	case videov1.EventType_EVENT_TYPE_VIDEO_UPDATED:
		handleErr = h.handleUpdated(ctx, sess, evt, videoID, occurredAt)
	case videov1.EventType_EVENT_TYPE_VIDEO_DELETED:
		handleErr = h.handleDeleted(ctx, sess, evt, videoID, occurredAt)
	default:
		h.log.WithContext(ctx).Debugw("msg", "catalog inbox: skip unsupported event", "event_type", evt.GetEventType().String(), "event_id", evt.GetEventId())
		return nil
	}

	if handleErr != nil {
		if h.metrics != nil {
			h.metrics.recordFailure(ctx, evt.GetEventType().String(), handleErr)
		}
		return handleErr
	}

	if h.metrics != nil {
		h.metrics.recordSuccess(ctx, evt.GetEventType().String(), occurredAt, h.clock())
	}
	return nil
}

func (h *eventHandler) handleCreated(ctx context.Context, sess txmanager.Session, evt *videov1.Event, videoID uuid.UUID, occurredAt time.Time) error {
	payload := evt.GetCreated()
	if payload == nil {
		return errors.New("catalog inbox: created payload missing")
	}

	version := eventVersion(evt.GetVersion(), payload.GetVersion())
	input := repositories.UpsertVideoProjectionInput{
		VideoID:           videoID,
		Title:             defaultTitle(payload.GetTitle()),
		Description:       cloneString(payload.Description),
		DurationMicros:    cloneInt64(payload.DurationMicros),
		ThumbnailURL:      nil,
		HLSMasterPlaylist: nil,
		Status:            optionalStringPtr(payload.GetStatus()),
		VisibilityStatus:  nil,
		PublishedAt:       optionalTime(payload.PublishedAt),
		Version:           version,
		UpdatedAt:         &occurredAt,
	}

	if err := h.projections.Upsert(ctx, sess, input); err != nil {
		return fmt.Errorf("catalog inbox: upsert created: %w", err)
	}
	return nil
}

func (h *eventHandler) handleUpdated(ctx context.Context, sess txmanager.Session, evt *videov1.Event, videoID uuid.UUID, occurredAt time.Time) error {
	payload := evt.GetUpdated()
	if payload == nil {
		return errors.New("catalog inbox: updated payload missing")
	}

	current, err := h.loadCurrent(ctx, sess, videoID)
	if err != nil {
		return err
	}
	if current == nil {
		h.log.WithContext(ctx).Debugw("msg", "catalog inbox: skip update without projection", "video_id", videoID)
		return nil
	}

	version := eventVersion(evt.GetVersion(), payload.GetVersion())
	if !shouldApply(version, current.Version) {
		h.log.WithContext(ctx).Debugw("msg", "catalog inbox: skip stale update", "video_id", videoID, "event_version", version, "current_version", current.Version)
		return nil
	}

	title := current.Title
	if payload.Title != nil {
		title = payload.GetTitle()
	}

	input := repositories.UpsertVideoProjectionInput{
		VideoID:           videoID,
		Title:             defaultTitle(title),
		Description:       coalesceString(payload.Description, current.Description),
		DurationMicros:    coalesceInt64(payload.DurationMicros, current.DurationMicros),
		ThumbnailURL:      coalesceString(payload.ThumbnailUrl, current.ThumbnailURL),
		HLSMasterPlaylist: coalesceString(payload.HlsMasterPlaylist, current.HLSMasterPlaylist),
		Status:            coalesceStringValue(payload.Status, current.Status),
		VisibilityStatus:  coalesceStringValue(payload.VisibilityStatus, current.VisibilityStatus),
		PublishedAt:       coalesceTime(payload.PublishedAt, current.PublishedAt),
		Version:           version,
		UpdatedAt:         &occurredAt,
	}

	if err := h.projections.Upsert(ctx, sess, input); err != nil {
		return fmt.Errorf("catalog inbox: upsert updated: %w", err)
	}
	return nil
}

func (h *eventHandler) handleDeleted(ctx context.Context, sess txmanager.Session, evt *videov1.Event, videoID uuid.UUID, occurredAt time.Time) error {
	current, err := h.loadCurrent(ctx, sess, videoID)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}

	version := evt.GetVersion()
	if payload := evt.GetDeleted(); payload != nil && payload.GetVersion() > 0 {
		version = payload.GetVersion()
	}
	if !shouldApply(version, current.Version) {
		h.log.WithContext(ctx).Debugw("msg", "catalog inbox: skip stale delete", "video_id", videoID, "event_version", version, "current_version", current.Version)
		return nil
	}

	statusDeleted := "deleted"
	input := repositories.UpsertVideoProjectionInput{
		VideoID:           videoID,
		Title:             defaultTitle(current.Title),
		Description:       current.Description,
		DurationMicros:    current.DurationMicros,
		ThumbnailURL:      current.ThumbnailURL,
		HLSMasterPlaylist: current.HLSMasterPlaylist,
		Status:            &statusDeleted,
		VisibilityStatus:  current.VisibilityStatus,
		PublishedAt:       current.PublishedAt,
		Version:           version,
		UpdatedAt:         &occurredAt,
	}

	if err := h.projections.Upsert(ctx, sess, input); err != nil {
		return fmt.Errorf("catalog inbox: upsert deleted: %w", err)
	}
	return nil
}

func (h *eventHandler) loadCurrent(ctx context.Context, sess txmanager.Session, videoID uuid.UUID) (*po.ProfileVideoProjection, error) {
	record, err := h.projections.Get(ctx, sess, videoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("catalog inbox: load projection: %w", err)
	}
	return record, nil
}

func parseRFC3339(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return ts.UTC(), nil
}

func eventVersion(eventVersion int64, payloadVersion int64) int64 {
	if payloadVersion > 0 {
		return payloadVersion
	}
	return eventVersion
}

func shouldApply(newVersion, currentVersion int64) bool {
	if newVersion == 0 {
		return true
	}
	return newVersion > currentVersion
}

func defaultTitle(title string) string {
	if title == "" {
		return "(untitled)"
	}
	return title
}

func optionalStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func optionalTime(ptr *string) *time.Time {
	if ptr == nil {
		return nil
	}
	parsed, err := parseRFC3339(*ptr)
	if err != nil || parsed.IsZero() {
		return nil
	}
	return &parsed
}

func coalesceString(newVal *string, current *string) *string {
	if newVal != nil {
		val := *newVal
		return &val
	}
	return current
}

func coalesceStringValue(newVal *string, current *string) *string {
	if newVal != nil {
		val := *newVal
		if val == "" {
			return nil
		}
		return &val
	}
	return current
}

func coalesceInt64(newVal *int64, current *int64) *int64 {
	if newVal != nil {
		val := *newVal
		return &val
	}
	return current
}

func coalesceTime(newVal *string, current *time.Time) *time.Time {
	if newVal != nil {
		parsed, err := parseRFC3339(*newVal)
		if err != nil || parsed.IsZero() {
			return current
		}
		return &parsed
	}
	return current
}

func cloneString(ptr *string) *string {
	if ptr == nil {
		return nil
	}
	val := *ptr
	return &val
}

func cloneInt64(ptr *int64) *int64 {
	if ptr == nil {
		return nil
	}
	val := *ptr
	return &val
}
