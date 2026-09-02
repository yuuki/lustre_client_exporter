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
	var sectionOperation string
	var pendingSectionOperation string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "snapshot_time") {
			continue
		}

		metricID, operation, value, isScalar := parseRPCScalar(line)
		if isScalar {
			observations = append(observations, Observation{
				Collector:  "client",
				Source:     source,
				MetricID:   metricID,
				MetricType: Gauge,
				Labels: map[string]string{
					"component": component,
					"target":    target,
					"type":      rpcType,
					"operation": operation,
				},
				Value: value,
			})
		}

		// Detect section headers
		switch {
		case line == "modify":
			pendingSectionOperation = "modify"
			continue
		case strings.HasPrefix(line, "pages per rpc"):
			section = "pages_per_rpc"
			sectionOperation = ""
			pendingSectionOperation = ""
			continue
		case strings.HasPrefix(line, "rpcs in flight"):
			section = "rpcs_in_flight"
			sectionOperation = pendingSectionOperation
			pendingSectionOperation = ""
			continue
		case strings.HasPrefix(line, "dio read RPCs in flight:"):
			section = "rpcs_in_flight"
			sectionOperation = "dio_read"
			pendingSectionOperation = ""
			continue
		case strings.HasPrefix(line, "dio write RPCs in flight:"):
			section = "rpcs_in_flight"
			sectionOperation = "dio_write"
			pendingSectionOperation = ""
			continue
		case strings.HasPrefix(line, "read RPCs in flight:"):
			section = "rpcs_in_flight"
			sectionOperation = "read"
			pendingSectionOperation = ""
			continue
		case strings.HasPrefix(line, "write RPCs in flight:"):
			section = "rpcs_in_flight"
			sectionOperation = "write"
			pendingSectionOperation = ""
			continue
		case strings.HasPrefix(line, "offset"):
			section = "rpcs_offset"
			sectionOperation = ""
			pendingSectionOperation = ""
			continue
		case strings.Contains(line, "read") && strings.Contains(line, "write"):
			// Column header line
			pendingSectionOperation = ""
			continue
		}

		if isScalar {
			// pending/modify scalars are not histogram section headers
			continue
		}

		if section == "" {
			continue
		}
		if !isRPCBucketLine(line) {
			continue
		}

		obs, err := parseRPCLine(line, source, component, target, rpcType, section, sectionOperation)
		if err != nil {
			return nil, fmt.Errorf("rpc stats %s: %q: %w", section, line, err)
		}
		observations = append(observations, obs...)
	}

	return observations, nil
}

func parseRPCScalar(line string) (metricID, operation string, value float64, ok bool) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return "", "", 0, false
	}
	label := strings.ToLower(strings.TrimSpace(line[:colon]))
	fields := strings.Fields(line[colon+1:])
	if len(fields) == 0 {
		return "", "", 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", "", 0, false
	}
	switch label {
	case "modify_rpcs_in_flight":
		return "rpcs_current", "modify", v, true
	case "dio read rpcs in flight":
		return "rpcs_current", "dio_read", v, true
	case "dio write rpcs in flight":
		return "rpcs_current", "dio_write", v, true
	case "read rpcs in flight":
		return "rpcs_current", "read", v, true
	case "write rpcs in flight":
		return "rpcs_current", "write", v, true
	case "pending read pages":
		return "pending_pages", "read", v, true
	case "pending write pages":
		return "pending_pages", "write", v, true
	default:
		return "", "", 0, false
	}
}

func isRPCBucketLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	return strings.HasSuffix(fields[0], ":")
}

// parseRPCLine parses a data line like "1:  10  50  50  20  40  40"
func parseRPCLine(line, source, component, target, rpcType, section, singleOperation string) ([]Observation, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, fmt.Errorf("expected at least 2 fields, got %d", len(fields))
	}

	size := strings.TrimSuffix(fields[0], ":")
	readVal, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, err
	}
	writeIndex := 4
	if len(fields) > writeIndex && fields[writeIndex] == "|" {
		writeIndex++
	}
	var writeVal float64
	hasWrite := len(fields) > writeIndex
	if hasWrite {
		writeVal, err = strconv.ParseFloat(fields[writeIndex], 64)
		if err != nil {
			return nil, err
		}
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
	operations := []struct {
		name  string
		value float64
	}{
		{"read", readVal},
		{"write", writeVal},
	}
	if !hasWrite && singleOperation != "" {
		operations = []struct {
			name  string
			value float64
		}{
			{singleOperation, readVal},
		}
	}
	for _, op := range operations {
		if op.name == "write" && !hasWrite && singleOperation == "" {
			continue
		}

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
