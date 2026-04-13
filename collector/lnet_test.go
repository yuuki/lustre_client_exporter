package collector

import (
	"context"
	"log/slog"
	"os"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

func newTestLNetFakeReader(t *testing.T) *reader.FakeReader {
	t.Helper()
	r := reader.NewFakeReader()

	stats, err := os.ReadFile("../testdata/lnet/stats.txt")
	if err != nil {
		t.Fatal(err)
	}
	r.Files["/proc/sys/lnet/stats"] = stats

	params := map[string]string{
		"console_backoff":             "../testdata/lnet/params/console_backoff",
		"console_max_delay_centisecs": "../testdata/lnet/params/console_max_delay_centisecs",
		"console_min_delay_centisecs": "../testdata/lnet/params/console_min_delay_centisecs",
		"console_ratelimit":           "../testdata/lnet/params/console_ratelimit",
		"debug_mb":                    "../testdata/lnet/params/debug_mb",
		"panic_on_lbug":               "../testdata/lnet/params/panic_on_lbug",
		"watchdog_ratelimit":          "../testdata/lnet/params/watchdog_ratelimit",
		"catastrophe":                 "../testdata/lnet/params/catastrophe",
		"lnet_memused":                "../testdata/lnet/params/lnet_memused",
	}

	for name, path := range params {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		r.Files["/proc/sys/lnet/"+name] = data
	}

	return r
}

func TestLNetCollector_DebugFS(t *testing.T) {
	r := newTestLNetFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 11 from stats + 9 from params = 20
	if len(metrics) != 20 {
		t.Fatalf("got %d metrics, want 20", len(metrics))
	}
}

func TestLNetCollector_LNetCtl(t *testing.T) {
	r := newTestLNetFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	jsonData, err := os.ReadFile("../testdata/lnet/lnetctl_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl stats show"] = jsonData

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceLNetCtl, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 11 from lnetctl + 9 from params = 20
	if len(metrics) != 20 {
		t.Fatalf("got %d metrics, want 20", len(metrics))
	}

	// Verify a specific metric value
	for _, m := range metrics {
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLNetCollector_Auto_FallbackToLNetCtl(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	jsonData, err := os.ReadFile("../testdata/lnet/lnetctl_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl stats show"] = jsonData

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceAuto, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 11 from lnetctl, 0 params (no proc files)
	if len(metrics) != 11 {
		t.Fatalf("got %d metrics, want 11", len(metrics))
	}
}
