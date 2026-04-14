package parser

import (
	"os"
	"testing"
)

func TestParseLDLMCBDStats(t *testing.T) {
	data, err := os.ReadFile("../../testdata/ldlm/ldlm_cbd_stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLDLMCBDStats(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 8 {
		t.Fatalf("got %d observations, want 8", len(obs))
	}

	for _, o := range obs {
		if o.Collector != "client" {
			t.Errorf("collector = %q, want client", o.Collector)
		}
		if o.MetricID != "ldlm_cbd_stats" {
			t.Errorf("metric ID = %q, want ldlm_cbd_stats", o.MetricID)
		}
		if o.MetricType != Counter {
			t.Errorf("metric type = %v, want Counter", o.MetricType)
		}
		if o.Labels["operation"] == "" {
			t.Error("operation label is empty")
		}
	}

	assertObservation(t, obs, "ldlm_bl_callback", 1236010)
	assertObservation(t, obs, "ldlm_cp_callback", 421007)
	assertObservation(t, obs, "ldlm_gl_callback", 99914)
	assertObservation(t, obs, "reqbuf_avail", 3575360)
}

func TestParseLDLMCBDStatsSkipsUnknownLines(t *testing.T) {
	data := []byte(`
snapshot_time             1710759783.270541554 secs.nsecs
unexpected_header         added by a future kernel
req_waittime              1756931 samples [usecs] 1 33890 255153949 6336801119
`)

	obs, err := ParseLDLMCBDStats(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}
	assertObservation(t, obs, "req_waittime", 1756931)
}

func assertObservation(t *testing.T, obs []Observation, operation string, value float64) {
	t.Helper()
	for _, o := range obs {
		if o.Labels["operation"] == operation {
			if o.Value != value {
				t.Fatalf("%s value = %v, want %v", operation, o.Value, value)
			}
			return
		}
	}
	t.Fatalf("operation %q not found", operation)
}
