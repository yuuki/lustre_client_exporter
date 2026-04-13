package parser

import (
	"os"
	"testing"
)

func TestParseEncryptPagePools(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sptlrpc/encrypt_page_pools.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseEncryptPagePools(data, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(obs) != 15 {
		t.Fatalf("got %d observations, want 15", len(obs))
	}

	expected := map[string]float64{
		"physical_pages":               262144,
		"pages_per_pool":               256,
		"maximum_pages":                65536,
		"maximum_pools":                256,
		"pages_in_pools":               1024,
		"free_pages":                   512,
		"maximum_pages_reached_total":  2048,
		"grows_total":                  10,
		"grows_failure_total":          1,
		"shrinks_total":                5,
		"cache_access_total":           50000,
		"cache_miss_total":             100,
		"free_page_low":                0,
		"maximum_waitqueue_depth":      3,
		"out_of_memory_request_total":  0,
	}

	found := map[string]float64{}
	for _, o := range obs {
		found[o.MetricID] = o.Value
		if o.Collector != "sptlrpc" {
			t.Errorf("collector = %q, want %q", o.Collector, "sptlrpc")
		}
	}

	for id, want := range expected {
		got, ok := found[id]
		if !ok {
			t.Errorf("missing metric %q", id)
			continue
		}
		if got != want {
			t.Errorf("%s = %f, want %f", id, got, want)
		}
	}
}

func TestParseEncryptPagePools_Empty(t *testing.T) {
	obs, err := ParseEncryptPagePools([]byte(""), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("got %d observations for empty input, want 0", len(obs))
	}
}
