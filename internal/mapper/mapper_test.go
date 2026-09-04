package mapper

import (
	"strings"
	"testing"

	"github.com/yuuki/lustre_client_exporter/internal/parser"
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

func TestMap_StatsSecondsSum(t *testing.T) {
	obs := []parser.Observation{
		{
			MetricID:   "stats_seconds_sum",
			MetricType: parser.Counter,
			Labels: map[string]string{
				"component": "client",
				"target":    "scratch-OST0000-osc-ffff0001",
				"type":      "osc",
				"operation": "req_waittime",
			},
			Value: 0.5,
		},
	}
	mapped, err := Map(obs)
	if err != nil {
		t.Fatal(err)
	}
	if mapped[0].Def.Name != "lustre_stats_seconds_sum" {
		t.Fatalf("name = %q", mapped[0].Def.Name)
	}
	if got := strings.Join(mapped[0].Def.LabelKeys, ","); got != "component,target,type,operation" {
		t.Fatalf("labels = %q", got)
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
