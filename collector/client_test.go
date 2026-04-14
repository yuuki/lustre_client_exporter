package collector

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

func newTestClientFakeReader(t *testing.T) *reader.FakeReader {
	t.Helper()
	r := reader.NewFakeReader()

	// Set up llite target
	r.Globs["/proc/fs/lustre/llite/*/stats"] = []string{
		"/proc/fs/lustre/llite/scratch-ffff0001/stats",
	}
	r.Globs["/proc/fs/lustre/mdc/*/stats"] = []string{
		"/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/stats",
	}
	r.Globs["/proc/fs/lustre/osc/*/stats"] = []string{
		"/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/stats",
	}

	// llite stats
	loadFixture(t, r, "/proc/fs/lustre/llite/scratch-ffff0001/stats", "../testdata/llite/stats.txt")

	// llite single files
	singleFiles := []string{
		"blocksize", "filesfree", "filestotal",
		"kbytesavail", "kbytesfree", "kbytestotal",
		"checksum_pages", "default_easize", "lazystatfs",
		"max_easize", "max_read_ahead_mb", "max_read_ahead_per_file_mb",
		"max_read_ahead_whole_mb", "statahead_agl", "statahead_max", "xattr_cache",
	}
	for _, name := range singleFiles {
		loadFixture(t, r, "/proc/fs/lustre/llite/scratch-ffff0001/"+name, "../testdata/llite/"+name)
	}

	// mdc stats and rpc_stats
	loadFixture(t, r, "/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/stats", "../testdata/mdc/stats.txt")
	loadFixture(t, r, "/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/rpc_stats", "../testdata/mdc/rpc_stats.txt")

	// osc stats and rpc_stats
	loadFixture(t, r, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/stats", "../testdata/osc/stats.txt")
	loadFixture(t, r, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/rpc_stats", "../testdata/osc/rpc_stats.txt")

	return r
}

func loadFixture(t *testing.T, r *reader.FakeReader, targetPath, fixturePath string) {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("loading fixture %s: %v", fixturePath, err)
	}
	r.Files[targetPath] = data
}

func TestClientCollector(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// llite: 15 stats + 16 single files = 31
	// mdc: 24 rpc_stats
	// osc: 26 rpc_stats
	// Total: 31 + 24 + 26 = 81
	if len(metrics) < 50 {
		t.Fatalf("got %d metrics, expected at least 50", len(metrics))
	}

	t.Logf("collected %d metrics total", len(metrics))
}

func TestClientCollector_NoTargets(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics for no targets, want 0", len(metrics))
	}
}

func TestClientCollector_RPCStatsWithoutStatsFile(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/llite/*/stats"] = nil
	r.Globs["/proc/fs/lustre/mdc/*/stats"] = nil
	r.Globs["/proc/fs/lustre/osc/*/stats"] = nil
	r.Globs["/proc/fs/lustre/osc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/osc/nonexistent-OST9999-osc-0000000000000000/rpc_stats",
	}
	loadFixture(t, r, "/proc/fs/lustre/osc/nonexistent-OST9999-osc-0000000000000000/rpc_stats", "../testdata/osc/rpc_stats.txt")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected rpc_stats metrics, got none")
	}
}

func TestClientCollector_RPCStatsGSICompatibleLabels(t *testing.T) {
	const target = "nonexistent-OST9999-osc-0000000000000000"
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/osc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/osc/" + target + "/rpc_stats",
	}
	r.Files["/proc/fs/lustre/osc/"+target+"/rpc_stats"] = []byte(`
snapshot_time:         1681000000.123456789 (secs.nsecs)

                        read            write
pages per rpc         rpcs   % cum %   rpcs   % cum %
1:                   147535  93  93   |     148562  92  92

                        read            write
rpcs in flight        rpcs   % cum %   rpcs   % cum %
7:                       16   0  99   |        315   0  98

                        read            write
offset                rpcs   % cum % |       rpcs   % cum %
0:                   153011  96  96   |     158331  99  99
`)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMetric(t, metrics, "lustre_pages_per_rpc_total", map[string]string{
		"component": "client",
		"operation": "read",
		"size":      "1",
		"target":    target,
	}, 147535)
	assertMetric(t, metrics, "lustre_rpcs_in_flight", map[string]string{
		"component": "client",
		"operation": "read",
		"size":      "7",
		"target":    target,
		"type":      "osc",
	}, 16)
	assertMetric(t, metrics, "lustre_rpcs_offset", map[string]string{
		"component": "client",
		"operation": "write",
		"size":      "0",
		"target":    target,
	}, 158331)
}

func TestClientCollector_LDLMCBDStats(t *testing.T) {
	r := reader.NewFakeReader()
	loadFixture(t, r, "/proc/fs/lustre/ldlm/services/ldlm_cbd/stats", "../testdata/ldlm/ldlm_cbd_stats.txt")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMetric(t, metrics, "lustre_ldlm_cbd_stats", map[string]string{
		"operation": "ldlm_bl_callback",
	}, 1236010)
	assertMetric(t, metrics, "lustre_ldlm_cbd_stats", map[string]string{
		"operation": "ldlm_cp_callback",
	}, 421007)
	assertMetric(t, metrics, "lustre_ldlm_cbd_stats", map[string]string{
		"operation": "ldlm_gl_callback",
	}, 99914)
	assertMetric(t, metrics, "lustre_ldlm_cbd_stats", map[string]string{
		"operation": "reqbuf_avail",
	}, 3575360)
}

func TestClientCollector_StrictReturnsErrorOnTargetFailure(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/llite/*/stats"] = []string{
		"/proc/fs/lustre/llite/missing/stats",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), logger, true)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected strict client collector to return target read error")
	}
}

func assertMetric(t *testing.T, metrics []prometheus.Metric, name string, labels map[string]string, value float64) {
	t.Helper()
	for _, metric := range metrics {
		if extractMetricName(metric.Desc().String()) != name {
			continue
		}
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			t.Fatal(err)
		}
		if !labelsMatch(dm.GetLabel(), labels) {
			continue
		}
		var gotValue float64
		var gotType string
		switch {
		case dm.GetCounter() != nil:
			gotType = "counter"
			gotValue = dm.GetCounter().GetValue()
		case dm.GetGauge() != nil:
			gotType = "gauge"
			gotValue = dm.GetGauge().GetValue()
		default:
			t.Fatalf("%s labels %v has no counter or gauge value", name, labels)
		}
		if gotValue == value {
			return
		}
		t.Fatalf("%s labels %v has wrong %s value: got %v, want %v", name, labels, gotType, gotValue, value)
	}
	t.Fatalf("metric %s with labels %v not found", name, labels)
}

func labelsMatch(got []*dto.LabelPair, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, label := range got {
		if want[label.GetName()] != label.GetValue() {
			return false
		}
	}
	return true
}
