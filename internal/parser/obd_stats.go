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
		name := fields[0]
		if name == "read_bytes" || name == "write_bytes" {
			obs, err := parseBytesStat(fields, source, component, target)
			if err != nil {
				return nil, fmt.Errorf("obd stats %s: %w", name, err)
			}
			observations = append(observations, obs...)
			continue
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
		observations = append(observations, Observation{
			Collector:  "client",
			Source:     source,
			MetricID:   "stats_total",
			MetricType: Counter,
			Labels:     base,
			Value:      count,
		})
		unit, sum, ok := statsSum(fields)
		if !ok {
			continue
		}
		seconds, ok := unitToSeconds(unit, sum)
		if !ok {
			continue
		}
		labels := map[string]string{
			"component": component,
			"target":    target,
			"type":      typ,
			"operation": name,
		}
		observations = append(observations, Observation{
			Collector:  "client",
			Source:     source,
			MetricID:   "stats_seconds_sum",
			MetricType: Counter,
			Labels:     labels,
			Value:      seconds,
		})
	}
	return observations, nil
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
