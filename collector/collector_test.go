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

type noopCollector struct{}

func (n noopCollector) Name() string { return "procfs" }

func (n noopCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	return nil, nil
}

type sourcedNoopCollector struct {
	name   string
	source string
}

func (n sourcedNoopCollector) Name() string { return n.name }

func (n sourcedNoopCollector) ScrapeSource() string { return n.source }

func (n sourcedNoopCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	return nil, nil
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

func TestRegistryEmitsGSICompatibleScrapeDurationSummary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := NewRegistry(logger, time.Second, time.Second, noopCollector{})
	ch := make(chan prometheus.Metric, 4)

	registry.Collect(ch)
	close(ch)

	for metric := range ch {
		if extractMetricName(metric.Desc().String()) != "lustre_exporter_scrape_duration_seconds" {
			continue
		}
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			t.Fatal(err)
		}
		if dm.GetSummary() == nil {
			t.Fatal("expected scrape duration to be a summary")
		}
		if !labelsMatch(dm.GetLabel(), map[string]string{
			"result": "success",
			"source": "procfs",
		}) {
			t.Fatalf("scrape duration labels = %v, want result=success source=procfs", dm.GetLabel())
		}
		return
	}

	t.Fatal("expected lustre_exporter_scrape_duration_seconds summary")
}

func TestRegistryAggregatesScrapeDurationBySource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := NewRegistry(logger, time.Second, time.Second,
		sourcedNoopCollector{name: "sptlrpc", source: "sys"},
		sourcedNoopCollector{name: "lnet", source: "sys"},
	)
	ch := make(chan prometheus.Metric, 8)

	registry.Collect(ch)
	close(ch)

	var matches int
	for metric := range ch {
		if extractMetricName(metric.Desc().String()) != "lustre_exporter_scrape_duration_seconds" {
			continue
		}
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			t.Fatal(err)
		}
		if !labelsMatch(dm.GetLabel(), map[string]string{
			"result": "success",
			"source": "sys",
		}) {
			continue
		}
		matches++
		if dm.GetSummary().GetSampleCount() != 1 {
			t.Fatalf("sample count = %d, want 1", dm.GetSummary().GetSampleCount())
		}
	}

	if matches != 1 {
		t.Fatalf("got %d sys scrape duration summaries, want 1", matches)
	}
}
