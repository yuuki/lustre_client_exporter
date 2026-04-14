package collector

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector defines the interface for Lustre metric collectors.
type Collector interface {
	Name() string
	Collect(ctx context.Context) ([]prometheus.Metric, error)
}

var (
	scrapeSuccessDesc = prometheus.NewDesc(
		"lustre_scrape_collector_success",
		"Whether the collector succeeded (1) or failed (0).",
		[]string{"collector"}, nil,
	)
	scrapeDurationDesc = prometheus.NewDesc(
		"lustre_scrape_collector_duration_seconds",
		"Duration of the collector scrape in seconds.",
		[]string{"collector"}, nil,
	)
	exporterScrapeDurationDesc = prometheus.NewDesc(
		"lustre_exporter_scrape_duration_seconds",
		"lustre_exporter: Duration of a scrape job.",
		[]string{"source", "result"}, nil,
	)
)

type scrapeDurationTotal struct {
	count uint64
	sum   float64
}

type scrapeDurationKey struct {
	source string
	result string
}

type scrapeSourceRecorderKey struct{}

type scrapeSourceRecorder func(string)

func recordScrapeSource(ctx context.Context, source string) {
	recorder, ok := ctx.Value(scrapeSourceRecorderKey{}).(scrapeSourceRecorder)
	if ok {
		recorder(source)
	}
}

// Registry implements prometheus.Collector by dispatching to registered Collectors.
type Registry struct {
	collectors    []Collector
	logger        *slog.Logger
	scrapeTimeout time.Duration
	sourceTimeout time.Duration
	strict        bool
	durationMu    sync.Mutex
	durations     map[scrapeDurationKey]scrapeDurationTotal
}

func NewRegistry(logger *slog.Logger, scrapeTimeout, sourceTimeout time.Duration, collectors ...Collector) *Registry {
	return NewRegistryWithStrict(logger, scrapeTimeout, sourceTimeout, false, collectors...)
}

func NewRegistryWithStrict(logger *slog.Logger, scrapeTimeout, sourceTimeout time.Duration, strict bool, collectors ...Collector) *Registry {
	return &Registry{
		collectors:    collectors,
		logger:        logger,
		scrapeTimeout: scrapeTimeout,
		sourceTimeout: sourceTimeout,
		strict:        strict,
		durations:     map[scrapeDurationKey]scrapeDurationTotal{},
	}
}

func (r *Registry) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeSuccessDesc
	ch <- scrapeDurationDesc
	ch <- exporterScrapeDurationDesc
}

func (r *Registry) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	if r.scrapeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.scrapeTimeout)
		defer cancel()
	}

	var wg sync.WaitGroup
	var scrapeDurationMu sync.Mutex
	scrapeDurations := map[scrapeDurationKey]float64{}

	for _, c := range r.collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()

			collectorCtx := ctx
			if r.sourceTimeout > 0 {
				var cancel context.CancelFunc
				collectorCtx, cancel = context.WithTimeout(ctx, r.sourceTimeout)
				defer cancel()
			}
			var scrapeSourceMu sync.Mutex
			scrapeSource := scrapeSourceName(c)
			collectorCtx = context.WithValue(collectorCtx, scrapeSourceRecorderKey{}, scrapeSourceRecorder(func(source string) {
				if source != "" {
					scrapeSourceMu.Lock()
					scrapeSource = source
					scrapeSourceMu.Unlock()
				}
			}))

			start := time.Now()
			metrics, err := c.Collect(collectorCtx)
			duration := time.Since(start).Seconds()

			success := 1.0
			result := "success"
			if err != nil {
				r.logger.Warn("collector failed", "collector", c.Name(), "error", err)
				success = 0.0
				result = "error"
				if r.strict {
					ch <- prometheus.NewInvalidMetric(scrapeSuccessDesc, fmt.Errorf("%s collector failed: %w", c.Name(), err))
				}
			} else {
				for _, m := range metrics {
					ch <- m
				}
			}

			ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, c.Name())
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration, c.Name())

			scrapeSourceMu.Lock()
			source := scrapeSource
			scrapeSourceMu.Unlock()

			key := scrapeDurationKey{source: source, result: result}
			scrapeDurationMu.Lock()
			scrapeDurations[key] += duration
			scrapeDurationMu.Unlock()
		}(c)
	}

	wg.Wait()

	for _, metric := range r.observeScrapeDurations(scrapeDurations) {
		ch <- metric
	}
}

func (r *Registry) observeScrapeDurations(scrapeDurations map[scrapeDurationKey]float64) []prometheus.Metric {
	keys := make([]scrapeDurationKey, 0, len(scrapeDurations))
	for key := range scrapeDurations {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source == keys[j].source {
			return keys[i].result < keys[j].result
		}
		return keys[i].source < keys[j].source
	})

	r.durationMu.Lock()
	defer r.durationMu.Unlock()

	metrics := make([]prometheus.Metric, 0, len(keys))
	for _, key := range keys {
		total := r.durations[key]
		total.count++
		total.sum += scrapeDurations[key]
		r.durations[key] = total

		metrics = append(metrics, prometheus.MustNewConstSummary(exporterScrapeDurationDesc, total.count, total.sum, nil, key.source, key.result))
	}
	return metrics
}

type scrapeSourceCollector interface {
	ScrapeSource() string
}

func scrapeSourceName(c Collector) string {
	if sourceCollector, ok := c.(scrapeSourceCollector); ok {
		return sourceCollector.ScrapeSource()
	}
	return c.Name()
}
