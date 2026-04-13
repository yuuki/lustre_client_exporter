package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// lnetStatsFields maps positional fields in /proc/sys/lnet/stats to MetricIDs.
// Format: msgs_alloc msgs_max errors send_count recv_count route_count drop_count send_bytes recv_bytes route_bytes drop_bytes
var lnetStatsFields = []struct {
	metricID   string
	metricType MetricType
}{
	{"allocated", Gauge},
	{"maximum", Gauge},
	{"errors_total", Counter},
	{"send_count_total", Counter},
	{"receive_count_total", Counter},
	{"route_count_total", Counter},
	{"drop_count_total", Counter},
	{"send_bytes_total", Counter},
	{"receive_bytes_total", Counter},
	{"route_bytes_total", Counter},
	{"drop_bytes_total", Counter},
}

// ParseLNetStats parses the single-line /proc/sys/lnet/stats file.
func ParseLNetStats(data []byte, source string) ([]Observation, error) {
	line := strings.TrimSpace(string(data))
	if line == "" {
		return nil, nil
	}

	fields := strings.Fields(line)
	if len(fields) != len(lnetStatsFields) {
		return nil, fmt.Errorf("lnet stats: expected %d fields, got %d", len(lnetStatsFields), len(fields))
	}

	observations := make([]Observation, 0, len(fields))
	for i, f := range fields {
		val, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, fmt.Errorf("lnet stats field %d: %w", i, err)
		}
		observations = append(observations, Observation{
			Collector:  "lnet",
			Source:     source,
			MetricID:   lnetStatsFields[i].metricID,
			MetricType: lnetStatsFields[i].metricType,
			Labels:     nil,
			Value:      val,
		})
	}
	return observations, nil
}

// lnetParamMap maps procfs param filenames to MetricIDs.
var lnetParamMap = map[string]struct {
	metricID   string
	metricType MetricType
}{
	"console_backoff":             {"console_backoff_enabled", Gauge},
	"console_max_delay_centisecs": {"console_max_delay_centiseconds", Gauge},
	"console_min_delay_centisecs": {"console_min_delay_centiseconds", Gauge},
	"console_ratelimit":           {"console_ratelimit_enabled", Gauge},
	"debug_mb":                    {"debug_megabytes", Gauge},
	"panic_on_lbug":               {"panic_on_lbug_enabled", Gauge},
	"watchdog_ratelimit":          {"watchdog_ratelimit_enabled", Gauge},
	"catastrophe":                 {"catastrophe_enabled", Gauge},
	"lnet_memused":                {"lnet_memory_used_bytes", Gauge},
	"fail_err":                    {"fail_error_total", Counter},
	"fail_max":                    {"fail_maximum", Gauge},
}

// ParseLNetParam parses a single-value LNet parameter file.
func ParseLNetParam(data []byte, source string, paramName string) ([]Observation, error) {
	entry, ok := lnetParamMap[paramName]
	if !ok {
		return nil, nil
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return nil, fmt.Errorf("lnet param %q: %w", paramName, err)
	}

	return []Observation{
		{
			Collector:  "lnet",
			Source:     source,
			MetricID:   entry.metricID,
			MetricType: entry.metricType,
			Labels:     nil,
			Value:      val,
		},
	}, nil
}

// lnetCtlStats represents the JSON output of `lnetctl stats show`.
type lnetCtlStats struct {
	Statistics struct {
		MsgsAlloc   float64 `json:"msgs_alloc"`
		MsgsMax     float64 `json:"msgs_max"`
		Errors      float64 `json:"errors"`
		SendCount   float64 `json:"send_count"`
		RecvCount   float64 `json:"recv_count"`
		RouteCount  float64 `json:"route_count"`
		DropCount   float64 `json:"drop_count"`
		SendLength  float64 `json:"send_length"`
		RecvLength  float64 `json:"recv_length"`
		RouteLength float64 `json:"route_length"`
		DropLength  float64 `json:"drop_length"`
	} `json:"statistics"`
}

// ParseLNetCtlStats parses the JSON output of `lnetctl stats show`.
func ParseLNetCtlStats(data []byte, source string) ([]Observation, error) {
	var stats lnetCtlStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("lnetctl stats: %w", err)
	}

	s := stats.Statistics
	return []Observation{
		{Collector: "lnet", Source: source, MetricID: "allocated", MetricType: Gauge, Value: s.MsgsAlloc},
		{Collector: "lnet", Source: source, MetricID: "maximum", MetricType: Gauge, Value: s.MsgsMax},
		{Collector: "lnet", Source: source, MetricID: "errors_total", MetricType: Counter, Value: s.Errors},
		{Collector: "lnet", Source: source, MetricID: "send_count_total", MetricType: Counter, Value: s.SendCount},
		{Collector: "lnet", Source: source, MetricID: "receive_count_total", MetricType: Counter, Value: s.RecvCount},
		{Collector: "lnet", Source: source, MetricID: "route_count_total", MetricType: Counter, Value: s.RouteCount},
		{Collector: "lnet", Source: source, MetricID: "drop_count_total", MetricType: Counter, Value: s.DropCount},
		{Collector: "lnet", Source: source, MetricID: "send_bytes_total", MetricType: Counter, Value: s.SendLength},
		{Collector: "lnet", Source: source, MetricID: "receive_bytes_total", MetricType: Counter, Value: s.RecvLength},
		{Collector: "lnet", Source: source, MetricID: "route_bytes_total", MetricType: Counter, Value: s.RouteLength},
		{Collector: "lnet", Source: source, MetricID: "drop_bytes_total", MetricType: Counter, Value: s.DropLength},
	}, nil
}
