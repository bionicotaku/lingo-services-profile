package cataloginbox

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
)

type inboxMetrics struct {
	success metric.Int64Counter
	failure metric.Int64Counter
	lag     metric.Float64Histogram
	enabled bool
}

func newInboxMetrics() *inboxMetrics {
	meterProvider := otel.GetMeterProvider()
	if meterProvider == nil {
		meterProvider = noopmetric.NewMeterProvider()
	}
	meter := meterProvider.Meter("lingo-services-profile.catalog_inbox")

	success, err := meter.Int64Counter("catalog_inbox_success_total", metric.WithDescription("Number of catalog projection events applied"))
	if err != nil {
		return &inboxMetrics{}
	}
	failure, err := meter.Int64Counter("catalog_inbox_failure_total", metric.WithDescription("Number of catalog projection events failed"))
	if err != nil {
		return &inboxMetrics{}
	}
	lag, err := meter.Float64Histogram("catalog_inbox_event_lag_ms", metric.WithDescription("Lag between event occurred_at and processing time"), metric.WithUnit("ms"))
	if err != nil {
		return &inboxMetrics{}
	}

	return &inboxMetrics{
		success: success,
		failure: failure,
		lag:     lag,
		enabled: true,
	}
}

func (m *inboxMetrics) recordSuccess(ctx context.Context, eventType string, occurredAt time.Time, now time.Time) {
	if m == nil || !m.enabled {
		return
	}
	attrs := metric.WithAttributes(attribute.String("event_type", eventType))
	m.success.Add(ctx, 1, attrs)
	if !occurredAt.IsZero() && !now.IsZero() {
		lag := now.Sub(occurredAt).Milliseconds()
		if lag < 0 {
			lag = 0
		}
		m.lag.Record(ctx, float64(lag), attrs)
	}
}

func (m *inboxMetrics) recordFailure(ctx context.Context, eventType string, _ error) {
	if m == nil || !m.enabled {
		return
	}
	attrs := metric.WithAttributes(attribute.String("event_type", eventType))
	m.failure.Add(ctx, 1, attrs)
}
