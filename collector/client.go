package collector

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yuuki/lustre_client_exporter/internal/discovery"
	"github.com/yuuki/lustre_client_exporter/internal/emitter"
	"github.com/yuuki/lustre_client_exporter/internal/mapper"
	"github.com/yuuki/lustre_client_exporter/internal/parser"
	"github.com/yuuki/lustre_client_exporter/internal/reader"
)

// lliteSingleFiles lists files read individually for each llite mount.
var lliteSingleFiles = []string{
	"blocksize", "filesfree", "filestotal",
	"kbytesavail", "kbytesfree", "kbytestotal",
	"checksum_pages", "default_easize", "lazystatfs",
	"max_easize", "max_read_ahead_mb", "max_read_ahead_per_file_mb",
	"max_read_ahead_whole_mb", "statahead_agl", "statahead_max", "xattr_cache",
}

// oscParamFiles are optional OSC writeback / RPC / import files under ParamRoots.
var oscParamFiles = []string{
	"cur_dirty_bytes", "max_dirty_mb", "max_pages_per_rpc", "max_rpcs_in_flight", "active", "state",
}

// mdcParamFiles are optional MDC RPC / import files under ParamRoots.
// max_pages_per_rpc is attempted; skip if the file is absent.
var mdcParamFiles = []string{
	"max_rpcs_in_flight", "max_mod_rpcs_in_flight", "max_pages_per_rpc", "active", "state",
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

func (c *ClientCollector) ScrapeSource() string { return "procfs" }

func (c *ClientCollector) Collect(ctx context.Context) ([]prometheus.Metric, error) {
	targets, err := discovery.DiscoverClients(ctx, c.reader, c.pathCfg)
	if err != nil {
		return nil, err
	}

	var allObs []parser.Observation

	ldlmObs, err := c.collectLDLMCBD(ctx)
	switch {
	case err == nil:
		allObs = append(allObs, ldlmObs...)
	case c.strict:
		return nil, err
	default:
		c.logger.Warn("ldlm_cbd stats collection failed", "error", err)
	}

	for _, t := range targets {
		obs, err := c.collectTarget(ctx, t)
		if err != nil {
			return nil, err
		}
		allObs = append(allObs, obs...)
	}

	mapped, err := mapper.Map(allObs)
	if err != nil {
		return nil, err
	}
	return emitter.Emit(mapped)
}

// collectTarget collects one discovered target according to its component.
// A failed target fails the scrape in strict mode and is skipped otherwise.
func (c *ClientCollector) collectTarget(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	switch t.Component {
	case "llite":
		obs, err := c.collectLLite(ctx, t)
		if err != nil {
			return nil, c.strictError(err, "llite collection failed", "target", t.Name)
		}
		return obs, nil
	case "mdc", "osc":
		obs, err := c.collectRPC(ctx, t)
		if err != nil {
			return nil, c.strictError(err, "rpc collection failed", "component", t.Component, "target", t.Name)
		}
		return obs, nil
	}
	return nil, nil
}

// strictError returns err in strict mode. Otherwise it logs err as a warning
// with msg and the key-value pairs in args and returns nil, so the caller
// skips the failed source and collection continues.
func (c *ClientCollector) strictError(err error, msg string, args ...any) error {
	if c.strict {
		return err
	}
	c.logger.Warn(msg, append(args, "error", err)...)
	return nil
}

func (c *ClientCollector) collectLDLMCBD(ctx context.Context) ([]parser.Observation, error) {
	data, path, err := reader.ReadFirstAvailable(ctx, c.reader, discovery.LDLMCBDStatsPaths(c.pathCfg))
	if err != nil {
		c.logger.Debug("ldlm_cbd stats not available", "error", err)
		return nil, nil
	}
	return parser.ParseLDLMCBDStats(data, path)
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
		data, path, err := c.readParamFile(ctx, t, name)
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

func (c *ClientCollector) readParamFile(ctx context.Context, t discovery.ClientTarget, name string) ([]byte, string, error) {
	var lastErr error
	for _, root := range t.ParamRoots {
		path := filepath.Join(root, name)
		data, err := c.reader.ReadFile(ctx, path)
		if err != nil {
			lastErr = err
			continue
		}
		return data, path, nil
	}
	if lastErr == nil {
		return nil, "", fmt.Errorf("param file %s not found", name)
	}
	return nil, "", lastErr
}

// collectRPC collects an mdc or osc target: its stats, its rpc_stats and its
// optional param files, in that order.
func (c *ClientCollector) collectRPC(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	var allObs []parser.Observation

	obs, err := c.collectOBDStats(ctx, t)
	if err != nil {
		return nil, err
	}
	allObs = append(allObs, obs...)

	obs, err = c.collectRPCStats(ctx, t)
	if err != nil {
		return nil, err
	}
	allObs = append(allObs, obs...)

	obs, err = c.collectClientParams(ctx, t)
	if err != nil {
		return nil, err
	}
	allObs = append(allObs, obs...)

	return allObs, nil
}

// collectOBDStats reads and parses the target's stats file, if one was discovered.
func (c *ClientCollector) collectOBDStats(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	if t.StatsPath == "" {
		return nil, nil
	}
	data, err := c.reader.ReadFile(ctx, t.StatsPath)
	if err != nil {
		return nil, c.strictError(err, "obd stats read failed", "component", t.Component, "target", t.Name)
	}
	obs, err := parser.ParseOBDStats(data, t.StatsPath, "client", t.Name, t.Component)
	if err != nil {
		return nil, c.strictError(err, "failed to parse obd stats", "component", t.Component, "target", t.Name)
	}
	return obs, nil
}

// collectRPCStats reads and parses the target's rpc_stats file, if one was
// discovered. A file that has disappeared since discovery is not an error,
// even in strict mode.
func (c *ClientCollector) collectRPCStats(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	if t.RpcStatsPath == "" {
		return nil, nil
	}
	data, err := c.reader.ReadFile(ctx, t.RpcStatsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			c.logger.Debug("rpc_stats not found", "component", t.Component, "target", t.Name, "path", t.RpcStatsPath, "error", err)
			return nil, nil
		}
		return nil, c.strictError(err, "rpc_stats read failed", "component", t.Component, "target", t.Name)
	}
	obs, err := parser.ParseRPCStats(data, t.RpcStatsPath, "client", t.Name, t.Component)
	if err != nil {
		return nil, c.strictError(err, "failed to parse rpc stats", "component", t.Component, "target", t.Name)
	}
	return obs, nil
}

// collectClientParams reads the optional mdc or osc param files of the target.
// An absent or unreadable file is skipped, even in strict mode; a malformed one
// fails only in strict mode.
func (c *ClientCollector) collectClientParams(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	var allObs []parser.Observation
	for _, name := range clientParamFiles(t.Component) {
		data, path, err := c.readParamFile(ctx, t, name)
		if err != nil {
			c.logger.Debug("client param file not found", "file", name, "component", t.Component, "target", t.Name)
			continue
		}
		obs, err := parseClientParamFile(data, path, name, t)
		if err != nil {
			if c.strict {
				return nil, err
			}
			c.logger.Warn("failed to parse client param file", "file", name, "component", t.Component, "target", t.Name, "error", err)
			continue
		}
		allObs = append(allObs, obs...)
	}
	return allObs, nil
}

// clientParamFiles returns the param files read for an mdc or osc target.
func clientParamFiles(component string) []string {
	if component == "mdc" {
		return mdcParamFiles
	}
	return oscParamFiles
}

// parseClientParamFile parses one mdc or osc param file: the import state, or
// a single-value file.
func parseClientParamFile(data []byte, path, name string, t discovery.ClientTarget) ([]parser.Observation, error) {
	if name == "state" {
		return parser.ParseClientState(data, path, "client", t.Name, t.Component)
	}
	return parser.ParseClientSingleFile(data, path, name, "client", t.Name, t.Component)
}
