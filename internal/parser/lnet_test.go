package parser

import (
	"os"
	"testing"
)

func TestParseLNetStats(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLNetStats(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 11 {
		t.Fatalf("got %d observations, want 11", len(obs))
	}

	expected := map[string]float64{
		"allocated":           0,
		"maximum":             32,
		"errors_total":        0,
		"send_count_total":    512,
		"receive_count_total": 256,
		"route_count_total":   64,
		"drop_count_total":    128,
		"send_bytes_total":    1048576,
		"receive_bytes_total": 524288,
		"route_bytes_total":   65536,
		"drop_bytes_total":    131072,
	}

	for _, o := range obs {
		want, ok := expected[o.MetricID]
		if !ok {
			t.Errorf("unexpected metric %q", o.MetricID)
			continue
		}
		if o.Value != want {
			t.Errorf("%s = %f, want %f", o.MetricID, o.Value, want)
		}
	}
}

func TestParseLNetStats_Empty(t *testing.T) {
	obs, err := ParseLNetStats([]byte(""), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("got %d for empty, want 0", len(obs))
	}
}

func TestParseLNetStats_WrongFieldCount(t *testing.T) {
	_, err := ParseLNetStats([]byte("1 2 3"), "test")
	if err == nil {
		t.Fatal("expected error for wrong field count")
	}
}

func TestParseLNetCtlStats(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_stats.json")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLNetCtlStats(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 11 {
		t.Fatalf("got %d observations, want 11", len(obs))
	}

	found := map[string]float64{}
	for _, o := range obs {
		found[o.MetricID] = o.Value
	}

	if found["send_count_total"] != 512 {
		t.Errorf("send_count_total = %f, want 512", found["send_count_total"])
	}
	if found["drop_bytes_total"] != 131072 {
		t.Errorf("drop_bytes_total = %f, want 131072", found["drop_bytes_total"])
	}
}

func TestParseLNetCtlStats_YAML(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_stats.yaml")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLNetCtlStats(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 11 {
		t.Fatalf("got %d observations, want 11", len(obs))
	}

	found := map[string]float64{}
	for _, o := range obs {
		found[o.MetricID] = o.Value
	}

	if found["send_count_total"] != 512 {
		t.Errorf("send_count_total = %f, want 512", found["send_count_total"])
	}
	if found["drop_bytes_total"] != 131072 {
		t.Errorf("drop_bytes_total = %f, want 131072", found["drop_bytes_total"])
	}
}

func TestParseLNetCtlNetStats_YAML(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_net_show.yaml")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLNetCtlNetStats(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 6 {
		t.Fatalf("got %d observations, want 6", len(obs))
	}

	found := map[string]float64{}
	for _, o := range obs {
		found[o.MetricID+"/"+o.Labels["nid"]] = o.Value
	}

	if found["send_count_by_nid_total/0@lo"] != 180076 {
		t.Errorf("send_count_by_nid_total/0@lo = %f, want 180076", found["send_count_by_nid_total/0@lo"])
	}
	if found["receive_count_by_nid_total/172.16.0.24@tcp"] != 464963 {
		t.Errorf("receive_count_by_nid_total/172.16.0.24@tcp = %f, want 464963", found["receive_count_by_nid_total/172.16.0.24@tcp"])
	}
	if found["drop_count_by_nid_total/172.16.0.24@tcp"] != 4 {
		t.Errorf("drop_count_by_nid_total/172.16.0.24@tcp = %f, want 4", found["drop_count_by_nid_total/172.16.0.24@tcp"])
	}
}

func TestParseLNetParam(t *testing.T) {
	tests := []struct {
		param    string
		data     string
		metricID string
		value    float64
	}{
		{"console_backoff", "1\n", "console_backoff_enabled", 1},
		{"debug_mb", "64\n", "debug_megabytes", 64},
		{"catastrophe", "0\n", "catastrophe_enabled", 0},
		{"lnet_memused", "4194304\n", "lnet_memory_used_bytes", 4194304},
		{"fail_val", "7\n", "fail_maximum", 7},
	}

	for _, tt := range tests {
		obs, err := ParseLNetParam([]byte(tt.data), "test", tt.param)
		if err != nil {
			t.Fatalf("%s: %v", tt.param, err)
		}
		if len(obs) != 1 {
			t.Fatalf("%s: got %d obs, want 1", tt.param, len(obs))
		}
		if obs[0].MetricID != tt.metricID {
			t.Errorf("%s: metricID = %q, want %q", tt.param, obs[0].MetricID, tt.metricID)
		}
		if obs[0].Value != tt.value {
			t.Errorf("%s: value = %f, want %f", tt.param, obs[0].Value, tt.value)
		}
	}
}

func TestParseLNetParam_Unknown(t *testing.T) {
	obs, err := ParseLNetParam([]byte("1\n"), "test", "unknown_param")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("got %d obs for unknown param, want 0", len(obs))
	}
}
