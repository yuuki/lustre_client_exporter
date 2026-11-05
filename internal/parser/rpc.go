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
	var section rpcStatsSection

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "snapshot_time") {
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

		// A subsection header such as "read RPCs in flight: 0" is also a
		// scalar. Neither headers nor scalars are histogram buckets.
		isHeader := section.applyHeader(line)
		if isHeader || isScalar || section.name == "" || !isRPCBucketLine(line) {
			continue
		}

		obs, err := parseRPCLine(line, source, component, target, rpcType, section.name, section.operation)
		if err != nil {
			return nil, fmt.Errorf("rpc stats %s: %q: %w", section.name, line, err)
		}
		observations = append(observations, obs...)
	}

	return observations, nil
}

// rpcStatsSection tracks the histogram that the bucket lines of an rpc_stats
// file belong to.
type rpcStatsSection struct {
	// name is "pages_per_rpc", "rpcs_in_flight" or "rpcs_offset", or empty
	// before the first section header.
	name string
	// operation labels the value of a bucket line without a write column;
	// empty means "read".
	operation string
	// pendingOperation is the operation that a lone "modify" line announces
	// for the next "rpcs in flight" header.
	pendingOperation string
}

// applyHeader updates s if line is a section, subsection or column header and
// reports whether it was one. The prefixes are checked before the column
// header test, so "offset  read  write" opens the offset section.
func (s *rpcStatsSection) applyHeader(line string) bool {
	switch {
	case line == "modify":
		s.pendingOperation = "modify"
	case strings.HasPrefix(line, "pages per rpc"):
		s.enter("pages_per_rpc", "")
	case strings.HasPrefix(line, "rpcs in flight"):
		s.enter("rpcs_in_flight", s.pendingOperation)
	case strings.HasPrefix(line, "dio read RPCs in flight:"):
		s.enter("rpcs_in_flight", "dio_read")
	case strings.HasPrefix(line, "dio write RPCs in flight:"):
		s.enter("rpcs_in_flight", "dio_write")
	case strings.HasPrefix(line, "read RPCs in flight:"):
		s.enter("rpcs_in_flight", "read")
	case strings.HasPrefix(line, "write RPCs in flight:"):
		s.enter("rpcs_in_flight", "write")
	case strings.HasPrefix(line, "offset"):
		s.enter("rpcs_offset", "")
	case strings.Contains(line, "read") && strings.Contains(line, "write"):
		// Column header line
		s.pendingOperation = ""
	default:
		return false
	}
	return true
}

// enter starts the section name, whose single-column buckets belong to
// operation, and drops any pending operation.
func (s *rpcStatsSection) enter(name, operation string) {
	s.name = name
	s.operation = operation
	s.pendingOperation = ""
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
	writeVal, hasWrite, err := parseRPCWriteValue(fields)
	if err != nil {
		return nil, err
	}

	metricID, mt := rpcSectionMetric(section)
	var observations []Observation
	for _, op := range rpcLineOperations(readVal, writeVal, hasWrite, singleOperation) {
		labels := map[string]string{
			"component": component,
			"target":    target,
			"operation": op.name,
			"size":      size,
		}

		if section == "rpcs_in_flight" {
			labels["type"] = rpcType
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

// parseRPCWriteValue parses the write column of a bucket line: the field
// after the three read columns and an optional "|". ok is false when the line
// has no write column.
func parseRPCWriteValue(fields []string) (value float64, ok bool, err error) {
	writeIndex := 4
	if len(fields) > writeIndex && fields[writeIndex] == "|" {
		writeIndex++
	}
	if len(fields) <= writeIndex {
		return 0, false, nil
	}
	value, err = strconv.ParseFloat(fields[writeIndex], 64)
	return value, true, err
}

// rpcOperationValue is one value column of a bucket line.
type rpcOperationValue struct {
	name  string
	value float64
}

// rpcLineOperations returns the values a bucket line reports: read and write
// when it has a write column, otherwise its read value under singleOperation,
// or under "read" when the section has no single operation.
func rpcLineOperations(readVal, writeVal float64, hasWrite bool, singleOperation string) []rpcOperationValue {
	switch {
	case hasWrite:
		return []rpcOperationValue{{"read", readVal}, {"write", writeVal}}
	case singleOperation != "":
		return []rpcOperationValue{{singleOperation, readVal}}
	default:
		return []rpcOperationValue{{"read", readVal}}
	}
}

// rpcSectionMetric returns the metric ID and type of the buckets of section.
func rpcSectionMetric(section string) (string, MetricType) {
	switch section {
	case "pages_per_rpc":
		return "pages_per_rpc_total", Counter
	case "rpcs_in_flight":
		return "rpcs_in_flight", Gauge
	case "rpcs_offset":
		return "rpcs_offset", Gauge
	default:
		return "", Counter
	}
}
