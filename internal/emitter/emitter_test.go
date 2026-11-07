package emitter

import (
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_client_exporter/internal/mapper"
	"github.com/yuuki/lustre_client_exporter/internal/parser"
)

func TestEmit_HealthGauge(t *testing.T) {
	mapped := []mapper.MappedObservation{
		{
			Def: mapper.MetricDef{
				Name: "lustre_health_check",
				Help: "Lustre filesystem health status (1 = healthy, 0 = unhealthy).",
				Type: parser.Gauge,
			},
			LabelVals: nil,
			Value:     1.0,
		},
	}

	metrics, err := Emit(mapped)
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
		t.Errorf("got value %f, want 1.0", m.GetGauge().GetValue())
	}
}

func TestEmit_Counter(t *testing.T) {
	mapped := []mapper.MappedObservation{
		{
			Def: mapper.MetricDef{
				Name:      "lustre_read_samples_total",
				Help:      "Total number of read samples.",
				Type:      parser.Counter,
				LabelKeys: []string{"component", "target"},
			},
			LabelVals: []string{"client", "scratch-ffff0001"},
			Value:     42.0,
		},
	}

	metrics, err := Emit(mapped)
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
	if m.GetCounter().GetValue() != 42.0 {
		t.Errorf("got value %f, want 42.0", m.GetCounter().GetValue())
	}

	// Verify labels
	labels := m.GetLabel()
	if len(labels) != 2 {
		t.Fatalf("got %d labels, want 2", len(labels))
	}
	found := map[string]string{}
	for _, l := range labels {
		found[l.GetName()] = l.GetValue()
	}
	if found["component"] != "client" {
		t.Errorf("component=%q, want %q", found["component"], "client")
	}
	if found["target"] != "scratch-ffff0001" {
		t.Errorf("target=%q, want %q", found["target"], "scratch-ffff0001")
	}
}

// A label value that is not valid UTF-8 can come from a corrupt source file.
// It must fail Emit with an error, not panic: Emit runs inside the collector
// goroutines of a scrape, where a panic crashes the exporter.
func TestEmit_InvalidUTF8LabelValueReturnsError(t *testing.T) {
	def := mapper.MetricDef{
		Name:      "lustre_stats_total",
		Help:      "Number of operations.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target", "operation"},
	}
	mapped := []mapper.MappedObservation{
		{Def: def, LabelVals: []string{"client", "scratch-ffff0001", "open"}, Value: 1},
		{Def: def, LabelVals: []string{"client", "scratch-ffff0001", "op\xff"}, Value: 2},
	}

	metrics, err := Emit(mapped)
	if err == nil {
		t.Fatal("expected an error for a label value that is not valid UTF-8")
	}
	if !strings.Contains(err.Error(), "lustre_stats_total") {
		t.Errorf("error %q does not name the metric", err)
	}
	if metrics != nil {
		t.Errorf("got %d metrics alongside the error, want nil", len(metrics))
	}
}
