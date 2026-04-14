package collector

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type failingCollector struct{}

func (f failingCollector) Name() string { return "failing" }

func (f failingCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	return nil, errors.New("collector failed")
}

func TestRegistryNonStrictReportsCollectorFailureAsSelfMetric(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := NewRegistry(logger, time.Second, time.Second, failingCollector{})
	ch := make(chan prometheus.Metric, 4)

	registry.Collect(ch)
	close(ch)

	var sawFailureMetric bool
	for metric := range ch {
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			t.Fatalf("non-strict registry emitted invalid metric: %v", err)
		}
		if dm.GetGauge().GetValue() == 0 {
			sawFailureMetric = true
		}
	}
	if !sawFailureMetric {
		t.Fatal("expected scrape success self-metric with value 0")
	}
}

func TestRegistryStrictEmitsInvalidMetricOnCollectorFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := NewRegistryWithStrict(logger, time.Second, time.Second, true, failingCollector{})
	ch := make(chan prometheus.Metric, 4)

	registry.Collect(ch)
	close(ch)

	var sawInvalid bool
	for metric := range ch {
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			sawInvalid = true
		}
	}
	if !sawInvalid {
		t.Fatal("expected strict registry to emit an invalid metric")
	}
}
