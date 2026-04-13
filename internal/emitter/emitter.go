package emitter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
)

// Emit converts mapped observations into Prometheus metrics.
func Emit(mapped []mapper.MappedObservation) []prometheus.Metric {
	metrics := make([]prometheus.Metric, 0, len(mapped))
	for _, m := range mapped {
		var vt prometheus.ValueType
		switch m.MetricType {
		case parser.Counter:
			vt = prometheus.CounterValue
		case parser.Gauge:
			vt = prometheus.GaugeValue
		default:
			vt = prometheus.UntypedValue
		}

		desc := prometheus.NewDesc(m.Def.Name, m.Def.Help, m.LabelKeys, nil)
		metric := prometheus.MustNewConstMetric(desc, vt, m.Value, m.LabelVals...)
		metrics = append(metrics, metric)
	}
	return metrics
}
