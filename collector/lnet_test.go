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

	found := map[string]float64{}
	for _, m := range metrics {
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
		if dm.GetGauge() != nil {
			found[extractMetricName(m.Desc().String())] = dm.GetGauge().GetValue()
		} else if dm.GetCounter() != nil {
			found[extractMetricName(m.Desc().String())] = dm.GetCounter().GetValue()
		}
	}

	if found["lustre_send_count_total"] != 512 {
		t.Errorf("lustre_send_count_total = %f, want 512", found["lustre_send_count_total"])
	}
	if found["lustre_fail_maximum"] != 7 {
		t.Errorf("lustre_fail_maximum = %f, want 7", found["lustre_fail_maximum"])
	}
}

func TestLNetCollector_LNetCtlNetShowAddsNIDLabels(t *testing.T) {
	r := newTestLNetFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	statsData, err := os.ReadFile("../testdata/lnet/lnetctl_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	netData, err := os.ReadFile("../testdata/lnet/lnetctl_net_show.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl stats show"] = statsData
	r.Commands["lnetctl net show"] = netData

	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceLNetCtl, "lnetctl", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, m := range metrics {
		if extractMetricName(m.Desc().String()) != "lustre_send_count_total" {
			continue
		}
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
		for _, label := range dm.GetLabel() {
			if label.GetName() == "nid" && label.GetValue() == "0@lo" && dm.GetCounter().GetValue() == 180076 {
				found = true
			}
		}
	}

	if !found {
		t.Fatal("expected lustre_send_count_total with nid=\"0@lo\" from lnetctl net show")
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
