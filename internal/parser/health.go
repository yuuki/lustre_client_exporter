package parser

import "strings"

// ParseHealth parses the content of /sys/fs/lustre/health_check.
// Returns a single observation: 1 for "healthy", 0 for anything else.
func ParseHealth(data []byte, source string) ([]Observation, error) {
	content := strings.TrimSpace(string(data))
	value := 0.0
	if content == "healthy" {
		value = 1.0
	}
	return []Observation{
		{
			Collector:  "health",
			Source:     source,
			MetricID:   "health_check",
			MetricType: Gauge,
			Labels:     nil,
			Value:      value,
		},
	}, nil
}
