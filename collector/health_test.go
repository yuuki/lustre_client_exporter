package collector

import (
	"context"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

func TestHealthCollector_Healthy(t *testing.T) {
	r := reader.NewFakeReader()
	r.Files["/sys/fs/lustre/health_check"] = []byte("healthy\n")

	c := NewHealthCollector(r, discovery.DefaultPathConfig())
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}

	var m dto.Metric
	if err := metrics[0].Write(&m); err != nil {
		t.Fatal(err)
	}
	if m.GetGauge().GetValue() != 1.0 {
		t.Errorf("health value = %f, want 1.0", m.GetGauge().GetValue())
	}
}

func TestHealthCollector_Unhealthy(t *testing.T) {
	r := reader.NewFakeReader()
	r.Files["/sys/fs/lustre/health_check"] = []byte("NOT HEALTHY\n")

	c := NewHealthCollector(r, discovery.DefaultPathConfig())
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}

	var m dto.Metric
	if err := metrics[0].Write(&m); err != nil {
		t.Fatal(err)
	}
	if m.GetGauge().GetValue() != 0.0 {
		t.Errorf("health value = %f, want 0.0", m.GetGauge().GetValue())
	}
}

func TestHealthCollector_MissingFile(t *testing.T) {
	r := reader.NewFakeReader()

	c := NewHealthCollector(r, discovery.DefaultPathConfig())
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for missing health_check file")
	}
}
