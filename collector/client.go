package collector

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_exporter/internal/discovery"
	"github.com/yuuki/lustre_exporter/internal/emitter"
	"github.com/yuuki/lustre_exporter/internal/mapper"
	"github.com/yuuki/lustre_exporter/internal/parser"
	"github.com/yuuki/lustre_exporter/internal/reader"
)

// lliteSingleFiles lists files read individually for each llite mount.
var lliteSingleFiles = []string{
	"blocksize", "filesfree", "filestotal",
	"kbytesavail", "kbytesfree", "kbytestotal",
	"checksum_pages", "default_easize", "lazystatfs",
	"max_easize", "max_read_ahead_mb", "max_read_ahead_per_file_mb",
	"max_read_ahead_whole_mb", "statahead_agl", "statahead_max", "xattr_cache",
}

// ClientCollector reads llite, mdc, and osc metrics.
type ClientCollector struct {
	reader  reader.Reader
	pathCfg discovery.PathConfig
	logger  *slog.Logger
	strict  bool
}

func NewClientCollector(r reader.Reader, cfg discovery.PathConfig, logger *slog.Logger) *ClientCollector {
	return NewClientCollectorWithStrict(r, cfg, logger, false)
}

func NewClientCollectorWithStrict(r reader.Reader, cfg discovery.PathConfig, logger *slog.Logger, strict bool) *ClientCollector {
	return &ClientCollector{reader: r, pathCfg: cfg, logger: logger, strict: strict}
}

func (c *ClientCollector) Name() string { return "client" }

func (c *ClientCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	targets, err := discovery.DiscoverClients(ctx, c.reader, c.pathCfg)
	if err != nil {
		return nil, err
	}

	var allObs []parser.Observation

	for _, t := range targets {
		switch t.Component {
		case "llite":
			obs, err := c.collectLLite(ctx, t)
			if err != nil {
				if c.strict {
					return nil, err
				}
				c.logger.Warn("llite collection failed", "target", t.Name, "error", err)
				continue
			}
			allObs = append(allObs, obs...)
		case "mdc", "osc":
			obs, err := c.collectRPC(ctx, t)
			if err != nil {
				if c.strict {
					return nil, err
				}
				c.logger.Warn("rpc collection failed", "component", t.Component, "target", t.Name, "error", err)
				continue
			}
			allObs = append(allObs, obs...)
		}
	}

	mapped, err := mapper.Map(allObs)
	if err != nil {
		return nil, err
	}
	return emitter.Emit(mapped), nil
}

func (c *ClientCollector) collectLLite(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	var allObs []parser.Observation

	// Parse stats file
	data, err := c.reader.ReadFile(ctx, t.StatsPath)
	if err != nil {
		return nil, err
	}
	obs, err := parser.ParseLLiteStats(data, t.StatsPath, "client", t.Name)
	if err != nil {
		return nil, err
	}
	allObs = append(allObs, obs...)

	// Parse single-value files
	for _, name := range lliteSingleFiles {
		path := filepath.Join(t.BasePath, name)
		data, err := c.reader.ReadFile(ctx, path)
		if err != nil {
			c.logger.Debug("llite file not found", "file", name, "target", t.Name)
			continue
		}
		obs, err := parser.ParseLLiteSingleFile(data, path, name, "client", t.Name)
		if err != nil {
			if c.strict {
				return nil, err
			}
			c.logger.Warn("failed to parse llite file", "file", name, "error", err)
			continue
		}
		allObs = append(allObs, obs...)
	}

	return allObs, nil
}

func (c *ClientCollector) collectRPC(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	var allObs []parser.Observation

	// Parse rpc_stats if available
	if t.RpcStatsPath != "" {
		data, err := c.reader.ReadFile(ctx, t.RpcStatsPath)
		if err != nil {
			c.logger.Debug("rpc_stats not available", "component", t.Component, "target", t.Name)
		} else {
			obs, err := parser.ParseRPCStats(data, t.RpcStatsPath, "client", t.Name, t.Component)
			if err != nil {
				return nil, err
			}
			allObs = append(allObs, obs...)
		}
	}

	return allObs, nil
}
