package emitter

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
)

var (
	descCache   = map[string]*prometheus.Desc{}
	descCacheMu sync.Mutex
)

func getDesc(def mapper.MetricDef) *prometheus.Desc {
	descCacheMu.Lock()
	defer descCacheMu.Unlock()
	if d, ok := descCache[def.Name]; ok {
		return d
	}
	d := prometheus.NewDesc(def.Name, def.Help, def.LabelKeys, nil)
	descCache[def.Name] = d
	return d
}

// Emit converts mapped observations into Prometheus metrics.
func Emit(mapped []mapper.MappedObservation) []prometheus.Metric {
	metrics := make([]prometheus.Metric, 0, len(mapped))
	for _, m := range mapped {
		var vt prometheus.ValueType
		switch m.Def.Type {
		case parser.Counter:
			vt = prometheus.CounterValue
		case parser.Gauge:
			vt = prometheus.GaugeValue
		default:
			vt = prometheus.UntypedValue
		}

		desc := getDesc(m.Def)
		metric := prometheus.MustNewConstMetric(desc, vt, m.Value, m.LabelVals...)
		metrics = append(metrics, metric)
	}
	return metrics
}
