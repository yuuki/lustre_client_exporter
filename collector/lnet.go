package collector

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/emitter"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

// LNetCollector reads LNet statistics from debugfs/procfs or lnetctl.
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

	// Collect params regardless of stats source
	paramObs, err := c.collectParams()
	if err != nil {
		c.logger.Warn("failed to collect LNet params", "error", err)
	} else {
		allObs = append(allObs, paramObs...)
	}

	mapped, err := mapper.Map(allObs)
	if err != nil {
		return nil, err
	}
	return emitter.Emit(mapped), nil
}

func (c *LNetCollector) collectFromDebugFS() ([]parser.Observation, error) {
	targets, err := discovery.DiscoverLNet(c.reader, c.pathCfg)
	if err != nil {
		return nil, err
	}
	if targets.StatsPath == "" {
		return nil, fmt.Errorf("lnet stats file not found")
	}

	data, err := c.reader.ReadFile(targets.StatsPath)
	if err != nil {
		return nil, err
	}
	return parser.ParseLNetStats(data, targets.StatsPath)
}

func (c *LNetCollector) collectFromLNetCtl(ctx context.Context) ([]parser.Observation, error) {
	data, err := c.reader.RunCommand(ctx, c.lnetctlBin, "stats", "show")
	if err != nil {
		return nil, err
	}
	return parser.ParseLNetCtlStats(data, "lnetctl stats show")
}

func (c *LNetCollector) collectParams() ([]parser.Observation, error) {
	targets, err := discovery.DiscoverLNet(c.reader, c.pathCfg)
	if err != nil {
		return nil, err
	}

	var allObs []parser.Observation
	for paramName, path := range targets.ParamPaths {
		data, err := c.reader.ReadFile(path)
		if err != nil {
			c.logger.Warn("failed to read LNet param", "param", paramName, "error", err)
			continue
		}
		obs, err := parser.ParseLNetParam(data, path, paramName)
		if err != nil {
			c.logger.Warn("failed to parse LNet param", "param", paramName, "error", err)
			continue
		}
		allObs = append(allObs, obs...)
	}
	return allObs, nil
}
