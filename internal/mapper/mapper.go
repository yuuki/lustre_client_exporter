package mapper

import (
	"fmt"

	"github.com/yuuki/lustre_client_exporter/internal/parser"
)

// MappedObservation is an observation enriched with the public metric definition.
type MappedObservation struct {
	Def       MetricDef
	LabelVals []string
	Value     float64
}

// Map converts internal observations to mapped observations using the Registry.
func Map(observations []parser.Observation) ([]MappedObservation, error) {
	result := make([]MappedObservation, 0, len(observations))
	for _, obs := range observations {
		def, ok := Registry[obs.MetricID]
		if !ok {
			return nil, fmt.Errorf("unknown metric ID: %s", obs.MetricID)
		}

		labelVals := make([]string, len(def.LabelKeys))
		for i, k := range def.LabelKeys {
			labelVals[i] = obs.Labels[k]
		}

		result = append(result, MappedObservation{
			Def:       def,
			LabelVals: labelVals,
			Value:     obs.Value,
		})
	}
	return result, nil
}
