package parser

import (
	"os"
	"testing"
)

func TestParseLpccStatus(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lpcc/status.json")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseLpccStatus(data, "lpcc status")
	if err != nil {
		t.Fatal(err)
	}

	// 2 mounts × (17 per-cache + 6 per-mount) = 46
	if got := len(obs); got != 46 {
		t.Fatalf("expected 46 observations, got %d", got)
	}

	// Verify a sample per-cache metric
	found := false
	for _, o := range obs {
		if o.MetricID == "pcc_status" && o.Labels["mount"] == "/disk/dn-h" {
			found = true
			if o.Value != 1.0 {
				t.Errorf("pcc_status for /disk/dn-h: expected 1.0, got %f", o.Value)
			}
			if o.Labels["cache"] != "/nvme67/dn-h-lpcc" {
				t.Errorf("expected cache /nvme67/dn-h-lpcc, got %s", o.Labels["cache"])
			}
			if o.Collector != "lpcc" {
				t.Errorf("expected collector lpcc, got %s", o.Collector)
			}
			break
		}
	}
	if !found {
		t.Error("pcc_status observation for /disk/dn-h not found")
	}

	// Verify purge_status for stopped purge
	for _, o := range obs {
		if o.MetricID == "pcc_purge_status" && o.Labels["mount"] == "/disk/dn-k" {
			if o.Value != 0.0 {
				t.Errorf("pcc_purge_status for /disk/dn-k (stopped): expected 0.0, got %f", o.Value)
			}
			break
		}
	}

	// Verify ratio conversion (cache_usage_pct 0.8523850759891535 / 100)
	for _, o := range obs {
		if o.MetricID == "pcc_cache_usage_ratio" && o.Labels["mount"] == "/disk/dn-h" {
			expected := 0.8523850759891535 / 100
			if o.Value != expected {
				t.Errorf("pcc_cache_usage_ratio: expected %f, got %f", expected, o.Value)
			}
			break
		}
	}

	// Verify a per-mount fs_stats metric
	for _, o := range obs {
		if o.MetricID == "pcc_fs_open_count_total" && o.Labels["mount"] == "/disk/dn-h" {
			if o.Value != 260373591 {
				t.Errorf("pcc_fs_open_count_total: expected 260373591, got %f", o.Value)
			}
			if _, hasCacheLabel := o.Labels["cache"]; hasCacheLabel {
				t.Error("fs_stats metric should not have cache label")
			}
			break
		}
	}
}

func TestParseLpccStatus_Empty(t *testing.T) {
	obs, err := ParseLpccStatus([]byte("{}"), "lpcc status")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Fatalf("expected 0 observations for empty JSON, got %d", len(obs))
	}
}

func TestParseLpccStatus_InvalidJSON(t *testing.T) {
	_, err := ParseLpccStatus([]byte("not json"), "lpcc status")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
