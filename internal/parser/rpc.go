package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseRPCStats parses mdc/osc rpc_stats files.
// Extracts pages_per_rpc, rpcs_in_flight, and offset sections.
func ParseRPCStats(data []byte, source string, component string, target string, rpcType string) ([]Observation, error) {
	var observations []Observation

	lines := strings.Split(string(data), "\n")
	var section string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "snapshot_time") {
			continue
		}

		// Detect section headers
		switch {
		case strings.HasPrefix(line, "pages per rpc"):
			section = "pages_per_rpc"
			continue
		case strings.HasPrefix(line, "rpcs in flight"):
			section = "rpcs_in_flight"
			continue
		case strings.HasPrefix(line, "offset"):
			section = "rpcs_offset"
			continue
		case strings.Contains(line, "read") && strings.Contains(line, "write"):
			// Column header line
			continue
		}

		if section == "" {
			continue
		}

		obs, err := parseRPCLine(line, source, component, target, rpcType, section)
		if err != nil {
			continue // skip malformed lines
		}
		observations = append(observations, obs...)
	}

	return observations, nil
}

// parseRPCLine parses a data line like "1:  10  50  50  20  40  40"
func parseRPCLine(line, source, component, target, rpcType, section string) ([]Observation, error) {
	fields := strings.Fields(line)
	if len(fields) < 7 {
		return nil, fmt.Errorf("expected at least 7 fields, got %d", len(fields))
	}

	size := strings.TrimSuffix(fields[0], ":")
	readVal, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, err
	}
	writeIndex := 4
	if fields[writeIndex] == "|" {
		writeIndex++
	}
	if len(fields) <= writeIndex {
		return nil, fmt.Errorf("expected write value at field %d, got %d fields", writeIndex, len(fields))
	}
	writeVal, err := strconv.ParseFloat(fields[writeIndex], 64)
	if err != nil {
		return nil, err
	}

	var metricID string
	switch section {
	case "pages_per_rpc":
		metricID = "pages_per_rpc_total"
	case "rpcs_in_flight":
		metricID = "rpcs_in_flight"
	case "rpcs_offset":
		metricID = "rpcs_offset"
	}

	var observations []Observation
	for _, op := range []struct {
		name  string
		value float64
	}{
		{"read", readVal},
		{"write", writeVal},
	} {
		labels := map[string]string{
			"component": component,
			"target":    target,
			"operation": op.name,
			"size":      size,
		}

		if section == "rpcs_in_flight" {
			labels["type"] = rpcType
		}

		mt := Counter
		if section == "rpcs_in_flight" || section == "rpcs_offset" {
			mt = Gauge
		}

		observations = append(observations, Observation{
			Collector:  "client",
			Source:     source,
			MetricID:   metricID,
			MetricType: mt,
			Labels:     labels,
			Value:      op.value,
		})
	}

	return observations, nil
}
