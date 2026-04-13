package collector

import (
	"context"
	"log/slog"
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
)

// Registry implements prometheus.Collector by dispatching to registered Collectors.
type Registry struct {
	collectors    []Collector
	logger        *slog.Logger
	scrapeTimeout time.Duration
	sourceTimeout time.Duration
}

func NewRegistry(logger *slog.Logger, scrapeTimeout, sourceTimeout time.Duration, collectors ...Collector) *Registry {
	return &Registry{
		collectors:    collectors,
		logger:        logger,
		scrapeTimeout: scrapeTimeout,
		sourceTimeout: sourceTimeout,
	}
}

func (r *Registry) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeSuccessDesc
	ch <- scrapeDurationDesc
}

func (r *Registry) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	if r.scrapeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.scrapeTimeout)
		defer cancel()
	}

	var wg sync.WaitGroup

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

			start := time.Now()
			metrics, err := c.Collect(collectorCtx)
			duration := time.Since(start).Seconds()

			success := 1.0
			if err != nil {
				r.logger.Warn("collector failed", "collector", c.Name(), "error", err)
				success = 0.0
			} else {
				for _, m := range metrics {
					ch <- m
				}
			}

			ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, c.Name())
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration, c.Name())
		}(c)
	}

	wg.Wait()
}
