package emitter

import (
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_client_exporter/internal/mapper"
	"github.com/yuuki/lustre_client_exporter/internal/parser"
)

var (
	descCache   = map[string]*prometheus.Desc{}
	descCacheMu sync.RWMutex
)

func getDesc(def mapper.MetricDef) *prometheus.Desc {
	key := def.Name + "\xff" + strings.Join(def.LabelKeys, "\xff")
	descCacheMu.RLock()
	if d, ok := descCache[key]; ok {
		descCacheMu.RUnlock()
		return d
	}
	descCacheMu.RUnlock()

	descCacheMu.Lock()
	defer descCacheMu.Unlock()
	if d, ok := descCache[key]; ok {
		return d
	}
	d := prometheus.NewDesc(def.Name, def.Help, def.LabelKeys, nil)
	descCache[key] = d
	return d
}

// Emit converts mapped observations into Prometheus metrics. An observation
// that cannot form a valid series, such as one whose label value read from a
// corrupt source is not valid UTF-8, fails the whole call with an error rather
// than a panic: collectors run in goroutines of the scrape, where a panic
// would crash the exporter.
func Emit(mapped []mapper.MappedObservation) ([]prometheus.Metric, error) {
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
		metric, err := prometheus.NewConstMetric(desc, vt, m.Value, m.LabelVals...)
		if err != nil {
			return nil, fmt.Errorf("emit %s: %w", m.Def.Name, err)
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}
