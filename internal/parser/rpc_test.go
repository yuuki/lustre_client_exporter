package parser

import (
	"os"
	"testing"
)

func TestParseRPCStats_MDC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/mdc/rpc_stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseRPCStats(data, "test", "mdc", "scratch-MDT0000-mdc-ffff0001", "mdc")
	if err != nil {
		t.Fatal(err)
	}

	// 3 sections * 4 rows * 2 (read+write) = 24
	if len(obs) != 24 {
		t.Fatalf("got %d observations, want 24", len(obs))
	}

	// Check that components and targets are set
	for _, o := range obs {
		if o.Labels["component"] != "mdc" {
			t.Errorf("component = %q, want mdc", o.Labels["component"])
		}
		if o.Labels["target"] != "scratch-MDT0000-mdc-ffff0001" {
			t.Errorf("target = %q", o.Labels["target"])
		}
	}

	// Check rpcs_in_flight has type label
	for _, o := range obs {
		if o.MetricID == "rpcs_in_flight" {
			if o.Labels["type"] != "mdc" {
				t.Errorf("rpcs_in_flight type = %q, want mdc", o.Labels["type"])
			}
		}
	}
}

func TestParseRPCStats_OSC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/rpc_stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseRPCStats(data, "test", "osc", "scratch-OST0000-osc-ffff0001", "osc")
	if err != nil {
		t.Fatal(err)
	}

	// 3 sections * 5 rows * 2 = 30
	// Actually: pages_per_rpc (5 rows * 2) + rpcs_in_flight (4 rows * 2) + offset (4 rows * 2) = 10 + 8 + 8 = 26
	if len(obs) != 26 {
		t.Fatalf("got %d observations, want 26", len(obs))
	}
}

func TestParseRPCStats_Empty(t *testing.T) {
	obs, err := ParseRPCStats([]byte(""), "test", "osc", "target", "osc")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("got %d for empty, want 0", len(obs))
	}
}

func TestParseRPCStats_RpcsInFlightReadOnlyLine(t *testing.T) {
	data := []byte(`
rpcs in flight        rpcs   % cum %
0:                         0   0   0
`)

	obs, err := ParseRPCStats(data, "test", "mdc", "target", "mdc")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}

	got := obs[0]
	if got.MetricID != "rpcs_in_flight" {
		t.Errorf("MetricID = %q, want rpcs_in_flight", got.MetricID)
	}
	if got.Labels["operation"] != "read" {
		t.Errorf("operation = %q, want read", got.Labels["operation"])
	}
	if got.Labels["size"] != "0" {
		t.Errorf("size = %q, want 0", got.Labels["size"])
	}
	if got.Labels["type"] != "mdc" {
		t.Errorf("type = %q, want mdc", got.Labels["type"])
	}
	if got.Value != 0 {
		t.Errorf("Value = %v, want 0", got.Value)
	}
}

func TestParseRPCStats_MalformedDataLineReturnsError(t *testing.T) {
	data := []byte(`
pages per rpc         rpcs   % cum %   rpcs   % cum %
1:                   not_a_number  93  93   |     148562  92  92
`)

	_, err := ParseRPCStats(data, "test", "osc", "target", "osc")
	if err == nil {
		t.Fatal("expected error for malformed data line")
	}
}
