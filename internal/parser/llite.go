package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseLLiteStats parses /proc/fs/lustre/llite/<mount>/stats.
// read_bytes and write_bytes lines produce 4 observations each.
// Other stat lines produce a stats_total observation with an operation label.
func ParseLLiteStats(data []byte, source string, component string, target string) ([]Observation, error) {
	var observations []Observation
	labels := map[string]string{
		"component": component,
		"target":    target,
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		if name == "snapshot_time" {
			continue
		}

		switch name {
		case "read_bytes", "write_bytes":
			obs, err := parseBytesStat(fields, source, component, target)
			if err != nil {
				return nil, fmt.Errorf("llite: %s: %w", name, err)
			}
			observations = append(observations, obs...)
		default:
			obs, err := parseOperationStat(fields, source, component, target)
			if err != nil {
				return nil, fmt.Errorf("llite: %s: %w", name, err)
			}
			observations = append(observations, obs...)
		}
	}

	_ = labels // labels are embedded in each observation
	return observations, nil
}

// parseBytesStat handles read_bytes/write_bytes: name count samples [unit] min max sum
func parseBytesStat(fields []string, source, component, target string) ([]Observation, error) {
	if len(fields) < 7 {
		return nil, fmt.Errorf("expected at least 7 fields, got %d", len(fields))
	}

	name := fields[0]
	prefix := "read"
	if strings.HasPrefix(name, "write") {
		prefix = "write"
	}

	count, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing count: %w", err)
	}
	min, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing min: %w", err)
	}
	max, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing max: %w", err)
	}
	sum, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing sum: %w", err)
	}

	labels := map[string]string{
		"component": component,
		"target":    target,
	}

	return []Observation{
		{Collector: "client", Source: source, MetricID: prefix + "_samples_total", MetricType: Counter, Labels: labels, Value: count},
		{Collector: "client", Source: source, MetricID: prefix + "_minimum_size_bytes", MetricType: Gauge, Labels: labels, Value: min},
		{Collector: "client", Source: source, MetricID: prefix + "_maximum_size_bytes", MetricType: Gauge, Labels: labels, Value: max},
		{Collector: "client", Source: source, MetricID: prefix + "_bytes_total", MetricType: Counter, Labels: labels, Value: sum},
	}, nil
}

// parseOperationStat handles generic stat lines: name count samples [unit]
func parseOperationStat(fields []string, source, component, target string) ([]Observation, error) {
	if len(fields) < 2 {
		return nil, fmt.Errorf("expected at least 2 fields, got %d", len(fields))
	}

	operation := fields[0]
	count, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing count: %w", err)
	}

	labels := map[string]string{
		"component": component,
		"target":    target,
		"operation": operation,
	}

	return []Observation{
		{Collector: "client", Source: source, MetricID: "stats_total", MetricType: Counter, Labels: labels, Value: count},
	}, nil
}

// lliteSingleFileMap maps llite single-value files to MetricIDs.
var lliteSingleFileMap = map[string]struct {
	metricID   string
	metricType MetricType
}{
	"blocksize":                 {"blocksize_bytes", Gauge},
	"filesfree":                 {"inodes_free", Gauge},
	"filestotal":                {"inodes_maximum", Gauge},
	"kbytesavail":               {"available_kibibytes", Gauge},
	"kbytesfree":                {"free_kibibytes", Gauge},
	"kbytestotal":               {"capacity_kibibytes", Gauge},
	"checksum_pages":            {"checksum_pages_enabled", Gauge},
	"default_easize":            {"default_ea_size_bytes", Gauge},
	"lazystatfs":                {"lazystatfs_enabled", Gauge},
	"max_easize":                {"maximum_ea_size_bytes", Gauge},
	"max_read_ahead_mb":         {"maximum_read_ahead_megabytes", Gauge},
	"max_read_ahead_per_file_mb": {"maximum_read_ahead_per_file_megabytes", Gauge},
	"max_read_ahead_whole_mb":   {"maximum_read_ahead_whole_megabytes", Gauge},
	"statahead_agl":             {"statahead_agl_enabled", Gauge},
	"statahead_max":             {"statahead_maximum", Gauge},
	"xattr_cache":               {"xattr_cache_enabled", Gauge},
}

// ParseLLiteSingleFile parses a single-value llite file (capacity or tunable).
func ParseLLiteSingleFile(data []byte, source string, fileName string, component string, target string) ([]Observation, error) {
	entry, ok := lliteSingleFileMap[fileName]
	if !ok {
		return nil, nil
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return nil, fmt.Errorf("llite %q: %w", fileName, err)
	}

	return []Observation{
		{
			Collector:  "client",
			Source:     source,
			MetricID:   entry.metricID,
			MetricType: entry.metricType,
			Labels: map[string]string{
				"component": component,
				"target":    target,
			},
			Value: val,
		},
	}, nil
}
