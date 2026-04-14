package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/emitter"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

type HealthCollector struct {
	reader  reader.Reader
	pathCfg discovery.PathConfig
}

func NewHealthCollector(r reader.Reader, cfg discovery.PathConfig) *HealthCollector {
	return &HealthCollector{reader: r, pathCfg: cfg}
}

func (c *HealthCollector) Name() string { return "health" }

func (c *HealthCollector) ScrapeSource() string { return "sysfs" }

func (c *HealthCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	path := discovery.HealthPath(c.pathCfg)

	data, err := c.reader.ReadFile(ctx, path)
	if err != nil {
		return nil, err
	}

	observations, err := parser.ParseHealth(data, path)
	if err != nil {
		return nil, err
	}

	mapped, err := mapper.Map(observations)
	if err != nil {
		return nil, err
	}

	return emitter.Emit(mapped), nil
}
