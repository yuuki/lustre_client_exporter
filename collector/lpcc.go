package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/emitter"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

type LpccCollector struct {
	reader  reader.Reader
	lpccBin string
	logger  *slog.Logger
}

func NewLpccCollector(r reader.Reader, lpccBin string, logger *slog.Logger) *LpccCollector {
	return &LpccCollector{
		reader:  r,
		lpccBin: lpccBin,
		logger:  logger,
	}
}

func (c *LpccCollector) Name() string { return "lpcc" }

func (c *LpccCollector) ScrapeSource() string { return "lpcc" }

func (c *LpccCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	data, err := c.reader.RunCommand(ctx, c.lpccBin, "status")
	if err != nil {
		c.logger.Warn("lpcc command failed", "error", err)
		return nil, err
	}

	obs, err := parser.ParseLpccStatus(data, "lpcc status")
	if err != nil {
		return nil, err
	}

	mapped, err := mapper.Map(obs)
	if err != nil {
		return nil, err
	}
	return emitter.Emit(mapped), nil
}
