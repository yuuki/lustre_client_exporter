package collector

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_client_exporter/internal/discovery"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
)

func TestHealthCollector_MissingFile_ReturnsError(t *testing.T) {
	r := reader.NewFakeReader()
	c := NewHealthCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for missing health_check file")
	}
}

func TestSptlrpcCollector_MissingFile_ReturnsError(t *testing.T) {
	r := reader.NewFakeReader()
	c := NewSptlrpcCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for missing encrypt_page_pools")
	}
}

func TestLNetCollector_NoSources_ReturnsError(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error when no LNet sources available")
	}
}

func TestClientCollector_EmptyFS_ReturnsEmpty(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics for empty FS, want 0", len(metrics))
	}
}

func TestSptlrpcCollector_MalformedValue(t *testing.T) {
	r := reader.NewFakeReader()
	r.Files["/sys/kernel/debug/lustre/sptlrpc/encrypt_page_pools"] = []byte("physical pages: not_a_number\n")

	c := NewSptlrpcCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for malformed number")
	}
}

func TestLNetCollector_Auto_FallsBackToLNetCtl(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	r.Commands["lnetctl stats show"] = []byte(`{"statistics":{"msgs_alloc":0,"msgs_max":1,"errors":0,"send_count":10,"recv_count":5,"route_count":0,"drop_count":0,"send_length":100,"recv_length":50,"route_length":0,"drop_length":0}}`)

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceAuto, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 11 {
		t.Errorf("got %d metrics, want 11 (stats only, no params)", len(metrics))
	}
}

// A source line whose operation name is not valid UTF-8 (a corrupt or foreign
// file) must fail only its collector. It used to panic in the emitter, inside
// a scrape goroutine, which crashed the whole exporter.
func TestRegistry_InvalidUTF8LabelFailsOnlyItsCollector(t *testing.T) {
	r := reader.NewFakeReader()
	r.Files["/sys/fs/lustre/health_check"] = []byte("healthy\n")
	r.Files["/proc/fs/lustre/ldlm/services/ldlm_cbd/stats"] = []byte("ldlm_bl_callback\xff 1 samples [usec] 1 2 3\n")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewRegistry(logger, time.Second, time.Second,
		NewHealthCollector(r, discovery.DefaultPathConfig()),
		NewClientCollector(r, discovery.DefaultPathConfig(), logger),
	)
	ch := make(chan prometheus.Metric, 16)

	registry.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}
	assertMetric(t, metrics, "lustre_scrape_collector_success", map[string]string{"collector": "client"}, 0)
	assertMetric(t, metrics, "lustre_scrape_collector_success", map[string]string{"collector": "health"}, 1)
	assertMetric(t, metrics, "lustre_health_check", map[string]string{"component": "health", "target": "lustre"}, 1)
}
