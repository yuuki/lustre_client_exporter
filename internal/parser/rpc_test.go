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

	// 3 sections * 4 rows * 2 (read+write) = 24 histogram + 5 scalars = 29
	if len(obs) != 29 {
		t.Fatalf("got %d observations, want 29", len(obs))
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
	// histogram 26 + current 4 + pending 2 = 32
	if len(obs) != 32 {
		t.Fatalf("got %d observations, want 32", len(obs))
	}
}

func TestParseRPCStats_CurrentScalarsOSC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/rpc_stats.txt")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseRPCStats(data, "test", "client", "scratch-OST0000-osc-ffff0001", "osc")
	if err != nil {
		t.Fatal(err)
	}
	current := map[string]float64{}
	pending := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_current":
			current[o.Labels["operation"]] = o.Value
			if o.Labels["type"] != "osc" || o.MetricType != Gauge {
				t.Fatalf("bad rpcs_current: %+v", o)
			}
		case "pending_pages":
			pending[o.Labels["operation"]] = o.Value
		}
	}
	if current["read"] != 3 || current["write"] != 5 || current["dio_read"] != 1 || current["dio_write"] != 0 {
		t.Fatalf("current = %v", current)
	}
	if pending["read"] != 8 || pending["write"] != 120 {
		t.Fatalf("pending = %v", pending)
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

func TestParseRPCStats_ScalarRpcsInFlightIsNotABucket(t *testing.T) {
	data := []byte(`
rpcs in flight        rpcs   % cum %
read RPCs in flight:  0
0:                    7 100 100
`)
	obs, err := ParseRPCStats(data, "test", "mdc", "target", "mdc")
	if err != nil {
		t.Fatal(err)
	}
	var buckets, currents int
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_in_flight":
			buckets++
			if o.Labels["size"] != "0" || o.Value != 7 {
				t.Fatalf("bucket = %+v", o)
			}
		case "rpcs_current":
			currents++
			if o.Labels["operation"] != "read" || o.Value != 0 {
				t.Fatalf("current = %+v", o)
			}
		default:
			t.Fatalf("unexpected %s", o.MetricID)
		}
	}
	if buckets != 1 || currents != 1 {
		t.Fatalf("buckets=%d currents=%d", buckets, currents)
	}
}

func TestParseRPCStats_RpcsInFlightReadWriteScalarSubsections(t *testing.T) {
	data := []byte(`
rpcs in flight        rpcs   % cum %
read RPCs in flight:  0
1:                    0 100 100
write RPCs in flight: 0
1:                    2 100 100
`)

	obs, err := ParseRPCStats(data, "test", "mdc", "target", "mdc")
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]float64{}
	current := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_in_flight":
			found[o.Labels["operation"]+"/"+o.Labels["size"]] = o.Value
		case "rpcs_current":
			current[o.Labels["operation"]] = o.Value
		default:
			continue
		}
	}
	if found["read/1"] != 0 {
		t.Errorf("read/1 = %v, want 0", found["read/1"])
	}
	if found["write/1"] != 2 {
		t.Errorf("write/1 = %v, want 2", found["write/1"])
	}
	if current["read"] != 0 || current["write"] != 0 {
		t.Fatalf("current = %v", current)
	}
}

func TestParseRPCStats_MDCRpcsInFlightModifyAndReadWriteSections(t *testing.T) {
	data := []byte(`
snapshot_time:            1776181530.650669515 secs.nsecs
modify_RPCs_in_flight:  0

                        modify
rpcs in flight        rpcs   % cum %
0:                       0   0   0
1:                 5268464  56  56

read RPCs in flight:  0
write RPCs in flight: 0
pending write pages:  0
pending read pages:   0

                        read                    write
rpcs in flight        rpcs   % cum % |       rpcs   % cum %
1:                       0   0   0   |          0   0   0
`)

	obs, err := ParseRPCStats(data, "test", "mdc", "target", "mdc")
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]float64{}
	current := map[string]float64{}
	pending := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_in_flight":
			key := o.Labels["operation"] + "/" + o.Labels["size"]
			if _, ok := found[key]; ok {
				t.Fatalf("duplicate rpcs_in_flight series for %s", key)
			}
			found[key] = o.Value
		case "rpcs_current":
			current[o.Labels["operation"]] = o.Value
		case "pending_pages":
			pending[o.Labels["operation"]] = o.Value
		default:
			continue
		}
	}
	if found["modify/1"] != 5268464 {
		t.Errorf("modify/1 = %v, want 5268464", found["modify/1"])
	}
	if found["read/1"] != 0 {
		t.Errorf("read/1 = %v, want 0", found["read/1"])
	}
	if found["write/1"] != 0 {
		t.Errorf("write/1 = %v, want 0", found["write/1"])
	}
	if current["modify"] != 0 || current["read"] != 0 || current["write"] != 0 {
		t.Fatalf("current = %v", current)
	}
	if pending["read"] != 0 || pending["write"] != 0 {
		t.Fatalf("pending = %v", pending)
	}
}

func TestParseRPCStats_MDCRpcsInFlightModifyBuckets(t *testing.T) {
	data := []byte(`
snapshot_time:            1776181750.010510947 secs.nsecs
modify_RPCs_in_flight:  0

                        modify
rpcs in flight        rpcs   % cum %
0:                       0   0   0
1:                  194398  99  99
2:                     656   0  99
3:                      41   0  99
4:                      23   0  99
5:                      32   0  99
6:                      29   0  99
7:                      27   0 100

read RPCs in flight:  0
write RPCs in flight: 0
pending write pages:  0
pending read pages:   0

                        read                    write
pages per rpc         rpcs   % cum % |       rpcs   % cum %
1:                       0   0   0   |          0   0   0

                        read                    write
rpcs in flight        rpcs   % cum % |       rpcs   % cum %
1:                       0   0   0   |          0   0   0

                        read                    write
offset                rpcs   % cum % |       rpcs   % cum %
0:                       0   0   0   |          0   0   0
`)

	obs, err := ParseRPCStats(data, "test", "mdc", "target", "mdc")
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]float64{}
	current := map[string]float64{}
	pending := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_in_flight":
			key := o.Labels["operation"] + "/" + o.Labels["size"]
			if _, ok := found[key]; ok {
				t.Fatalf("duplicate rpcs_in_flight series for %s", key)
			}
			found[key] = o.Value
		case "rpcs_current":
			current[o.Labels["operation"]] = o.Value
		case "pending_pages":
			pending[o.Labels["operation"]] = o.Value
		default:
			continue
		}
	}

	expected := map[string]float64{
		"modify/0": 0,
		"modify/1": 194398,
		"modify/2": 656,
		"modify/3": 41,
		"modify/4": 23,
		"modify/5": 32,
		"modify/6": 29,
		"modify/7": 27,
		"read/1":   0,
		"write/1":  0,
	}
	for key, want := range expected {
		if found[key] != want {
			t.Errorf("%s = %v, want %v", key, found[key], want)
		}
	}
	if current["modify"] != 0 || current["read"] != 0 || current["write"] != 0 {
		t.Fatalf("current = %v", current)
	}
	if pending["read"] != 0 || pending["write"] != 0 {
		t.Fatalf("pending = %v", pending)
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

// The expected observations of TestParseRPCStats_Exact, one constructor per
// public series so that each hard-codes its metric type and label set.
func rpcPagesPerRPC(op, size string, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: "pages_per_rpc_total", MetricType: Counter,
		Labels: map[string]string{"component": "client", "target": "tgt", "operation": op, "size": size}, Value: v}
}

func rpcInFlight(op, size string, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: "rpcs_in_flight", MetricType: Gauge,
		Labels: map[string]string{"component": "client", "target": "tgt", "operation": op, "size": size, "type": "osc"}, Value: v}
}

func rpcOffset(op, size string, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: "rpcs_offset", MetricType: Gauge,
		Labels: map[string]string{"component": "client", "target": "tgt", "operation": op, "size": size}, Value: v}
}

func rpcScalar(metricID, op string, v float64) Observation {
	return Observation{Collector: "client", Source: "src", MetricID: metricID, MetricType: Gauge,
		Labels: map[string]string{"component": "client", "target": "tgt", "type": "osc", "operation": op}, Value: v}
}

// TestParseRPCStats_Exact pins the complete output of ParseRPCStats, in
// order: which lines are scalars, section headers or buckets, which operation
// a bucket is attributed to, and the error text of a malformed bucket.
func TestParseRPCStats_Exact(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []Observation
		wantErr string
	}{
		{name: "empty", data: "", want: nil},
		{name: "blank lines only", data: "\n  \n\t\n", want: nil},
		{
			name: "read and write columns",
			data: "pages per rpc  rpcs % cum % rpcs % cum %\n1:  10 50 50  20 40 40\n2: 5 25 75 15 30 70 99\n",
			want: []Observation{
				rpcPagesPerRPC("read", "1", 10), rpcPagesPerRPC("write", "1", 20),
				rpcPagesPerRPC("read", "2", 5), rpcPagesPerRPC("write", "2", 15),
			},
		},
		{
			name: "pipe between the read and write columns",
			data: "rpcs in flight  rpcs % cum % | rpcs % cum %\n1: 3 1 1 | 4 1 1\n",
			want: []Observation{rpcInFlight("read", "1", 3), rpcInFlight("write", "1", 4)},
		},
		{
			name: "offset section",
			data: "offset  pages % cum %  pages % cum %\n0: 7 1 1 8 1 1\n",
			want: []Observation{rpcOffset("read", "0", 7), rpcOffset("write", "0", 8)},
		},
		{
			name: "bucket without write column is read only",
			data: "rpcs in flight  rpcs % cum %\n0: 7 100 100\n1: 9\n2: 6 1 1 |\n",
			want: []Observation{rpcInFlight("read", "0", 7), rpcInFlight("read", "1", 9), rpcInFlight("read", "2", 6)},
		},
		{
			name: "pending modify names the next rpcs in flight section",
			data: "  modify  \nrpcs in flight  rpcs % cum %\n0: 3 1 1\n1: 4 1 1 | 5 1 1\n",
			want: []Observation{rpcInFlight("modify", "0", 3), rpcInFlight("read", "1", 4), rpcInFlight("write", "1", 5)},
		},
		{
			name: "rpcs in flight consumes the pending modify",
			data: "modify\nrpcs in flight\n0: 3 1 1\nrpcs in flight\n1: 4 1 1\n",
			want: []Observation{rpcInFlight("modify", "0", 3), rpcInFlight("read", "1", 4)},
		},
		{
			name: "modify_RPCs_in_flight is a scalar, not the modify header",
			data: "modify_RPCs_in_flight: 1\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "modify", 1), rpcInFlight("read", "0", 3)},
		},
		{
			name: "scalar lines and blank lines keep the pending modify",
			data: "modify\n\nmodify_RPCs_in_flight: 2\npending read pages: 3\npending write pages: 4\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{
				rpcScalar("rpcs_current", "modify", 2), rpcScalar("pending_pages", "read", 3),
				rpcScalar("pending_pages", "write", 4), rpcInFlight("modify", "0", 3),
			},
		},
		{
			name: "column header clears the pending modify",
			data: "modify\n      read    write\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcInFlight("read", "0", 3)},
		},
		{
			name: "pages per rpc clears the pending modify",
			data: "modify\npages per rpc\n1: 1 1 1\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcPagesPerRPC("read", "1", 1), rpcInFlight("read", "0", 3)},
		},
		{
			name: "offset clears the pending modify",
			data: "modify\noffset\n1: 1 1 1\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcOffset("read", "1", 1), rpcInFlight("read", "0", 3)},
		},
		{
			name: "dio read subsection clears the pending modify",
			data: "modify\ndio read RPCs in flight: 2\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "dio_read", 2), rpcInFlight("read", "0", 3)},
		},
		{
			name: "dio write subsection clears the pending modify",
			data: "modify\ndio write RPCs in flight: 2\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "dio_write", 2), rpcInFlight("read", "0", 3)},
		},
		{
			name: "read subsection clears the pending modify",
			data: "modify\nread RPCs in flight: 2\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "read", 2), rpcInFlight("read", "0", 3)},
		},
		{
			name: "write subsection clears the pending modify",
			data: "modify\nwrite RPCs in flight: 2\nrpcs in flight\n0: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "write", 2), rpcInFlight("read", "0", 3)},
		},
		{
			name: "per-operation subsections",
			data: "rpcs in flight  rpcs % cum %\n" +
				"read RPCs in flight: 1\n0: 2 1 1\n" +
				"write RPCs in flight: 3\n0: 4 1 1\n" +
				"dio read RPCs in flight: 5\n0: 6 1 1\n" +
				"dio write RPCs in flight: 7\n0: 8 1 1\n",
			want: []Observation{
				rpcScalar("rpcs_current", "read", 1), rpcInFlight("read", "0", 2),
				rpcScalar("rpcs_current", "write", 3), rpcInFlight("write", "0", 4),
				rpcScalar("rpcs_current", "dio_read", 5), rpcInFlight("dio_read", "0", 6),
				rpcScalar("rpcs_current", "dio_write", 7), rpcInFlight("dio_write", "0", 8),
			},
		},
		{
			name: "subsection operation is ignored when the bucket has a write column",
			data: "write RPCs in flight: 0\n1: 2 1 1 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "write", 0), rpcInFlight("read", "1", 2), rpcInFlight("write", "1", 3)},
		},
		{
			name: "column header keeps the subsection operation",
			data: "write RPCs in flight: 0\n   read   write\n1: 2 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "write", 0), rpcInFlight("write", "1", 2)},
		},
		{
			name: "subsection header without a valid value still opens the section",
			data: "dio write RPCs in flight:\n1: 2 1 1\nwrite RPCs in flight: x\n2: 3 1 1\n",
			want: []Observation{rpcInFlight("dio_write", "1", 2), rpcInFlight("write", "2", 3)},
		},
		{
			name: "pages per rpc resets the subsection operation",
			data: "write RPCs in flight: 0\npages per rpc\n1: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "write", 0), rpcPagesPerRPC("read", "1", 3)},
		},
		{
			name: "offset resets the subsection operation",
			data: "write RPCs in flight: 0\noffset\n1: 3 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "write", 0), rpcOffset("read", "1", 3)},
		},
		{
			name: "section headers win over the column header check",
			data: "modify\nrpcs in flight   read   write\n0: 1 1 1\n" +
				"offset   read   write\n1: 2 1 1\n" +
				"pages per rpc   read   write\n2: 3 1 1\n",
			want: []Observation{rpcInFlight("modify", "0", 1), rpcOffset("read", "1", 2), rpcPagesPerRPC("read", "2", 3)},
		},
		{
			name: "buckets before any section are ignored",
			data: "1: 2 3 4 5\npages per rpc\n1: 6 1 1\n",
			want: []Observation{rpcPagesPerRPC("read", "1", 6)},
		},
		{
			name: "non-bucket lines inside a section are ignored",
			data: "pages per rpc\nrpcs % cum %\n1 2 3 4 5\n1: 6 1 1\n",
			want: []Observation{rpcPagesPerRPC("read", "1", 6)},
		},
		{
			name: "scalars inside a section are not buckets",
			data: "pages per rpc\nmodify_RPCs_in_flight: 9\nPENDING read pages: 8 pages\npending write pages: 120\n1: 6 1 1\n",
			want: []Observation{
				rpcScalar("rpcs_current", "modify", 9), rpcScalar("pending_pages", "read", 8),
				rpcScalar("pending_pages", "write", 120), rpcPagesPerRPC("read", "1", 6),
			},
		},
		{
			name: "scalar labels are case-insensitive but subsection headers are not",
			data: "pages per rpc\nREAD RPCs IN FLIGHT: 4\n1: 6 1 1\n",
			want: []Observation{rpcScalar("rpcs_current", "read", 4), rpcPagesPerRPC("read", "1", 6)},
		},
		{
			name: "invalid scalars are skipped",
			data: "pending write pages: x\nmodify_rpcs_in_flight:\nunknown rpcs: 3\npages per rpc\n1: 6 1 1\n",
			want: []Observation{rpcPagesPerRPC("read", "1", 6)},
		},
		{
			name: "snapshot_time is skipped inside a section",
			data: "pages per rpc\nsnapshot_time: 1681000000 1 1 2\n1: 6 1 1\n",
			want: []Observation{rpcPagesPerRPC("read", "1", 6)},
		},
		{
			name:    "bucket with a single field",
			data:    "pages per rpc\n1: 6 1 1\n2:\n",
			wantErr: `rpc stats pages_per_rpc: "2:": expected at least 2 fields, got 1`,
		},
		{
			name:    "malformed read value",
			data:    "read RPCs in flight: 1\nrpcs in flight\n1:   abc 1 1\n",
			wantErr: `rpc stats rpcs_in_flight: "1:   abc 1 1": strconv.ParseFloat: parsing "abc": invalid syntax`,
		},
		{
			name:    "malformed write value",
			data:    "offset\n1: 1 1 1 x\n",
			wantErr: `rpc stats rpcs_offset: "1: 1 1 1 x": strconv.ParseFloat: parsing "x": invalid syntax`,
		},
		{
			name:    "malformed write value after a pipe",
			data:    "offset\n1: 1 1 1 | y 1 1\n",
			wantErr: `rpc stats rpcs_offset: "1: 1 1 1 | y 1 1": strconv.ParseFloat: parsing "y": invalid syntax`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, err := ParseRPCStats([]byte(tt.data), "src", "client", "tgt", "osc")
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
