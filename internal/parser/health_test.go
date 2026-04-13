package parser

import "testing"

func TestParseHealth_Healthy(t *testing.T) {
	obs, err := ParseHealth([]byte("healthy\n"), "/sys/fs/lustre/health_check")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}
	if obs[0].Value != 1.0 {
		t.Errorf("got value %f, want 1.0", obs[0].Value)
	}
	if obs[0].MetricID != "health_check" {
		t.Errorf("got metric ID %q, want %q", obs[0].MetricID, "health_check")
	}
	if obs[0].Collector != "health" {
		t.Errorf("got collector %q, want %q", obs[0].Collector, "health")
	}
}

func TestParseHealth_Unhealthy(t *testing.T) {
	obs, err := ParseHealth([]byte("NOT HEALTHY\n"), "/sys/fs/lustre/health_check")
	if err != nil {
		t.Fatal(err)
	}
	if obs[0].Value != 0.0 {
		t.Errorf("got value %f, want 0.0", obs[0].Value)
	}
}

func TestParseHealth_Empty(t *testing.T) {
	obs, err := ParseHealth([]byte("\n"), "/sys/fs/lustre/health_check")
	if err != nil {
		t.Fatal(err)
	}
	if obs[0].Value != 0.0 {
		t.Errorf("got value %f, want 0.0 for empty input", obs[0].Value)
	}
}

func TestParseHealth_NoNewline(t *testing.T) {
	obs, err := ParseHealth([]byte("healthy"), "/sys/fs/lustre/health_check")
	if err != nil {
		t.Fatal(err)
	}
	if obs[0].Value != 1.0 {
		t.Errorf("got value %f, want 1.0", obs[0].Value)
	}
}
