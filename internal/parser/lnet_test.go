package parser

import (
	"fmt"
	"os"
	"strings"
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

func TestParseLNetCtlNetNI_Verbose(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_net_show_verbose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseLNetCtlNetNI(data, "test")
	if err != nil {
		t.Fatal(err)
	}
	up := map[string]float64{}
	health := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "lnet_ni_up":
			up[o.Labels["nid"]] = o.Value
		case "lnet_ni_health":
			health[o.Labels["nid"]] = o.Value
		}
	}
	if up["0@lo"] != 1 || up["10.200.200.54@o2ib"] != 0 {
		t.Fatalf("up = %v", up)
	}
	if health["0@lo"] != 0 || health["10.200.200.54@o2ib"] != 800 {
		t.Fatalf("health = %v", health)
	}
	wantCounters := map[string]float64{
		"lnet_ni_health_interrupts_total": 1,
		"lnet_ni_health_dropped_total":    136,
		"lnet_ni_health_aborted_total":    2,
		"lnet_ni_health_no_route_total":   3,
		"lnet_ni_health_timeouts_total":   4,
		"lnet_ni_health_errors_total":     5,
	}
	for id, want := range wantCounters {
		var got float64
		var found bool
		for _, o := range obs {
			if o.MetricID == id && o.Labels["nid"] == "10.200.200.54@o2ib" {
				got = o.Value
				found = true
			}
		}
		if !found || got != want {
			t.Fatalf("%s = %v found=%v want %v", id, got, found, want)
		}
	}
}

func TestParseLNetCtlNetNI_StatusWithoutHealth(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_net_show.yaml")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseLNetCtlNetNI(data, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("got %d, want 2 up gauges", len(obs))
	}
	for _, o := range obs {
		if o.MetricID != "lnet_ni_up" {
			t.Fatalf("unexpected %s", o.MetricID)
		}
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

func lnetNIObs(metricID string, mt MetricType, nid string, v float64) Observation {
	return Observation{Collector: "lnet", Source: "src", MetricID: metricID, MetricType: mt,
		Labels: map[string]string{"component": "lnet", "target": "lnet", "nid": nid}, Value: v}
}

// lnetNIHealth returns the health series of one NI in emission order, with
// values base, base+1, ... so that a swapped field shows up.
func lnetNIHealth(nid string, base float64) []Observation {
	return []Observation{
		lnetNIObs("lnet_ni_health", Gauge, nid, base),
		lnetNIObs("lnet_ni_health_interrupts_total", Counter, nid, base+1),
		lnetNIObs("lnet_ni_health_dropped_total", Counter, nid, base+2),
		lnetNIObs("lnet_ni_health_aborted_total", Counter, nid, base+3),
		lnetNIObs("lnet_ni_health_no_route_total", Counter, nid, base+4),
		lnetNIObs("lnet_ni_health_timeouts_total", Counter, nid, base+5),
		lnetNIObs("lnet_ni_health_errors_total", Counter, nid, base+6),
	}
}

// TestParseLNetCtlNetNI_Exact pins the complete output of ParseLNetCtlNetNI,
// in order: NIs without a nid are skipped, the up gauge needs a status and is
// 1 only for "up" or "UP", and the health series need health stats.
func TestParseLNetCtlNetNI_Exact(t *testing.T) {
	const health = `
          health stats:
              health value: %d
              interrupts: %d
              dropped: %d
              aborted: %d
              no route: %d
              timeouts: %d
              error: %d`
	healthOf := func(base int) string {
		return fmt.Sprintf(health, base, base+1, base+2, base+3, base+4, base+5, base+6)
	}
	yamlData := `
net:
    - net type: lo
      local NI(s):
        - nid: 0@lo
          status: up
    - net type: tcp
      local NI(s):
        - nid: ""
          status: up` + healthOf(10) + `
        - nid: 10.0.0.1@tcp
          status: UP
        - nid: 10.0.0.2@tcp
          status: Up
        - nid: 10.0.0.3@tcp
          status: down` + healthOf(1000) + `
        - nid: 10.0.0.4@tcp` + healthOf(2000) + `
        - nid: 10.0.0.5@tcp
    - net type: o2ib
    - net type: o2ib1
      local NI(s):
        - nid: 10.0.1.1@o2ib1
          status: up` + healthOf(3000) + `
`
	var wantYAML []Observation
	wantYAML = append(wantYAML,
		lnetNIObs("lnet_ni_up", Gauge, "0@lo", 1),
		lnetNIObs("lnet_ni_up", Gauge, "10.0.0.1@tcp", 1),
		lnetNIObs("lnet_ni_up", Gauge, "10.0.0.2@tcp", 0),
		lnetNIObs("lnet_ni_up", Gauge, "10.0.0.3@tcp", 0),
	)
	wantYAML = append(wantYAML, lnetNIHealth("10.0.0.3@tcp", 1000)...)
	wantYAML = append(wantYAML, lnetNIHealth("10.0.0.4@tcp", 2000)...)
	wantYAML = append(wantYAML, lnetNIObs("lnet_ni_up", Gauge, "10.0.1.1@o2ib1", 1))
	wantYAML = append(wantYAML, lnetNIHealth("10.0.1.1@o2ib1", 3000)...)

	jsonData := `{"net":[{"local NI(s)":[{"nid":"1@tcp","status":"up","health stats":{"health value":5,"interrupts":6,"dropped":7,"aborted":8,"no route":9,"timeouts":10,"error":11}}]}]}`
	wantJSON := append([]Observation{lnetNIObs("lnet_ni_up", Gauge, "1@tcp", 1)}, lnetNIHealth("1@tcp", 5)...)

	tests := []struct {
		name string
		data string
		want []Observation
	}{
		{name: "empty", data: "", want: nil},
		{name: "no NIs", data: "net:\n    - net type: lo\n", want: nil},
		{name: "only NIs without nid", data: "net:\n    - local NI(s):\n        - status: up\n", want: nil},
		{name: "yaml", data: yamlData, want: wantYAML},
		{name: "json", data: jsonData, want: wantJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, err := ParseLNetCtlNetNI([]byte(tt.data), "src")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertObservations(t, obs, tt.want)
		})
	}
}

func TestParseLNetCtlNetNI_Malformed(t *testing.T) {
	obs, err := ParseLNetCtlNetNI([]byte("net: [\n"), "src")
	const prefix = "lnetctl net show NI: failed to parse as YAML ("
	if err == nil || !strings.HasPrefix(err.Error(), prefix) || !strings.Contains(err.Error(), ") or JSON (") {
		t.Fatalf("err = %v, want prefix %q", err, prefix)
	}
	if obs != nil {
		t.Fatalf("got %d observations alongside the error, want nil", len(obs))
	}
}
