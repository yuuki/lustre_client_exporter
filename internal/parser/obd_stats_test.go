package parser

import (
	"os"
	"testing"
)

func TestParseOBDStats_OSC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseOBDStats(data, "test", "client", "scratch-OST0000-osc-ffff0001", "osc")
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]float64{}
	seconds := map[string]float64{}
	var writeBytes float64
	for _, o := range obs {
		switch o.MetricID {
		case "stats_total":
			counts[o.Labels["operation"]] = o.Value
			if _, ok := o.Labels["type"]; ok {
				t.Fatalf("stats_total must not have type label, got %v", o.Labels)
			}
		case "stats_seconds_sum":
			seconds[o.Labels["operation"]] = o.Value
			if o.Labels["type"] != "osc" {
				t.Fatalf("stats_seconds_sum type = %q", o.Labels["type"])
			}
			if o.MetricType != Counter {
				t.Fatalf("stats_seconds_sum type = %v, want Counter", o.MetricType)
			}
		case "write_bytes_total":
			writeBytes = o.Value
		}
		if o.Labels["component"] != "client" {
			t.Fatalf("component = %q", o.Labels["component"])
		}
	}

	if counts["req_waittime"] != 200 {
		t.Errorf("req_waittime count = %v, want 200", counts["req_waittime"])
	}
	if counts["req_qdepth"] != 200 {
		t.Errorf("req_qdepth count = %v, want 200", counts["req_qdepth"])
	}
	if _, ok := seconds["req_qdepth"]; ok {
		t.Fatal("req_qdepth must not emit seconds_sum")
	}
	if seconds["req_waittime"] != 0.5 {
		t.Errorf("req_waittime seconds_sum = %v, want 0.5", seconds["req_waittime"])
	}
	if seconds["req_timeout"] != 2000 {
		t.Errorf("req_timeout seconds_sum = %v, want 2000", seconds["req_timeout"])
	}
	if seconds["obd_ping"] != 0.025 {
		t.Errorf("obd_ping seconds_sum = %v, want 0.025", seconds["obd_ping"])
	}
	if writeBytes != 838860800 {
		t.Errorf("write_bytes_total = %v, want 838860800", writeBytes)
	}
	if _, ok := counts["write_bytes"]; ok {
		t.Fatal("write_bytes must not emit stats_total")
	}
}

func TestParseOBDStats_MDC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/mdc/stats.txt")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseOBDStats(data, "test", "client", "scratch-MDT0000-mdc-ffff0001", "mdc")
	if err != nil {
		t.Fatal(err)
	}
	seconds := map[string]float64{}
	for _, o := range obs {
		if o.MetricID == "stats_seconds_sum" {
			seconds[o.Labels["operation"]] = o.Value
			if o.Labels["type"] != "mdc" {
				t.Fatalf("type = %q", o.Labels["type"])
			}
		}
	}
	if seconds["mds_getattr"] != 0.1 {
		t.Errorf("mds_getattr seconds_sum = %v, want 0.1", seconds["mds_getattr"])
	}
}

func TestParseOBDStats_Empty(t *testing.T) {
	obs, err := ParseOBDStats([]byte(""), "test", "client", "t", "osc")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Fatalf("got %d, want 0", len(obs))
	}
}

func TestParseOBDStats_MalformedCount(t *testing.T) {
	_, err := ParseOBDStats([]byte("req_waittime not_a_number samples [usec] 1 2 3\n"), "test", "client", "t", "osc")
	if err == nil {
		t.Fatal("expected error for malformed count")
	}
}
