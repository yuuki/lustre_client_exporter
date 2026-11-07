package collector

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_client_exporter/internal/discovery"
)

// BenchmarkRegistryCollect measures one full scrape of every default
// collector over the fixtures: discovery, reads from a FakeReader, parsing,
// mapping and emitting. It excludes real procfs reads and HTTP encoding.
func BenchmarkRegistryCollect(b *testing.B) {
	r := newFullFakeReader(b)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := discovery.DefaultPathConfig()
	registry := NewRegistry(logger, time.Minute, time.Minute,
		NewHealthCollector(r, cfg),
		NewSptlrpcCollector(r, cfg),
		NewLNetCollector(r, cfg, discovery.LNetSourceAuto, "lnetctl", logger),
		NewClientCollector(r, cfg, logger),
	)
	b.ReportAllocs()
	for b.Loop() {
		ch := make(chan prometheus.Metric, 1024)
		registry.Collect(ch)
		close(ch)
		if len(ch) == 0 {
			b.Fatal("scrape produced no metrics")
		}
	}
}
