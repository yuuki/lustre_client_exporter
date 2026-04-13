package mapper

import (
	"fmt"

	"github.com/yuuki/lustre_exporter/internal/parser"
)

// MappedObservation is an observation enriched with the public metric definition.
type MappedObservation struct {
	Def        MetricDef
	Labels     map[string]string
	LabelKeys  []string
	LabelVals  []string
	Value      float64
	MetricType parser.MetricType
}

// Map converts internal observations to mapped observations using the Registry.
func Map(observations []parser.Observation) ([]MappedObservation, error) {
	result := make([]MappedObservation, 0, len(observations))
	for _, obs := range observations {
		def, ok := Registry[obs.MetricID]
		if !ok {
			return nil, fmt.Errorf("unknown metric ID: %s", obs.MetricID)
		}

		labelKeys := make([]string, 0, len(def.LabelKeys))
		labelVals := make([]string, 0, len(def.LabelKeys))
		for _, k := range def.LabelKeys {
			labelKeys = append(labelKeys, k)
			labelVals = append(labelVals, obs.Labels[k])
		}

		result = append(result, MappedObservation{
			Def:        def,
			Labels:     obs.Labels,
			LabelKeys:  labelKeys,
			LabelVals:  labelVals,
			Value:      obs.Value,
			MetricType: obs.MetricType,
		})
	}
	return result, nil
}
