package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseLDLMCBDStats parses ldlm/services/ldlm_cbd/stats.
func ParseLDLMCBDStats(data []byte, source string) ([]Observation, error) {
	var obs []Observation

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "snapshot_time" || strings.HasSuffix(fields[0], "=") {
			continue
		}
		if len(fields) < 3 || fields[2] != "samples" {
			continue
		}

		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("ldlm_cbd stats: parsing %q value %q: %w", fields[0], fields[1], err)
		}

		obs = append(obs, Observation{
			Collector:  "client",
			Source:     source,
			MetricID:   "ldlm_cbd_stats",
			MetricType: Counter,
			Labels: map[string]string{
				"operation": fields[0],
			},
			Value: value,
		})
	}

	return obs, nil
}
