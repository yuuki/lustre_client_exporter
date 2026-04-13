package parser

// MetricType represents the Prometheus metric type for an observation.
type MetricType int

const (
	Gauge   MetricType = iota
	Counter
	Untyped
)

// Observation is the normalized output of a parser. It carries no Prometheus
// dependency — the mapper and emitter layers handle the public metric contract.
type Observation struct {
	Collector  string
	Source     string
	MetricID   string
	MetricType MetricType
	Labels     map[string]string
	Value      float64
}
