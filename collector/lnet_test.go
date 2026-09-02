package collector

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_client_exporter/internal/discovery"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
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
		"fail_err":                    "../testdata/lnet/params/fail_err",
		"fail_val":                    "../testdata/lnet/params/fail_val",
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

	// 11 from stats + 11 from params = 22
	if len(metrics) != 22 {
		t.Fatalf("got %d metrics, want 22", len(metrics))
	}

	assertMetric(t, metrics, "lustre_allocated", map[string]string{
		"component": "lnet",
		"target":    "lnet",
	}, 0)
	assertMetric(t, metrics, "lustre_console_backoff_enabled", map[string]string{
		"component": "lnet",
		"target":    "lnet",
	}, 1)
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

	// 11 from lnetctl + 11 from params = 22
	if len(metrics) != 22 {
		t.Fatalf("got %d metrics, want 22", len(metrics))
	}

	// Verify a specific metric value
	for _, m := range metrics {
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLNetCollector_DebugFSReadsDebugFSStatsAndParams(t *testing.T) {
	stats, err := os.ReadFile("../testdata/lnet/stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	r := reader.NewFakeReader()
	r.Files["/sys/kernel/debug/lnet/stats"] = stats
	r.Files["/sys/kernel/debug/lnet/fail_val"] = []byte("7\n")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMetric(t, metrics, "lustre_send_count_total", map[string]string{
		"component": "lnet",
		"target":    "lnet",
	}, 512)
	assertMetric(t, metrics, "lustre_fail_maximum", map[string]string{
		"component": "lnet",
		"target":    "lnet",
	}, 7)
}

func TestLNetCollector_LNetCtlNetShowAddsNIDLabels(t *testing.T) {
	r := newTestLNetFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	statsData, err := os.ReadFile("../testdata/lnet/lnetctl_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	netData, err := os.ReadFile("../testdata/lnet/lnetctl_net_show_verbose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl stats show"] = statsData
	r.Commands["lnetctl net show -v 3"] = netData

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceLNetCtl, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMetric(t, metrics, "lustre_send_count_total", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "0@lo",
	}, 180076)
	assertMetric(t, metrics, "lustre_lnet_ni_up", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "10.200.200.54@o2ib",
	}, 0)
	assertMetric(t, metrics, "lustre_lnet_ni_health", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "0@lo",
	}, 0)

	for _, m := range metrics {
		if extractMetricName(m.Desc().String()) != "lustre_send_count_total" {
			continue
		}
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
		hasNID := false
		for _, label := range dm.GetLabel() {
			if label.GetName() == "nid" {
				hasNID = true
			}
		}
		if !hasNID {
			t.Fatal("expected no unlabeled lustre_send_count_total after nid counts")
		}
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewRegistry(logger, 0, 0, c))
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
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

func TestLNetCollector_AutoFallbackReportsLctlScrapeSource(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	jsonData, err := os.ReadFile("../testdata/lnet/lnetctl_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl stats show"] = jsonData

	registry := NewRegistry(logger, 0, 0, NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceAuto, "lnetctl", logger))
	ch := make(chan prometheus.Metric, 16)

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
		if labelsMatch(dm.GetLabel(), map[string]string{
			"result": "success",
			"source": "lctl",
		}) {
			return
		}
	}

	t.Fatal("expected auto fallback scrape duration to report source=lctl")
}

func TestLNetCollector_AutoSupplementalNIDoesNotMixCountLabels(t *testing.T) {
	r := newTestLNetFakeReader(t)
	verbose, err := os.ReadFile("../testdata/lnet/lnetctl_net_show_verbose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl net show -v 3"] = verbose
	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceAuto, "lnetctl", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetric(t, metrics, "lustre_lnet_ni_up", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "10.200.200.54@o2ib",
	}, 0)
	assertMetric(t, metrics, "lustre_lnet_ni_health", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "0@lo",
	}, 0)
	assertMetric(t, metrics, "lustre_send_count_total", map[string]string{
		"component": "lnet", "target": "lnet",
	}, 512)
	for _, m := range metrics {
		if extractMetricName(m.Desc().String()) != "lustre_send_count_total" {
			continue
		}
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
		for _, label := range dm.GetLabel() {
			if label.GetName() == "nid" {
				t.Fatal("auto supplemental must not add nid-labeled send_count")
			}
		}
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewRegistry(slog.New(slog.NewTextHandler(os.Stderr, nil)), 0, 0, c))
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
	}
}

type commandSpyReader struct {
	*reader.FakeReader
	cmds []string
}

func (r *commandSpyReader) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.cmds = append(r.cmds, name+" "+strings.Join(args, " "))
	return r.FakeReader.RunCommand(ctx, name, args...)
}

func TestLNetCollector_DebugFSDoesNotRunLnetctl(t *testing.T) {
	spy := &commandSpyReader{FakeReader: newTestLNetFakeReader(t)}
	spy.Commands["lnetctl net show -v 3"] = []byte("not: [ yaml")
	c := NewLNetCollector(spy, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 22 {
		t.Fatalf("got %d, want 22", len(metrics))
	}
	if len(spy.cmds) != 0 {
		t.Fatalf("debugfs source must not call RunCommand, got %v", spy.cmds)
	}
	for _, m := range metrics {
		name := extractMetricName(m.Desc().String())
		if len(name) >= 15 && name[:15] == "lustre_lnet_ni_" {
			t.Fatalf("debugfs source must not emit %s", name)
		}
	}
}
