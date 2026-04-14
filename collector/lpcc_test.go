package collector

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
)

func newTestLpccFakeReader(t *testing.T) *reader.FakeReader {
	t.Helper()
	r := reader.NewFakeReader()

	data, err := os.ReadFile("../testdata/lpcc/status.json")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lpcc status"] = data
	return r
}

func TestLpccCollector_Success(t *testing.T) {
	r := newTestLpccFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c := NewLpccCollector(r, "lpcc", logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 2 mounts × (17 per-cache + 6 per-mount) = 46
	if len(metrics) != 46 {
		t.Fatalf("expected 46 metrics, got %d", len(metrics))
	}

	// Verify a specific metric value
	for _, m := range metrics {
		d := &dto.Metric{}
		if err := m.Write(d); err != nil {
			t.Fatal(err)
		}
		desc := m.Desc().String()
		// Check lustre_pcc_cached_files for /disk/dn-h
		if strings.Contains(desc, "lustre_pcc_cached_files") {
			for _, lp := range d.GetLabel() {
				if lp.GetName() == "mount" && lp.GetValue() == "/disk/dn-h" {
					if got := d.GetGauge().GetValue(); got != 1318 {
						t.Errorf("lustre_pcc_cached_files for /disk/dn-h: expected 1318, got %f", got)
					}
				}
			}
		}
	}
}

func TestLpccCollector_CommandNotFound(t *testing.T) {
	r := reader.NewFakeReader()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c := NewLpccCollector(r, "lpcc", logger)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error on command failure")
	}
}

func TestLpccCollector_MalformedJSON(t *testing.T) {
	r := reader.NewFakeReader()
	r.Commands["lpcc status"] = []byte("not json")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c := NewLpccCollector(r, "lpcc", logger)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
