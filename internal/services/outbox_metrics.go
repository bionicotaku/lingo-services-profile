package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
)

var (
	outboxMetricsMu      sync.Mutex
	outboxMetricsEnabled bool
	outboxSuccessCounter metric.Int64Counter
	outboxFailureCounter metric.Int64Counter
	outboxLagHistogram   metric.Float64Histogram
)

const (
	outboxSuccessMetricName = "profile_outbox_enqueue_total"
	outboxFailureMetricName = "profile_outbox_enqueue_failures_total"
	outboxLagMetricName     = "profile_outbox_enqueue_lag_ms"
)

var (
	attrComponent = attribute.Key("component")
	attrEventType = attribute.Key("event_type")
	attrErrorKind = attribute.Key("error_kind")
)

type outboxMetrics struct {
	component string
}

func newOutboxMetrics(component string) *outboxMetrics {
	outboxMetricsMu.Lock()
	defer outboxMetricsMu.Unlock()
	if !outboxMetricsEnabled {
		initOutboxMetricsLocked()
	}
	if !outboxMetricsEnabled {
		return &outboxMetrics{}
	}
	return &outboxMetrics{component: component}
}

func initOutboxMetricsLocked() {
	provider := otel.GetMeterProvider()
	if provider == nil {
		provider = noopmetric.NewMeterProvider()
	}
	meter := provider.Meter("lingo-services-profile.services.outbox")

	var err error
	outboxSuccessCounter, err = meter.Int64Counter(outboxSuccessMetricName,
		metric.WithDescription("Number of domain events enqueued to profile outbox"))
	if err != nil {
		outboxMetricsEnabled = false
		return
	}
	outboxFailureCounter, err = meter.Int64Counter(outboxFailureMetricName,
		metric.WithDescription("Number of profile outbox enqueue attempts that failed"))
	if err != nil {
		outboxMetricsEnabled = false
		return
	}
	outboxLagHistogram, err = meter.Float64Histogram(outboxLagMetricName,
		metric.WithDescription("Lag between event occurrence time and enqueue time"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		outboxMetricsEnabled = false
		return
	}
	outboxMetricsEnabled = true
}

func (m *outboxMetrics) recordSuccess(ctx context.Context, eventType string, occurredAt time.Time) {
	if m == nil || !outboxMetricsEnabled || outboxSuccessCounter == nil {
		return
	}
	attrs := metric.WithAttributes(
		attrComponent.String(m.component),
		attrEventType.String(eventType),
	)
	outboxSuccessCounter.Add(ctx, 1, attrs)
	if occurredAt.IsZero() || outboxLagHistogram == nil {
		return
	}
	lag := time.Since(occurredAt).Milliseconds()
	if lag < 0 {
		lag = 0
	}
	outboxLagHistogram.Record(ctx, float64(lag), attrs)
}

func (m *outboxMetrics) recordFailure(ctx context.Context, eventType string, err error) {
	if m == nil || !outboxMetricsEnabled || outboxFailureCounter == nil {
		return
	}
	errKind := "unknown"
	if err != nil {
		errKind = fmt.Sprintf("%T", err)
	}
	outboxFailureCounter.Add(ctx, 1, metric.WithAttributes(
		attrComponent.String(m.component),
		attrEventType.String(eventType),
		attrErrorKind.String(errKind),
	))
}
