package collector

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

// expectedMetricNames lists all metric names from the GSI-HPC compatible contract.
var expectedMetricNames = map[string]bool{
	// Health
	"lustre_health_check": true,

	// SPTLRPC
	"lustre_physical_pages":               true,
	"lustre_pages_per_pool":               true,
	"lustre_maximum_pages":                true,
	"lustre_maximum_pools":                true,
	"lustre_pages_in_pools":               true,
	"lustre_free_pages":                   true,
	"lustre_maximum_pages_reached_total":  true,
	"lustre_grows_total":                  true,
	"lustre_grows_failure_total":          true,
	"lustre_shrinks_total":                true,
	"lustre_cache_access_total":           true,
	"lustre_cache_miss_total":             true,
	"lustre_free_page_low":                true,
	"lustre_maximum_waitqueue_depth":      true,
	"lustre_out_of_memory_request_total":  true,

	// LNet stats
	"lustre_allocated":           true,
	"lustre_maximum":             true,
	"lustre_errors_total":        true,
	"lustre_send_count_total":    true,
	"lustre_receive_count_total": true,
	"lustre_route_count_total":   true,
	"lustre_drop_count_total":    true,
	"lustre_send_bytes_total":    true,
	"lustre_receive_bytes_total": true,
	"lustre_route_bytes_total":   true,
	"lustre_drop_bytes_total":    true,

	// LNet params
	"lustre_console_backoff_enabled":          true,
	"lustre_console_max_delay_centiseconds":   true,
	"lustre_console_min_delay_centiseconds":   true,
	"lustre_console_ratelimit_enabled":        true,
	"lustre_debug_megabytes":                  true,
	"lustre_panic_on_lbug_enabled":            true,
	"lustre_watchdog_ratelimit_enabled":       true,
	"lustre_catastrophe_enabled":              true,
	"lustre_lnet_memory_used_bytes":           true,

	// Client core
	"lustre_blocksize_bytes":        true,
	"lustre_inodes_free":            true,
	"lustre_inodes_maximum":         true,
	"lustre_available_kibibytes":    true,
	"lustre_free_kibibytes":         true,
	"lustre_capacity_kibibytes":     true,
	"lustre_read_samples_total":     true,
	"lustre_read_minimum_size_bytes": true,
	"lustre_read_maximum_size_bytes": true,
	"lustre_read_bytes_total":       true,
	"lustre_write_samples_total":    true,
	"lustre_write_minimum_size_bytes": true,
	"lustre_write_maximum_size_bytes": true,
	"lustre_write_bytes_total":      true,
	"lustre_stats_total":            true,

	// Client tunables
	"lustre_checksum_pages_enabled":                  true,
	"lustre_default_ea_size_bytes":                   true,
	"lustre_lazystatfs_enabled":                      true,
	"lustre_maximum_ea_size_bytes":                   true,
	"lustre_maximum_read_ahead_megabytes":            true,
	"lustre_maximum_read_ahead_per_file_megabytes":   true,
	"lustre_maximum_read_ahead_whole_megabytes":      true,
	"lustre_statahead_agl_enabled":                   true,
	"lustre_statahead_maximum":                       true,
	"lustre_xattr_cache_enabled":                     true,

	// RPC
	"lustre_pages_per_rpc_total": true,
	"lustre_rpcs_in_flight":     true,
	"lustre_rpcs_offset":        true,
}

// excludedMetricNames lists metrics we explicitly do NOT emit.
var excludedMetricNames = []string{
	"lustre_available_kilobytes",
	"lustre_free_kilobytes",
	"lustre_capacity_kilobytes",
	"lustre_health_healthy",
	"lustre_lnet_mem_used",
	"target_info",
	"lustre_job_read_bytes_total",
	"lustre_job_write_bytes_total",
}

func TestContract_AllExpectedMetricsPresent(t *testing.T) {
	r := newFullFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	collectors := []Collector{
		NewHealthCollector(r, discovery.DefaultPathConfig()),
		NewSptlrpcCollector(r, discovery.DefaultPathConfig()),
		NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger),
		NewClientCollector(r, discovery.DefaultPathConfig(), logger),
	}

	foundNames := map[string]bool{}

	for _, c := range collectors {
		metrics, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("collector %s: %v", c.Name(), err)
		}
		for _, m := range metrics {
			desc := m.Desc().String()
			// Extract metric name from desc string
			name := extractMetricName(desc)
			foundNames[name] = true
		}
	}

	for name := range expectedMetricNames {
		if !foundNames[name] {
			t.Errorf("expected metric %q not found in output", name)
		}
	}
}

func TestContract_ExcludedMetricsAbsent(t *testing.T) {
	r := newFullFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	collectors := []Collector{
		NewHealthCollector(r, discovery.DefaultPathConfig()),
		NewSptlrpcCollector(r, discovery.DefaultPathConfig()),
		NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger),
		NewClientCollector(r, discovery.DefaultPathConfig(), logger),
	}

	foundNames := map[string]bool{}
	for _, c := range collectors {
		metrics, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("collector %s: %v", c.Name(), err)
		}
		for _, m := range metrics {
			name := extractMetricName(m.Desc().String())
			foundNames[name] = true
		}
	}

	for _, name := range excludedMetricNames {
		if foundNames[name] {
			t.Errorf("excluded metric %q found in output", name)
		}
	}
}

func TestContract_AllMetricsHaveLustrePrefix(t *testing.T) {
	r := newFullFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	collectors := []Collector{
		NewHealthCollector(r, discovery.DefaultPathConfig()),
		NewSptlrpcCollector(r, discovery.DefaultPathConfig()),
		NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger),
		NewClientCollector(r, discovery.DefaultPathConfig(), logger),
	}

	for _, c := range collectors {
		metrics, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("collector %s: %v", c.Name(), err)
		}
		for _, m := range metrics {
			name := extractMetricName(m.Desc().String())
			if !strings.HasPrefix(name, "lustre_") {
				t.Errorf("metric %q does not have lustre_ prefix", name)
			}
		}
	}
}

func TestContract_MetricTypes(t *testing.T) {
	r := newFullFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	collectors := []Collector{
		NewHealthCollector(r, discovery.DefaultPathConfig()),
		NewSptlrpcCollector(r, discovery.DefaultPathConfig()),
		NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", logger),
		NewClientCollector(r, discovery.DefaultPathConfig(), logger),
	}

	for _, c := range collectors {
		metrics, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("collector %s: %v", c.Name(), err)
		}
		for _, m := range metrics {
			var dm dto.Metric
			if err := m.Write(&dm); err != nil {
				t.Fatal(err)
			}
			name := extractMetricName(m.Desc().String())
			isCounter := strings.HasSuffix(name, "_total")

			if isCounter && dm.GetCounter() == nil {
				t.Errorf("metric %q ends in _total but is not a counter type", name)
			}
		}
	}
}

// newFullFakeReader creates a FakeReader loaded with all test fixtures.
func newFullFakeReader(t *testing.T) *reader.FakeReader {
	t.Helper()
	r := newTestClientFakeReader(t)

	// Health
	r.Files["/sys/fs/lustre/health_check"] = []byte("healthy\n")

	// SPTLRPC
	loadFixture(t, r, "/sys/kernel/debug/lustre/sptlrpc/encrypt_page_pools", "../testdata/sptlrpc/encrypt_page_pools.txt")

	// LNet stats
	loadFixture(t, r, "/proc/sys/lnet/stats", "../testdata/lnet/stats.txt")

	// LNet params
	lnetParams := []string{
		"console_backoff", "console_max_delay_centisecs", "console_min_delay_centisecs",
		"console_ratelimit", "debug_mb", "panic_on_lbug", "watchdog_ratelimit",
		"catastrophe", "lnet_memused",
	}
	for _, name := range lnetParams {
		loadFixture(t, r, "/proc/sys/lnet/"+name, "../testdata/lnet/params/"+name)
	}

	return r
}

// extractMetricName pulls the fqName from a Desc string.
// Desc format: Desc{fqName: "lustre_health_check", help: "...", ...}
func extractMetricName(desc string) string {
	start := strings.Index(desc, `fqName: "`)
	if start < 0 {
		return ""
	}
	start += len(`fqName: "`)
	end := strings.Index(desc[start:], `"`)
	if end < 0 {
		return ""
	}
	return desc[start : start+end]
}
