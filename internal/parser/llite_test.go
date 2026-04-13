package parser

import (
	"os"
	"testing"
)

func TestParseLLiteStats(t *testing.T) {
	data, err := os.ReadFile("../../testdata/llite/stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLLiteStats(data, "test", "llite", "scratch-ffff0001")
	if err != nil {
		t.Fatal(err)
	}

	// read_bytes -> 4 obs, write_bytes -> 4 obs, 7 operations -> 7 obs = 15
	if len(obs) != 15 {
		t.Fatalf("got %d observations, want 15", len(obs))
	}

	// Check read_bytes produced correct metrics
	found := map[string]float64{}
	for _, o := range obs {
		if o.Labels["operation"] == "" {
			found[o.MetricID] = o.Value
		}
	}

	if found["read_samples_total"] != 500 {
		t.Errorf("read_samples_total = %f, want 500", found["read_samples_total"])
	}
	if found["read_bytes_total"] != 524288000 {
		t.Errorf("read_bytes_total = %f, want 524288000", found["read_bytes_total"])
	}
	if found["write_samples_total"] != 200 {
		t.Errorf("write_samples_total = %f, want 200", found["write_samples_total"])
	}

	// Check operation stats
	opCounts := map[string]float64{}
	for _, o := range obs {
		if o.MetricID == "stats_total" {
			opCounts[o.Labels["operation"]] = o.Value
		}
	}
	if opCounts["open"] != 1024 {
		t.Errorf("open = %f, want 1024", opCounts["open"])
	}
	if opCounts["getattr"] != 5000 {
		t.Errorf("getattr = %f, want 5000", opCounts["getattr"])
	}
}

func TestParseLLiteStats_Labels(t *testing.T) {
	data, err := os.ReadFile("../../testdata/llite/stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLLiteStats(data, "test", "llite", "scratch-ffff0001")
	if err != nil {
		t.Fatal(err)
	}

	for _, o := range obs {
		if o.Labels["component"] != "llite" {
			t.Errorf("component = %q, want llite", o.Labels["component"])
		}
		if o.Labels["target"] != "scratch-ffff0001" {
			t.Errorf("target = %q, want scratch-ffff0001", o.Labels["target"])
		}
	}
}

func TestParseLLiteSingleFile(t *testing.T) {
	tests := []struct {
		file     string
		metricID string
		value    float64
	}{
		{"blocksize", "blocksize_bytes", 4194304},
		{"filesfree", "inodes_free", 1000000},
		{"filestotal", "inodes_maximum", 2000000},
		{"kbytesavail", "available_kibibytes", 500000000},
		{"kbytesfree", "free_kibibytes", 600000000},
		{"kbytestotal", "capacity_kibibytes", 1000000000},
		{"checksum_pages", "checksum_pages_enabled", 1},
		{"max_read_ahead_mb", "maximum_read_ahead_megabytes", 64},
		{"xattr_cache", "xattr_cache_enabled", 1},
	}

	for _, tt := range tests {
		data, err := os.ReadFile("../../testdata/llite/" + tt.file)
		if err != nil {
			t.Fatalf("%s: %v", tt.file, err)
		}

		obs, err := ParseLLiteSingleFile(data, "test", tt.file, "llite", "scratch-ffff0001")
		if err != nil {
			t.Fatalf("%s: %v", tt.file, err)
		}
		if len(obs) != 1 {
			t.Fatalf("%s: got %d obs, want 1", tt.file, len(obs))
		}
		if obs[0].MetricID != tt.metricID {
			t.Errorf("%s: metricID = %q, want %q", tt.file, obs[0].MetricID, tt.metricID)
		}
		if obs[0].Value != tt.value {
			t.Errorf("%s: value = %f, want %f", tt.file, obs[0].Value, tt.value)
		}
	}
}
