package parser

import (
	"os"
	"testing"
)

func TestParseClientSingleFile(t *testing.T) {
	tests := []struct {
		file     string
		typ      string
		metricID string
		value    float64
		hasType  bool
	}{
		{"cur_dirty_bytes", "osc", "osc_dirty_bytes", 1048576, false},
		{"max_dirty_mb", "osc", "osc_max_dirty_bytes", 33554432, false},
		{"max_pages_per_rpc", "osc", "max_pages_per_rpc", 256, true},
		{"max_rpcs_in_flight", "osc", "max_rpcs_in_flight", 8, true},
		{"max_mod_rpcs_in_flight", "mdc", "max_mod_rpcs_in_flight", 16, true},
		{"active", "osc", "target_active", 1, true},
	}
	for _, tt := range tests {
		dir := "osc"
		if tt.typ == "mdc" {
			dir = "mdc"
		}
		data, err := os.ReadFile("../../testdata/" + dir + "/" + tt.file)
		if err != nil {
			t.Fatal(err)
		}
		obs, err := ParseClientSingleFile(data, "test", tt.file, "client", "tgt", tt.typ)
		if err != nil {
			t.Fatal(err)
		}
		if len(obs) != 1 || obs[0].MetricID != tt.metricID || obs[0].Value != tt.value {
			t.Fatalf("%s: %+v", tt.file, obs)
		}
		if _, ok := obs[0].Labels["type"]; ok != tt.hasType {
			t.Fatalf("%s type label present=%v", tt.file, ok)
		}
	}
}

func TestParseClientSingleFile_FractionalMB(t *testing.T) {
	obs, err := ParseClientSingleFile([]byte("29.15\n"), "test", "max_dirty_mb", "client", "tgt", "osc")
	if err != nil {
		t.Fatal(err)
	}
	want := 29.15 * 1024 * 1024
	if obs[0].Value != want {
		t.Fatalf("got %v want %v", obs[0].Value, want)
	}
}

func TestParseClientSingleFile_Unknown(t *testing.T) {
	obs, err := ParseClientSingleFile([]byte("1\n"), "test", "unknown", "client", "tgt", "osc")
	if err != nil || len(obs) != 0 {
		t.Fatalf("got %v %v", obs, err)
	}
}

func TestParseClientState_CurrentStateOnly(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/state.txt")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseClientState(data, "test", "client", "tgt", "osc")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].MetricID != "target_state" || obs[0].Value != 1 {
		t.Fatalf("%+v", obs)
	}
	if obs[0].Labels["state"] != "FULL" || obs[0].Labels["type"] != "osc" {
		t.Fatalf("labels = %v", obs[0].Labels)
	}
}

func TestParseClientState_BareToken(t *testing.T) {
	obs, err := ParseClientState([]byte("FULL\n"), "test", "client", "tgt", "osc")
	if err != nil || len(obs) != 1 || obs[0].Labels["state"] != "FULL" {
		t.Fatalf("%v %v", obs, err)
	}
}

func TestParseClientState_Empty(t *testing.T) {
	obs, err := ParseClientState([]byte("\n"), "test", "client", "tgt", "osc")
	if err != nil || len(obs) != 0 {
		t.Fatalf("%v %v", obs, err)
	}
}
