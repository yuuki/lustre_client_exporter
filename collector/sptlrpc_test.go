package collector

import (
	"context"
	"os"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

func TestSptlrpcCollector(t *testing.T) {
	data, err := os.ReadFile("../testdata/sptlrpc/encrypt_page_pools.txt")
	if err != nil {
		t.Fatal(err)
	}

	r := reader.NewFakeReader()
	r.Files["/sys/kernel/debug/lustre/sptlrpc/encrypt_page_pools"] = data

	c := NewSptlrpcCollector(r, discovery.DefaultPathConfig())
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(metrics) != 15 {
		t.Fatalf("got %d metrics, want 15", len(metrics))
	}

	// Spot check a few values
	found := map[string]float64{}
	for _, m := range metrics {
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
		desc := m.Desc().String()
		if dm.GetGauge() != nil {
			found[desc] = dm.GetGauge().GetValue()
		} else if dm.GetCounter() != nil {
			found[desc] = dm.GetCounter().GetValue()
		}
	}

	if len(found) != 15 {
		t.Errorf("got %d unique metrics, want 15", len(found))
	}
}

func TestSptlrpcCollector_MissingFile(t *testing.T) {
	r := reader.NewFakeReader()
	c := NewSptlrpcCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for missing encrypt_page_pools")
	}
}
