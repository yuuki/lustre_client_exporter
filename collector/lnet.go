package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/emitter"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

type LNetCollector struct {
	reader     reader.Reader
	pathCfg    discovery.PathConfig
	source     discovery.LNetSource
	lnetctlBin string
	logger     *slog.Logger
}

func NewLNetCollector(r reader.Reader, cfg discovery.PathConfig, source discovery.LNetSource, lnetctlBin string, logger *slog.Logger) *LNetCollector {
	return &LNetCollector{
		reader:     r,
		pathCfg:    cfg,
		source:     source,
		lnetctlBin: lnetctlBin,
		logger:     logger,
	}
}

func (c *LNetCollector) Name() string { return "lnet" }

func (c *LNetCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	var allObs []parser.Observation

	switch c.source {
	case discovery.LNetSourceLNetCtl:
		obs, err := c.collectFromLNetCtl(ctx)
		if err != nil {
			return nil, err
		}
		allObs = append(allObs, obs...)
	case discovery.LNetSourceDebugFS:
		obs, err := c.collectFromDebugFS()
		if err != nil {
			return nil, err
		}
		allObs = append(allObs, obs...)
	default: // auto
		obs, err := c.collectFromDebugFS()
		if err != nil {
			c.logger.Debug("debugfs lnet not available, trying lnetctl", "error", err)
			obs, err = c.collectFromLNetCtl(ctx)
			if err != nil {
				return nil, err
			}
		}
		allObs = append(allObs, obs...)
	}

	paramObs := c.collectParams()
	allObs = append(allObs, paramObs...)

	mapped, err := mapper.Map(allObs)
	if err != nil {
		return nil, err
	}
	return emitter.Emit(mapped), nil
}

func (c *LNetCollector) collectFromDebugFS() ([]parser.Observation, error) {
	path := discovery.LNetStatsPath(c.pathCfg)
	data, err := c.reader.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parser.ParseLNetStats(data, path)
}

func (c *LNetCollector) collectFromLNetCtl(ctx context.Context) ([]parser.Observation, error) {
	data, err := c.reader.RunCommand(ctx, c.lnetctlBin, "stats", "show")
	if err != nil {
		return nil, err
	}
	return parser.ParseLNetCtlStats(data, "lnetctl stats show")
}

func (c *LNetCollector) collectParams() []parser.Observation {
	var allObs []parser.Observation
	for _, paramName := range discovery.LNetParamNames {
		path := discovery.LNetParamPath(c.pathCfg, paramName)
		data, err := c.reader.ReadFile(path)
		if err != nil {
			continue
		}
		obs, err := parser.ParseLNetParam(data, path, paramName)
		if err != nil {
			c.logger.Warn("failed to parse LNet param", "param", paramName, "error", err)
			continue
		}
		allObs = append(allObs, obs...)
	}
	return allObs
}
