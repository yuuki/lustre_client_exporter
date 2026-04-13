package collector

import (
	"context"
	"log/slog"
	"os"
	"testing"

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
		"/proc/fs/lustre/osc/lfs-dn-h-OST0000-osc-ff36f2b293cbd800/rpc_stats",
	}
	loadFixture(t, r, "/proc/fs/lustre/osc/lfs-dn-h-OST0000-osc-ff36f2b293cbd800/rpc_stats", "../testdata/osc/rpc_stats.txt")

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
