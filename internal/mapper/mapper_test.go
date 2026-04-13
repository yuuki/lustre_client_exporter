package mapper

import (
	"testing"

	"github.com/yuuki/lustre_exporter/internal/parser"
)

func TestMap_Health(t *testing.T) {
	obs := []parser.Observation{
		{
			Collector:  "health",
			Source:     "/sys/fs/lustre/health_check",
			MetricID:   "health_check",
			MetricType: parser.Gauge,
			Value:      1.0,
		},
	}

	mapped, err := Map(obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapped) != 1 {
		t.Fatalf("got %d mapped, want 1", len(mapped))
	}
	if mapped[0].Def.Name != "lustre_health_check" {
		t.Errorf("got name %q, want %q", mapped[0].Def.Name, "lustre_health_check")
	}
	if mapped[0].Value != 1.0 {
		t.Errorf("got value %f, want 1.0", mapped[0].Value)
	}
}

func TestMap_UnknownMetricID(t *testing.T) {
	obs := []parser.Observation{
		{MetricID: "nonexistent"},
	}
	_, err := Map(obs)
	if err == nil {
		t.Fatal("expected error for unknown metric ID")
	}
}
