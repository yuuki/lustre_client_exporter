package collector

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_client_exporter/internal/discovery"
	"github.com/yuuki/lustre_client_exporter/internal/parser"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
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
	r.Globs["/proc/fs/lustre/mdc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/rpc_stats",
	}
	r.Globs["/proc/fs/lustre/osc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/rpc_stats",
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

	// osc tunables and import state (virtual path is .../state, fixture is state.txt)
	oscParamFiles := []string{
		"cur_dirty_bytes", "max_dirty_mb", "max_pages_per_rpc", "max_rpcs_in_flight", "active",
	}
	for _, name := range oscParamFiles {
		loadFixture(t, r, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/"+name, "../testdata/osc/"+name)
	}
	loadFixture(t, r, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/state", "../testdata/osc/state.txt")

	// mdc tunables and import state (no max_pages_per_rpc fixture)
	mdcParamFiles := []string{
		"max_rpcs_in_flight", "max_mod_rpcs_in_flight", "active",
	}
	for _, name := range mdcParamFiles {
		loadFixture(t, r, "/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/"+name, "../testdata/mdc/"+name)
	}
	loadFixture(t, r, "/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/state", "../testdata/mdc/state.txt")

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

func TestClientCollector_MDCOSCStats(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMetric(t, metrics, "lustre_stats_total", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"operation": "req_waittime",
	}, 200)
	assertMetric(t, metrics, "lustre_stats_seconds_sum", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"type":      "osc",
		"operation": "req_waittime",
	}, 0.5)
	assertMetric(t, metrics, "lustre_stats_seconds_sum", map[string]string{
		"component": "client",
		"target":    "scratch-MDT0000-mdc-ffff0001",
		"type":      "mdc",
		"operation": "mds_getattr",
	}, 0.1)
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

func TestClientCollector_MDCRPCStatsWithScalarInFlightLines(t *testing.T) {
	const target = "lfs-dn-h-MDT0000-mdc-ff36f2b293cbd800"
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/mdc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/mdc/" + target + "/rpc_stats",
	}
	r.Files["/proc/fs/lustre/mdc/"+target+"/rpc_stats"] = []byte(`
snapshot_time:            1776181530.650669515 secs.nsecs
modify_RPCs_in_flight:  0

                        modify
rpcs in flight        rpcs   % cum %
0:                       0   0   0
1:                 5268464  56  56

read RPCs in flight:  0
write RPCs in flight: 0
pending write pages:  0
pending read pages:   0

                        read                    write
rpcs in flight        rpcs   % cum % |       rpcs   % cum %
1:                       0   0   0   |          0   0   0
`)

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), "rpc collection failed") {
		t.Fatalf("unexpected rpc collection warning: %s", logs.String())
	}

	assertMetric(t, metrics, "lustre_rpcs_in_flight", map[string]string{
		"component": "client",
		"operation": "modify",
		"size":      "1",
		"target":    target,
		"type":      "mdc",
	}, 5268464)
	assertMetric(t, metrics, "lustre_rpcs_in_flight", map[string]string{
		"component": "client",
		"operation": "read",
		"size":      "1",
		"target":    target,
		"type":      "mdc",
	}, 0)
	assertMetric(t, metrics, "lustre_rpcs_in_flight", map[string]string{
		"component": "client",
		"operation": "write",
		"size":      "1",
		"target":    target,
		"type":      "mdc",
	}, 0)

	promRegistry := prometheus.NewRegistry()
	promRegistry.MustRegister(NewRegistry(logger, 0, 0, c))
	if _, err := promRegistry.Gather(); err != nil {
		t.Fatalf("unexpected prometheus gather error: %v", err)
	}
}

func TestClientCollector_RPCCurrentScalars(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetric(t, metrics, "lustre_rpcs_current", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"type":      "osc",
		"operation": "write",
	}, 5)
	assertMetric(t, metrics, "lustre_pending_pages", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"type":      "osc",
		"operation": "write",
	}, 120)
}

func TestClientCollector_DebugFSStatsWithProcFSRPCStats(t *testing.T) {
	for _, component := range []string{"mdc", "osc"} {
		for _, mode := range []struct {
			name   string
			strict bool
		}{
			{"non_strict", false},
			{"strict", true},
		} {
			t.Run(component+"/"+mode.name, func(t *testing.T) {
				const target = "scratch-target-ffff0001"
				r := reader.NewFakeReader()
				statsPath := "/sys/kernel/debug/lustre/" + component + "/" + target + "/stats"
				rpcPath := "/proc/fs/lustre/" + component + "/" + target + "/rpc_stats"
				r.Globs["/sys/kernel/debug/lustre/"+component+"/*/stats"] = []string{statsPath}
				r.Globs["/proc/fs/lustre/"+component+"/*/rpc_stats"] = []string{rpcPath}
				loadFixture(t, r, statsPath, "../testdata/"+component+"/stats.txt")
				r.Files[rpcPath] = []byte(`
snapshot_time:         1681000000.123456789 (secs.nsecs)
read RPCs in flight:  0
write RPCs in flight: 0
pending read pages:   0
pending write pages:  0

                        read            write
pages per rpc         rpcs   % cum %   rpcs   % cum %
1:                       10 100 100   |     20 100 100

                        read            write
rpcs in flight        rpcs   % cum %   rpcs   % cum %
1:                        8 100 100   |     12 100 100

                        read            write
offset                pages  % cum %  pages  % cum %
0:                        5 100 100   |     15 100 100
`)

				logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
				c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), logger, mode.strict)
				metrics, err := c.Collect(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				waitSeconds := 0.25
				if component == "osc" {
					waitSeconds = 0.5
				}
				assertMetric(t, metrics, "lustre_stats_seconds_sum", map[string]string{
					"component": "client", "target": target, "type": component, "operation": "req_waittime",
				}, waitSeconds)
				for _, operation := range []string{"read", "write"} {
					labels := map[string]string{
						"component": "client", "target": target, "type": component, "operation": operation,
					}
					assertMetric(t, metrics, "lustre_rpcs_current", labels, 0)
					assertMetric(t, metrics, "lustre_pending_pages", labels, 0)
				}
				for _, histogram := range []struct {
					name       string
					size       string
					readValue  float64
					writeValue float64
				}{
					{"lustre_pages_per_rpc_total", "1", 10, 20},
					{"lustre_rpcs_in_flight", "1", 8, 12},
					{"lustre_rpcs_offset", "0", 5, 15},
				} {
					labels := map[string]string{
						"component": "client", "target": target, "size": histogram.size, "operation": "read",
					}
					if histogram.name == "lustre_rpcs_in_flight" {
						labels["type"] = component
					}
					assertMetric(t, metrics, histogram.name, labels, histogram.readValue)
					labels["operation"] = "write"
					assertMetric(t, metrics, histogram.name, labels, histogram.writeValue)
				}

				registry := prometheus.NewRegistry()
				registry.MustRegister(NewRegistryWithStrict(logger, 0, 0, mode.strict, c))
				families, err := registry.Gather()
				if err != nil {
					t.Fatal(err)
				}
				for _, family := range families {
					if family.GetName() == "lustre_scrape_collector_success" {
						if len(family.Metric) != 1 || family.Metric[0].GetGauge().GetValue() != 1 {
							t.Fatalf("unexpected collector success: %v", family)
						}
						return
					}
				}
				t.Fatal("collector success metric not found")
			})
		}
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

func TestClientCollector_LLiteParamsFromSysFSWhenStatsInDebugFS(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/sys/kernel/debug/lustre/llite/*/stats"] = []string{
		"/sys/kernel/debug/lustre/llite/scratch-ffff0001/stats",
	}
	loadFixture(t, r, "/sys/kernel/debug/lustre/llite/scratch-ffff0001/stats", "../testdata/llite/stats.txt")
	loadFixture(t, r, "/sys/fs/lustre/llite/scratch-ffff0001/blocksize", "../testdata/llite/blocksize")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetric(t, metrics, "lustre_blocksize_bytes", map[string]string{
		"component": "client",
		"target":    "scratch-ffff0001",
	}, 4194304)
}

func TestClientCollector_StrictReturnsErrorOnDiscoveredStatsRead(t *testing.T) {
	r := newTestClientFakeReader(t)
	r.Errors["/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/stats"] = errors.New("permission denied")
	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientCollector_MissingSiblingRPCStatsIsNotStrictError(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/mdc/*/stats"] = []string{
		"/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/stats",
	}
	r.Globs["/proc/fs/lustre/osc/*/stats"] = []string{
		"/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/stats",
	}
	loadFixture(t, r, "/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/stats", "../testdata/mdc/stats.txt")
	loadFixture(t, r, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/stats", "../testdata/osc/stats.txt")

	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected stats metrics when sibling rpc_stats is absent")
	}
}

func TestClientCollector_WritebackAndState(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	osc := "scratch-OST0000-osc-ffff0001"
	mdc := "scratch-MDT0000-mdc-ffff0001"
	assertMetric(t, metrics, "lustre_osc_dirty_bytes", map[string]string{"component": "client", "target": osc}, 1048576)
	assertMetric(t, metrics, "lustre_osc_max_dirty_bytes", map[string]string{"component": "client", "target": osc}, 33554432)
	assertMetric(t, metrics, "lustre_max_rpcs_in_flight", map[string]string{"component": "client", "target": osc, "type": "osc"}, 8)
	assertMetric(t, metrics, "lustre_max_mod_rpcs_in_flight", map[string]string{"component": "client", "target": mdc, "type": "mdc"}, 16)
	assertMetric(t, metrics, "lustre_target_active", map[string]string{"component": "client", "target": osc, "type": "osc"}, 1)
	assertMetric(t, metrics, "lustre_target_state", map[string]string{"component": "client", "target": osc, "type": "osc", "state": "FULL"}, 1)
}

func TestClientCollector_MissingOptionalTunableIsNotStrictError(t *testing.T) {
	r := newTestClientFakeReader(t)
	delete(r.Files, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/cur_dirty_bytes")
	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
	if _, err := c.Collect(context.Background()); err != nil {
		t.Fatal(err)
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

// hasMetric reports whether metrics contains a series with exactly these labels.
func hasMetric(t *testing.T, metrics []prometheus.Metric, name string, labels map[string]string) bool {
	t.Helper()
	for _, metric := range metrics {
		if extractMetricName(metric.Desc().String()) != name {
			continue
		}
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			t.Fatal(err)
		}
		if labelsMatch(dm.GetLabel(), labels) {
			return true
		}
	}
	return false
}

func newDebugLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestClientCollector_RPCTargetSourceFailures(t *testing.T) {
	const osc = "scratch-OST0000-osc-ffff0001"
	const oscDir = "/proc/fs/lustre/osc/" + osc + "/"
	type series struct {
		name   string
		labels map[string]string
		value  float64
	}
	// One series per source of the osc target: stats, rpc_stats, a tunable and
	// the import state (read after the tunable).
	statsSum := series{"lustre_stats_seconds_sum", map[string]string{
		"component": "client", "target": osc, "type": "osc", "operation": "req_waittime"}, 0.5}
	rpcCurrent := series{"lustre_rpcs_current", map[string]string{
		"component": "client", "target": osc, "type": "osc", "operation": "write"}, 5}
	maxDirty := series{"lustre_osc_max_dirty_bytes", map[string]string{
		"component": "client", "target": osc}, 33554432}
	state := series{"lustre_target_state", map[string]string{
		"component": "client", "target": osc, "type": "osc", "state": "FULL"}, 1}
	errDenied := errors.New("permission denied")

	tests := []struct {
		name      string
		inject    func(r *reader.FakeReader)
		wantLog   string
		strictErr bool // whether strict mode fails the whole collection
		kept      []series
		dropped   series
	}{
		{
			name:      "stats read error",
			inject:    func(r *reader.FakeReader) { r.Errors[oscDir+"stats"] = errDenied },
			wantLog:   `level=WARN msg="obd stats read failed" component=osc target=` + osc + ` error="permission denied"`,
			strictErr: true,
			kept:      []series{rpcCurrent, maxDirty, state},
			dropped:   statsSum,
		},
		{
			name: "stats parse error",
			inject: func(r *reader.FakeReader) {
				r.Files[oscDir+"stats"] = []byte("req_waittime notanumber samples [usec] 1 2 3\n")
			},
			wantLog:   `level=WARN msg="failed to parse obd stats" component=osc target=` + osc,
			strictErr: true,
			kept:      []series{rpcCurrent, maxDirty, state},
			dropped:   statsSum,
		},
		{
			name:      "rpc_stats missing",
			inject:    func(r *reader.FakeReader) { delete(r.Files, oscDir+"rpc_stats") },
			wantLog:   `level=DEBUG msg="rpc_stats not found" component=osc target=` + osc + ` path=` + oscDir + `rpc_stats`,
			strictErr: false,
			kept:      []series{statsSum, maxDirty, state},
			dropped:   rpcCurrent,
		},
		{
			name:      "rpc_stats read error",
			inject:    func(r *reader.FakeReader) { r.Errors[oscDir+"rpc_stats"] = errDenied },
			wantLog:   `level=WARN msg="rpc_stats read failed" component=osc target=` + osc + ` error="permission denied"`,
			strictErr: true,
			kept:      []series{statsSum, maxDirty, state},
			dropped:   rpcCurrent,
		},
		{
			name: "rpc_stats parse error",
			inject: func(r *reader.FakeReader) {
				r.Files[oscDir+"rpc_stats"] = []byte("write RPCs in flight: 5\n\n" +
					"                        read            write\n" +
					"pages per rpc         rpcs   % cum %   rpcs   % cum %\n" +
					"1:                     abc  93  93   |  148562  92  92\n")
			},
			wantLog:   `level=WARN msg="failed to parse rpc stats" component=osc target=` + osc,
			strictErr: true,
			kept:      []series{statsSum, maxDirty, state},
			dropped:   rpcCurrent,
		},
		{
			name:      "param file parse error",
			inject:    func(r *reader.FakeReader) { r.Files[oscDir+"max_dirty_mb"] = []byte("garbage\n") },
			wantLog:   `level=WARN msg="failed to parse client param file" file=max_dirty_mb component=osc target=` + osc,
			strictErr: true,
			kept:      []series{statsSum, rpcCurrent, state},
			dropped:   maxDirty,
		},
		{
			name: "param file missing",
			inject: func(r *reader.FakeReader) {
				delete(r.Files, oscDir+"max_dirty_mb")
			},
			wantLog:   `level=DEBUG msg="client param file not found" file=max_dirty_mb component=osc target=` + osc,
			strictErr: false,
			kept:      []series{statsSum, rpcCurrent, state},
			dropped:   maxDirty,
		},
	}
	// assertSkipped checks that only the failed source is missing: the log
	// names it, its series is absent and the target's other sources are kept.
	assertSkipped := func(t *testing.T, metrics []prometheus.Metric, logs string, wantLog string, kept []series, dropped series) {
		t.Helper()
		if !strings.Contains(logs, wantLog) {
			t.Fatalf("log %q not found in:\n%s", wantLog, logs)
		}
		for _, s := range kept {
			assertMetric(t, metrics, s.name, s.labels, s.value)
		}
		if hasMetric(t, metrics, dropped.name, dropped.labels) {
			t.Fatalf("metric %s %v from the failed source is present", dropped.name, dropped.labels)
		}
	}
	for _, tt := range tests {
		t.Run(tt.name+"/non_strict", func(t *testing.T) {
			r := newTestClientFakeReader(t)
			tt.inject(r)
			var logs bytes.Buffer
			c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), newDebugLogger(&logs), false)
			metrics, err := c.Collect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			assertSkipped(t, metrics, logs.String(), tt.wantLog, tt.kept, tt.dropped)
		})
		t.Run(tt.name+"/strict", func(t *testing.T) {
			r := newTestClientFakeReader(t)
			tt.inject(r)
			var logs bytes.Buffer
			c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), newDebugLogger(&logs), true)
			metrics, err := c.Collect(context.Background())
			if tt.strictErr {
				if err == nil {
					t.Fatal("expected strict collection to fail")
				}
				if metrics != nil {
					t.Fatalf("got %d metrics with error", len(metrics))
				}
				if strings.Contains(logs.String(), "level=WARN") {
					t.Fatalf("strict mode logged a warning:\n%s", logs.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertSkipped(t, metrics, logs.String(), tt.wantLog, tt.kept, tt.dropped)
		})
	}
}

func TestClientCollector_StrictReturnsUnwrappedReadError(t *testing.T) {
	errDenied := errors.New("permission denied")
	for _, file := range []string{"stats", "rpc_stats", "max_dirty_mb"} {
		t.Run(file, func(t *testing.T) {
			r := newTestClientFakeReader(t)
			path := "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/" + file
			r.Errors[path] = errDenied
			c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
			_, err := c.Collect(context.Background())
			if file == "max_dirty_mb" {
				// An unreadable optional tunable is skipped even in strict mode.
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != errDenied {
				t.Fatalf("err = %v, want the read error itself", err)
			}
		})
	}
}

func TestClientCollector_LDLMCBDParseFailure(t *testing.T) {
	newReader := func(t *testing.T) *reader.FakeReader {
		r := newTestClientFakeReader(t)
		r.Files["/proc/fs/lustre/ldlm/services/ldlm_cbd/stats"] = []byte("ldlm_bl_callback xx samples [reqs]\n")
		return r
	}

	var logs bytes.Buffer
	c := NewClientCollectorWithStrict(newReader(t), discovery.DefaultPathConfig(), newDebugLogger(&logs), false)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `level=WARN msg="ldlm_cbd stats collection failed" error=`) {
		t.Fatalf("missing ldlm_cbd warning in:\n%s", logs.String())
	}
	assertMetric(t, metrics, "lustre_blocksize_bytes", map[string]string{
		"component": "client", "target": "scratch-ffff0001",
	}, 4194304)

	c = NewClientCollectorWithStrict(newReader(t), discovery.DefaultPathConfig(), newDebugLogger(&logs), true)
	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("expected strict collection to fail on malformed ldlm_cbd stats")
	}
}

func TestClientCollector_LLiteFailureSkipsOnlyThatTarget(t *testing.T) {
	r := newTestClientFakeReader(t)
	r.Errors["/proc/fs/lustre/llite/scratch-ffff0001/stats"] = errors.New("permission denied")

	var logs bytes.Buffer
	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), newDebugLogger(&logs), false)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := `level=WARN msg="llite collection failed" target=scratch-ffff0001 error="permission denied"`
	if !strings.Contains(logs.String(), want) {
		t.Fatalf("log %q not found in:\n%s", want, logs.String())
	}
	if hasMetric(t, metrics, "lustre_blocksize_bytes", map[string]string{"component": "client", "target": "scratch-ffff0001"}) {
		t.Fatal("llite metrics present although its stats read failed")
	}
	assertMetric(t, metrics, "lustre_stats_seconds_sum", map[string]string{
		"component": "client", "target": "scratch-OST0000-osc-ffff0001", "type": "osc", "operation": "req_waittime",
	}, 0.5)
}

func TestClientCollector_StrictReturnsFirstErrorInCollectionOrder(t *testing.T) {
	const (
		mdcDir = "/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/"
		oscDir = "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/"
	)
	badStats := []byte("req_waittime notanumber samples [usec] 1 2 3\n")
	_, parseErr := parser.ParseOBDStats(badStats, oscDir+"stats", "client", "scratch-OST0000-osc-ffff0001", "osc")
	if parseErr == nil {
		t.Fatal("malformed stats fixture parsed without error")
	}
	errFirst := errors.New("first failure")
	errSecond := errors.New("second failure")

	tests := []struct {
		name   string
		inject func(r *reader.FakeReader)
		want   error
	}{
		{
			// Within a target, stats are collected before rpc_stats.
			name: "stats read error before rpc_stats read error",
			inject: func(r *reader.FakeReader) {
				r.Errors[oscDir+"stats"] = errFirst
				r.Errors[oscDir+"rpc_stats"] = errSecond
			},
			want: errFirst,
		},
		{
			name: "stats parse error before rpc_stats read error",
			inject: func(r *reader.FakeReader) {
				r.Files[oscDir+"stats"] = badStats
				r.Errors[oscDir+"rpc_stats"] = errSecond
			},
			want: parseErr,
		},
		{
			// Targets are collected in discovery order (mdc before osc) and
			// the first failure ends the scrape.
			name: "earlier target fails first",
			inject: func(r *reader.FakeReader) {
				r.Errors[mdcDir+"rpc_stats"] = errFirst
				r.Errors[oscDir+"stats"] = errSecond
			},
			want: errFirst,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestClientFakeReader(t)
			tt.inject(r)
			c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
			_, err := c.Collect(context.Background())
			// Injected read errors must come back as the same value. The parse
			// error is built by the parser, so it is compared by value: equal
			// only if it was returned unwrapped.
			if err != tt.want && !reflect.DeepEqual(err, tt.want) {
				t.Fatalf("err = %v (%T), want %v (%T)", err, err, tt.want, tt.want)
			}
		})
	}
}
