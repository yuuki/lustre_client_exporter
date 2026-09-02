package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// clientSingleFileMap maps OSC/MDC single-value files to MetricIDs.
var clientSingleFileMap = map[string]struct {
	metricID string
	hasType  bool
	scale    float64
}{
	"cur_dirty_bytes":        {"osc_dirty_bytes", false, 1},
	"max_dirty_mb":           {"osc_max_dirty_bytes", false, 1024 * 1024},
	"max_pages_per_rpc":      {"max_pages_per_rpc", true, 1},
	"max_rpcs_in_flight":     {"max_rpcs_in_flight", true, 1},
	"max_mod_rpcs_in_flight": {"max_mod_rpcs_in_flight", true, 1},
	"active":                 {"target_active", true, 1},
}

// ParseClientSingleFile parses a single-value OSC/MDC file (dirty bytes, RPC limits, active).
func ParseClientSingleFile(data []byte, source, fileName, component, target, typ string) ([]Observation, error) {
	entry, ok := clientSingleFileMap[fileName]
	if !ok {
		return nil, nil
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return nil, fmt.Errorf("client %q: %w", fileName, err)
	}
	val *= entry.scale

	labels := map[string]string{
		"component": component,
		"target":    target,
	}
	if entry.hasType {
		labels["type"] = typ
	}

	return []Observation{
		{
			Collector:  "client",
			Source:     source,
			MetricID:   entry.metricID,
			MetricType: Gauge,
			Labels:     labels,
			Value:      val,
		},
	}, nil
}
