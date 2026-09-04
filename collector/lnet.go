package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_client_exporter/internal/discovery"
	"github.com/yuuki/lustre_client_exporter/internal/emitter"
	"github.com/yuuki/lustre_client_exporter/internal/mapper"
	"github.com/yuuki/lustre_client_exporter/internal/parser"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
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

func (c *LNetCollector) ScrapeSource() string {
	if c.source == discovery.LNetSourceLNetCtl {
		return "lctl"
	}
	return "sys"
}

func (c *LNetCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	var allObs []parser.Observation

	switch c.source {
	case discovery.LNetSourceLNetCtl:
		recordScrapeSource(ctx, "lctl")
		obs, err := c.collectFromLNetCtl(ctx)
		if err != nil {
			return nil, err
		}
		allObs = append(allObs, obs...)
	case discovery.LNetSourceDebugFS:
		recordScrapeSource(ctx, "sys")
		obs, err := c.collectFromDebugFS(ctx)
		if err != nil {
			return nil, err
		}
		allObs = append(allObs, obs...)
	default: // auto
		recordScrapeSource(ctx, "sys")
		obs, err := c.collectFromDebugFS(ctx)
		if err != nil {
			c.logger.Debug("debugfs lnet not available, trying lnetctl", "error", err)
			recordScrapeSource(ctx, "lctl")
			obs, err = c.collectFromLNetCtl(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			obs = append(obs, c.collectSupplementalNetNI(ctx)...)
		}
		allObs = append(allObs, obs...)
	}

	paramObs := c.collectParams(ctx)
	allObs = append(allObs, paramObs...)

	mapped, err := mapper.Map(allObs)
	if err != nil {
		return nil, err
	}
	return emitter.Emit(mapped), nil
}

func (c *LNetCollector) collectFromDebugFS(ctx context.Context) ([]parser.Observation, error) {
	data, path, err := reader.ReadFirstAvailable(ctx, c.reader, discovery.LNetStatsPaths(c.pathCfg))
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
	obs, err := parser.ParseLNetCtlStats(data, "lnetctl stats show")
	if err != nil {
		return nil, err
	}

	netData, err := c.reader.RunCommand(ctx, c.lnetctlBin, "net", "show", "-v", "3")
	if err != nil {
		c.logger.Debug("lnetctl net show -v 3 not available", "error", err)
		netData, err = c.reader.RunCommand(ctx, c.lnetctlBin, "net", "show")
		if err != nil {
			c.logger.Debug("lnetctl net show not available", "error", err)
			return obs, nil
		}
	}
	countObs, err := parser.ParseLNetCtlNetStats(netData, "lnetctl net show")
	if err != nil {
		c.logger.Warn("failed to parse lnetctl net show", "error", err)
		return obs, nil
	}
	niObs, err := parser.ParseLNetCtlNetNI(netData, "lnetctl net show")
	if err != nil {
		c.logger.Warn("failed to parse lnetctl net show NI", "error", err)
		niObs = nil
	}
	if len(countObs) > 0 {
		obs = dropGlobalLNetCountStats(obs)
		obs = append(obs, countObs...)
	}
	return append(obs, niObs...), nil
}

func (c *LNetCollector) collectSupplementalNetNI(ctx context.Context) []parser.Observation {
	netData, err := c.reader.RunCommand(ctx, c.lnetctlBin, "net", "show", "-v", "3")
	if err != nil {
		c.logger.Debug("lnetctl net show -v 3 not available", "error", err)
		netData, err = c.reader.RunCommand(ctx, c.lnetctlBin, "net", "show")
		if err != nil {
			c.logger.Debug("lnetctl net show not available", "error", err)
			return nil
		}
	}
	niObs, err := parser.ParseLNetCtlNetNI(netData, "lnetctl net show")
	if err != nil {
		c.logger.Warn("failed to parse lnetctl net show NI", "error", err)
		return nil
	}
	return niObs
}

func dropGlobalLNetCountStats(obs []parser.Observation) []parser.Observation {
	filtered := make([]parser.Observation, 0, len(obs))
	for _, o := range obs {
		switch o.MetricID {
		case "send_count_total", "receive_count_total", "drop_count_total":
			continue
		}
		filtered = append(filtered, o)
	}
	return filtered
}

func (c *LNetCollector) collectParams(ctx context.Context) []parser.Observation {
	var allObs []parser.Observation
	for _, paramName := range discovery.LNetParamNames {
		data, path, err := reader.ReadFirstAvailable(ctx, c.reader, discovery.LNetParamPaths(c.pathCfg, paramName))
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
