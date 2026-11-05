package discovery

import (
	"context"
	"path/filepath"

	"github.com/yuuki/lustre_client_exporter/internal/reader"
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
	Component    string   // "llite", "mdc", or "osc"
	Name         string   // target name (mount name or target name)
	StatsPath    string   // path to stats file
	RpcStatsPath string   // path to rpc_stats file (mdc/osc only)
	BasePath     string   // ParamRoots[0]。既存テスト互換
	ParamRoots   []string // proc, then sys. never debugfs
}

func clientParamRoots(cfg PathConfig, component, name string) []string {
	return []string{
		filepath.Join(cfg.ProcFS, "fs", "lustre", component, name),
		filepath.Join(cfg.SysFS, "fs", "lustre", component, name),
	}
}

func clientStatsPatterns(cfg PathConfig, component string) []string {
	return []string{
		filepath.Join(cfg.ProcFS, "fs", "lustre", component, "*", "stats"),
		filepath.Join(cfg.SysFS, "fs", "lustre", component, "*", "stats"),
		filepath.Join(cfg.DebugFS, "lustre", component, "*", "stats"),
	}
}

func clientRPCStatsPatterns(cfg PathConfig, component string) []string {
	return []string{
		filepath.Join(cfg.ProcFS, "fs", "lustre", component, "*", "rpc_stats"),
		filepath.Join(cfg.SysFS, "fs", "lustre", component, "*", "rpc_stats"),
		filepath.Join(cfg.DebugFS, "lustre", component, "*", "rpc_stats"),
	}
}

func newClientTarget(cfg PathConfig, component, name string) ClientTarget {
	ct := ClientTarget{
		Component:  component,
		Name:       name,
		ParamRoots: clientParamRoots(cfg, component, name),
	}
	ct.BasePath = ct.ParamRoots[0]
	return ct
}

// DiscoverClients enumerates all llite, mdc, and osc targets.
func DiscoverClients(ctx context.Context, r reader.Reader, cfg PathConfig) ([]ClientTarget, error) {
	idx := clientIndex{cfg: cfg, seen: map[string]int{}}

	for _, component := range []string{"llite", "mdc", "osc"} {
		for _, statsPath := range globAll(ctx, r, clientStatsPatterns(cfg, component)) {
			idx.addStats(component, statsPath)
		}
	}

	for _, component := range []string{"mdc", "osc"} {
		for _, rpcStatsPath := range globAll(ctx, r, clientRPCStatsPatterns(cfg, component)) {
			idx.addRPCStats(component, rpcStatsPath)
		}
	}

	return idx.targets, nil
}

// globAll returns the matches of every pattern, in pattern order. A pattern
// whose glob fails contributes no matches.
func globAll(ctx context.Context, r reader.Reader, patterns []string) []string {
	var all []string
	for _, pattern := range patterns {
		matches, err := r.Glob(ctx, pattern)
		if err != nil {
			continue
		}
		all = append(all, matches...)
	}
	return all
}

// clientIndex accumulates discovered client targets in discovery order,
// keeping one target per component and name.
type clientIndex struct {
	cfg     PathConfig
	targets []ClientTarget
	seen    map[string]int // "component/name" -> index into targets
}

// addStats records the stats file of a target. The first stats file found for
// a target wins.
func (x *clientIndex) addStats(component, statsPath string) {
	name := filepath.Base(filepath.Dir(statsPath))
	key := component + "/" + name
	if _, ok := x.seen[key]; ok {
		return
	}
	ct := newClientTarget(x.cfg, component, name)
	ct.StatsPath = statsPath
	x.add(key, ct)
}

// addRPCStats records the rpc_stats file of a target, independently of where
// its stats file was found. A target without a stats file is still added. The
// first rpc_stats file found for a target wins.
func (x *clientIndex) addRPCStats(component, rpcStatsPath string) {
	name := filepath.Base(filepath.Dir(rpcStatsPath))
	key := component + "/" + name
	if i, ok := x.seen[key]; ok {
		if x.targets[i].RpcStatsPath == "" {
			x.targets[i].RpcStatsPath = rpcStatsPath
		}
		return
	}
	ct := newClientTarget(x.cfg, component, name)
	ct.RpcStatsPath = rpcStatsPath
	x.add(key, ct)
}

func (x *clientIndex) add(key string, ct ClientTarget) {
	x.seen[key] = len(x.targets)
	x.targets = append(x.targets, ct)
}
