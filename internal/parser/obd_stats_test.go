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

func obdStatsTotal(op string, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: "stats_total", MetricType: Counter,
		Labels: map[string]string{"component": "client", "target": "tgt", "operation": op}, Value: v}
}

func obdStatsSeconds(op string, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: "stats_seconds_sum", MetricType: Counter,
		Labels: map[string]string{"component": "client", "target": "tgt", "type": "osc", "operation": op}, Value: v}
}

func obdBytes(metricID string, mt MetricType, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: metricID, MetricType: mt,
		Labels: map[string]string{"component": "client", "target": "tgt"}, Value: v}
}

// TestParseOBDStats_Exact pins the complete output of ParseOBDStats, in
// order: every time unit, the lines that emit no seconds_sum, the byte
// counters, skipped lines and the error texts.
func TestParseOBDStats_Exact(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []Observation
		wantErr string
	}{
		{name: "empty", data: "", want: nil},
		{name: "skipped lines only", data: "\n   \nlonely\nsnapshot_time 1 samples [secs] 1 1 1\n", want: nil},
		{
			name: "time units",
			data: "a 1 samples [nsec] 0 0 3000000000\n" +
				"b 2 samples [nsecs] 0 0 4000000000\n" +
				"c 3 samples [usec] 0 0 2500000\n" +
				"d 4 samples [usecs] 0 0 3500000\n" +
				"e 5 samples [msec] 0 0 1500\n" +
				"f 6 samples [msecs] 0 0 2500\n" +
				"g 7 samples [sec] 0 0 7\n" +
				"h 8 samples [secs] 0 0 8 64\n" +
				"i 9 samples [USEC] 0 0 9000000\n" +
				"j 10 samples usec 0 0 10000000\n" +
				"k 11 samples [[msec]] 0 0 11000\n",
			want: []Observation{
				obdStatsTotal("a", 1), obdStatsSeconds("a", 3),
				obdStatsTotal("b", 2), obdStatsSeconds("b", 4),
				obdStatsTotal("c", 3), obdStatsSeconds("c", 2.5),
				obdStatsTotal("d", 4), obdStatsSeconds("d", 3.5),
				obdStatsTotal("e", 5), obdStatsSeconds("e", 1.5),
				obdStatsTotal("f", 6), obdStatsSeconds("f", 2.5),
				obdStatsTotal("g", 7), obdStatsSeconds("g", 7),
				obdStatsTotal("h", 8), obdStatsSeconds("h", 8),
				obdStatsTotal("i", 9), obdStatsSeconds("i", 9),
				obdStatsTotal("j", 10), obdStatsSeconds("j", 10),
				obdStatsTotal("k", 11), obdStatsSeconds("k", 11),
			},
		},
		{
			name: "lines without a seconds sum",
			data: "count_only 5\n" +
				"reqs 6 samples [reqs] 1 2 3\n" +
				"short 7 samples [usec] 1 2\n" +
				"events 8 events [usec] 1 2 3\n" +
				"badsum 9 samples [usec] 1 2 x\n" +
				"exact 10 samples [usec] 1 2 3000000\n",
			want: []Observation{
				obdStatsTotal("count_only", 5),
				obdStatsTotal("reqs", 6),
				obdStatsTotal("short", 7),
				obdStatsTotal("events", 8),
				obdStatsTotal("badsum", 9),
				obdStatsTotal("exact", 10), obdStatsSeconds("exact", 3),
			},
		},
		{
			name: "byte counters emit size metrics only",
			data: "  read_bytes 3 samples [usec] 4096 8192 12288  \n" +
				"write_bytes 4 samples [bytes] 512 1024 2048\n" +
				"read_bytes_x 5 samples [secs] 1 1 6\n",
			want: []Observation{
				obdBytes("read_samples_total", Counter, 3),
				obdBytes("read_minimum_size_bytes", Gauge, 4096),
				obdBytes("read_maximum_size_bytes", Gauge, 8192),
				obdBytes("read_bytes_total", Counter, 12288),
				obdBytes("write_samples_total", Counter, 4),
				obdBytes("write_minimum_size_bytes", Gauge, 512),
				obdBytes("write_maximum_size_bytes", Gauge, 1024),
				obdBytes("write_bytes_total", Counter, 2048),
				obdStatsTotal("read_bytes_x", 5), obdStatsSeconds("read_bytes_x", 6),
			},
		},
		{
			name:    "malformed count",
			data:    "ok 1 samples [secs] 1 1 1\nreq_waittime x samples [usec] 1 2 3\n",
			wantErr: `obd stats req_waittime: parsing count: strconv.ParseFloat: parsing "x": invalid syntax`,
		},
		{
			name:    "short byte counter",
			data:    "ok 1\nwrite_bytes 3 samples\n",
			wantErr: "obd stats write_bytes: expected at least 7 fields, got 3",
		},
		{
			name:    "malformed byte counter",
			data:    "read_bytes 3 samples [bytes] 1 y 3\n",
			wantErr: `obd stats read_bytes: parsing max: strconv.ParseFloat: parsing "y": invalid syntax`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, err := ParseOBDStats([]byte(tt.data), "src", "client", "tgt", "osc")
			if tt.wantErr != "" {
				assertParseError(t, obs, err, tt.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertObservations(t, obs, tt.want)
		})
	}
}
