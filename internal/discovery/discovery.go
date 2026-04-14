package discovery

import (
	"context"
	"path/filepath"

	"github.com/yuuki/lustre_exporter/internal/reader"
)

// PathConfig holds base paths for Lustre filesystem locations.
type PathConfig struct {
	ProcFS  string // default: /proc
	SysFS   string // default: /sys
	DebugFS string // default: /sys/kernel/debug
}

// DefaultPathConfig returns the standard Linux paths.
func DefaultPathConfig() PathConfig {
	return PathConfig{
		ProcFS:  "/proc",
		SysFS:   "/sys",
		DebugFS: "/sys/kernel/debug",
	}
}

// Target represents a discovered file or command to read metrics from.
type Target struct {
	Path string
	Name string // human-readable target identifier (e.g., mount name)
}

// HealthPath returns the path to the Lustre health_check file.
func HealthPath(cfg PathConfig) string {
	return filepath.Join(cfg.SysFS, "fs", "lustre", "health_check")
}

// sptlrpcPath returns the debugfs path to the sptlrpc encrypt_page_pools file.
func sptlrpcPath(cfg PathConfig) string {
	return filepath.Join(cfg.DebugFS, "lustre", "sptlrpc", "encrypt_page_pools")
}

// SptlrpcPaths returns known locations for the sptlrpc encrypt_page_pools file.
func SptlrpcPaths(cfg PathConfig) []string {
	return []string{
		sptlrpcPath(cfg),
		filepath.Join(cfg.ProcFS, "fs", "lustre", "sptlrpc", "encrypt_page_pools"),
	}
}

// LDLMCBDStatsPaths returns known locations for LDLM callback service stats.
func LDLMCBDStatsPaths(cfg PathConfig) []string {
	return []string{
		filepath.Join(cfg.DebugFS, "lustre", "ldlm", "services", "ldlm_cbd", "stats"),
		filepath.Join(cfg.ProcFS, "fs", "lustre", "ldlm", "services", "ldlm_cbd", "stats"),
	}
}

// LNetSource represents the source to use for LNet stats.
type LNetSource int

const (
	LNetSourceAuto LNetSource = iota
	LNetSourceDebugFS
	LNetSourceLNetCtl
)

// lnetStatsPath returns the legacy procfs path to the LNet stats file.
func lnetStatsPath(cfg PathConfig) string {
	return filepath.Join(cfg.ProcFS, "sys", "lnet", "stats")
}

// LNetDebugFSStatsPath returns the debugfs path to the LNet stats file.
func LNetDebugFSStatsPath(cfg PathConfig) string {
	return filepath.Join(cfg.DebugFS, "lnet", "stats")
}

// LNetStatsPaths returns LNet stats paths in preferred order.
func LNetStatsPaths(cfg PathConfig) []string {
	return []string{
		LNetDebugFSStatsPath(cfg),
		lnetStatsPath(cfg),
	}
}

// LNetParamNames lists all known LNet parameter file names.
var LNetParamNames = []string{
	"console_backoff", "console_max_delay_centisecs", "console_min_delay_centisecs",
	"console_ratelimit", "debug_mb", "panic_on_lbug", "watchdog_ratelimit",
	"catastrophe", "lnet_memused", "fail_err", "fail_val",
}

// LNetParamPath returns the path for a specific LNet parameter.
func LNetParamPath(cfg PathConfig, paramName string) string {
	return filepath.Join(cfg.ProcFS, "sys", "lnet", paramName)
}

// LNetDebugFSParamPath returns the debugfs path for a specific LNet parameter.
func LNetDebugFSParamPath(cfg PathConfig, paramName string) string {
	return filepath.Join(cfg.DebugFS, "lnet", paramName)
}

// LNetParamPaths returns LNet parameter paths in preferred order.
func LNetParamPaths(cfg PathConfig, paramName string) []string {
	if paramName == "fail_val" {
		return []string{
			LNetDebugFSParamPath(cfg, "fail_val"),
			LNetParamPath(cfg, "fail_val"),
			LNetDebugFSParamPath(cfg, "fail_max"),
			LNetParamPath(cfg, "fail_max"),
		}
	}
	return []string{
		LNetDebugFSParamPath(cfg, paramName),
		LNetParamPath(cfg, paramName),
	}
}

// ClientTarget represents a discovered llite, mdc, or osc target.
type ClientTarget struct {
	Component    string // "llite", "mdc", or "osc"
	Name         string // target name (mount name or target name)
	StatsPath    string // path to stats file
	RpcStatsPath string // path to rpc_stats file (mdc/osc only)
	BasePath     string // base directory for single-value files
}

// DiscoverClients enumerates all llite, mdc, and osc targets.
func DiscoverClients(ctx context.Context, r reader.Reader, cfg PathConfig) ([]ClientTarget, error) {
	var targets []ClientTarget
	seen := map[string]int{}

	for _, component := range []string{"llite", "mdc", "osc"} {
		pattern := filepath.Join(cfg.ProcFS, "fs", "lustre", component, "*", "stats")
		matches, err := r.Glob(ctx, pattern)
		if err != nil {
			continue
		}
		for _, statsPath := range matches {
			dir := filepath.Dir(statsPath)
			ct := ClientTarget{
				Component: component,
				Name:      filepath.Base(dir),
				StatsPath: statsPath,
				BasePath:  dir,
			}
			if component == "mdc" || component == "osc" {
				ct.RpcStatsPath = filepath.Join(dir, "rpc_stats")
			}
			key := component + "/" + ct.Name
			seen[key] = len(targets)
			targets = append(targets, ct)
		}
	}

	for _, component := range []string{"mdc", "osc"} {
		pattern := filepath.Join(cfg.ProcFS, "fs", "lustre", component, "*", "rpc_stats")
		matches, err := r.Glob(ctx, pattern)
		if err != nil {
			continue
		}
		for _, rpcStatsPath := range matches {
			dir := filepath.Dir(rpcStatsPath)
			name := filepath.Base(dir)
			key := component + "/" + name
			if idx, ok := seen[key]; ok {
				targets[idx].RpcStatsPath = rpcStatsPath
				continue
			}

			seen[key] = len(targets)
			targets = append(targets, ClientTarget{
				Component:    component,
				Name:         name,
				RpcStatsPath: rpcStatsPath,
				BasePath:     dir,
			})
		}
	}

	return targets, nil
}
