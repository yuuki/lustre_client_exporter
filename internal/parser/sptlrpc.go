package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// sptlrpcKeyMap maps the key strings found in encrypt_page_pools to internal MetricIDs.
var sptlrpcKeyMap = map[string]struct {
	metricID   string
	metricType MetricType
}{
	"physical pages":       {"physical_pages", Gauge},
	"pages per pool":       {"pages_per_pool", Gauge},
	"max pages":            {"maximum_pages", Gauge},
	"max pools":            {"maximum_pools", Gauge},
	"pages in pools":       {"pages_in_pools", Gauge},
	"free pages":           {"free_pages", Gauge},
	"max pages reached":    {"maximum_pages_reached_total", Counter},
	"grows":                {"grows_total", Counter},
	"grows failure":        {"grows_failure_total", Counter},
	"shrinks":              {"shrinks_total", Counter},
	"cache access":         {"cache_access_total", Counter},
	"cache miss":           {"cache_miss_total", Counter},
	"free page low":        {"free_page_low", Gauge},
	"max waitqueue depth":  {"maximum_waitqueue_depth", Gauge},
	"out of mem":           {"out_of_memory_request_total", Counter},
}

// ParseEncryptPagePools parses the content of sptlrpc/encrypt_page_pools.
func ParseEncryptPagePools(data []byte, source string) ([]Observation, error) {
	var observations []Observation

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		valStr := strings.TrimSpace(line[idx+1:])

		entry, ok := sptlrpcKeyMap[key]
		if !ok {
			continue
		}

		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return nil, fmt.Errorf("sptlrpc: parsing %q value %q: %w", key, valStr, err)
		}

		observations = append(observations, Observation{
			Collector:  "sptlrpc",
			Source:     source,
			MetricID:   entry.metricID,
			MetricType: entry.metricType,
			Labels:     nil,
			Value:      val,
		})
	}

	return observations, nil
}
