package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseOBDStats parses mdc/osc stats files.
// Every named line emits stats_total unless it is read_bytes/write_bytes.
// Time-unit lines also emit stats_seconds_sum.
// read_bytes/write_bytes emit the existing GSI size metrics.
func ParseOBDStats(data []byte, source, component, target, typ string) ([]Observation, error) {
	var observations []Observation
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "snapshot_time" {
			continue
		}
		obs, err := parseOBDStatLine(fields, source, component, target, typ)
		if err != nil {
			return nil, err
		}
		observations = append(observations, obs...)
	}
	return observations, nil
}

// parseOBDStatLine parses the fields of one named stats line, which has at
// least a name and a count.
func parseOBDStatLine(fields []string, source, component, target, typ string) ([]Observation, error) {
	name := fields[0]
	if name == "read_bytes" || name == "write_bytes" {
		obs, err := parseBytesStat(fields, source, component, target)
		if err != nil {
			return nil, fmt.Errorf("obd stats %s: %w", name, err)
		}
		return obs, nil
	}
	count, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, fmt.Errorf("obd stats %s: parsing count: %w", name, err)
	}
	base := map[string]string{
		"component": component,
		"target":    target,
		"operation": name,
	}
	observations := []Observation{{
		Collector:  "client",
		Source:     source,
		MetricID:   "stats_total",
		MetricType: Counter,
		Labels:     base,
		Value:      count,
	}}
	seconds, ok := statsSecondsSum(fields)
	if !ok {
		return observations, nil
	}
	labels := map[string]string{
		"component": component,
		"target":    target,
		"type":      typ,
		"operation": name,
	}
	return append(observations, Observation{
		Collector:  "client",
		Source:     source,
		MetricID:   "stats_seconds_sum",
		MetricType: Counter,
		Labels:     labels,
		Value:      seconds,
	}), nil
}

// statsSecondsSum returns the sum of a time-unit stats line in seconds; ok is
// false when the line has no sum or its unit is not a time unit.
func statsSecondsSum(fields []string) (float64, bool) {
	unit, sum, ok := statsSum(fields)
	if !ok {
		return 0, false
	}
	return unitToSeconds(unit, sum)
}

// statsSum reads optional "samples [unit] min max sum [sumsq]".
func statsSum(fields []string) (unit string, sum float64, ok bool) {
	// name count samples [unit] min max sum [sumsq]
	if len(fields) < 7 || fields[2] != "samples" {
		return "", 0, false
	}
	unit = strings.Trim(fields[3], "[]")
	sum, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return "", 0, false
	}
	return unit, sum, true
}

func unitToSeconds(unit string, sum float64) (float64, bool) {
	switch strings.ToLower(unit) {
	case "nsec", "nsecs":
		return sum / 1e9, true
	case "usec", "usecs":
		return sum / 1e6, true
	case "msec", "msecs":
		return sum / 1e3, true
	case "sec", "secs":
		return sum, true
	default:
		return 0, false
	}
}
