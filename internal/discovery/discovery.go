package discovery

import (
	"fmt"
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

// DiscoverHealth checks for the Lustre health_check file.
func DiscoverHealth(r reader.Reader, cfg PathConfig) (*Target, error) {
	path := filepath.Join(cfg.SysFS, "fs", "lustre", "health_check")
	_, err := r.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Target{Path: path, Name: "health_check"}, nil
}

// DiscoverSptlrpc checks for the sptlrpc encrypt_page_pools file.
func DiscoverSptlrpc(r reader.Reader, cfg PathConfig) (*Target, error) {
	path := filepath.Join(cfg.DebugFS, "lustre", "sptlrpc", "encrypt_page_pools")
	_, err := r.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Target{Path: path, Name: "encrypt_page_pools"}, nil
}

// LNetSource represents the source to use for LNet stats.
type LNetSource int

const (
	LNetSourceAuto    LNetSource = iota
	LNetSourceDebugFS
	LNetSourceLNetCtl
)

// LNetTargets holds discovered paths for the LNet collector.
type LNetTargets struct {
	StatsPath  string            // /proc/sys/lnet/stats
	ParamPaths map[string]string // paramName -> path
}

// DiscoverLNet checks for LNet procfs stat and param files.
func DiscoverLNet(r reader.Reader, cfg PathConfig) (*LNetTargets, error) {
	targets := &LNetTargets{
		ParamPaths: make(map[string]string),
	}

	statsPath := filepath.Join(cfg.ProcFS, "sys", "lnet", "stats")
	if _, err := r.ReadFile(statsPath); err == nil {
		targets.StatsPath = statsPath
	}

	paramNames := []string{
		"console_backoff", "console_max_delay_centisecs", "console_min_delay_centisecs",
		"console_ratelimit", "debug_mb", "panic_on_lbug", "watchdog_ratelimit",
		"catastrophe", "lnet_memused", "fail_err", "fail_max",
	}

	for _, name := range paramNames {
		path := filepath.Join(cfg.ProcFS, "sys", "lnet", name)
		if _, err := r.ReadFile(path); err == nil {
			targets.ParamPaths[name] = path
		}
	}

	if targets.StatsPath == "" && len(targets.ParamPaths) == 0 {
		return nil, fmt.Errorf("no LNet sources found")
	}

	return targets, nil
}

// ClientTarget represents a discovered llite, mdc, or osc target.
type ClientTarget struct {
	Component string // "llite", "mdc", or "osc"
	Name      string // target name (mount name or target name)
	StatsPath string // path to stats file
	RpcStatsPath string // path to rpc_stats file (mdc/osc only)
	BasePath  string // base directory for single-value files
}

// DiscoverClients enumerates all llite, mdc, and osc targets.
func DiscoverClients(r reader.Reader, cfg PathConfig) ([]ClientTarget, error) {
	var targets []ClientTarget

	// llite targets from /proc/fs/lustre/llite/*/stats
	llitePattern := filepath.Join(cfg.ProcFS, "fs", "lustre", "llite", "*", "stats")
	lliteMatches, err := r.Glob(llitePattern)
	if err == nil {
		for _, statsPath := range lliteMatches {
			dir := filepath.Dir(statsPath)
			name := filepath.Base(dir)
			targets = append(targets, ClientTarget{
				Component: "llite",
				Name:      name,
				StatsPath: statsPath,
				BasePath:  dir,
			})
		}
	}

	// mdc targets from /proc/fs/lustre/mdc/*/stats
	mdcPattern := filepath.Join(cfg.ProcFS, "fs", "lustre", "mdc", "*", "stats")
	mdcMatches, err := r.Glob(mdcPattern)
	if err == nil {
		for _, statsPath := range mdcMatches {
			dir := filepath.Dir(statsPath)
			name := filepath.Base(dir)
			rpcPath := filepath.Join(dir, "rpc_stats")
			targets = append(targets, ClientTarget{
				Component:    "mdc",
				Name:         name,
				StatsPath:    statsPath,
				RpcStatsPath: rpcPath,
				BasePath:     dir,
			})
		}
	}

	// osc targets from /proc/fs/lustre/osc/*/stats
	oscPattern := filepath.Join(cfg.ProcFS, "fs", "lustre", "osc", "*", "stats")
	oscMatches, err := r.Glob(oscPattern)
	if err == nil {
		for _, statsPath := range oscMatches {
			dir := filepath.Dir(statsPath)
			name := filepath.Base(dir)
			rpcPath := filepath.Join(dir, "rpc_stats")
			targets = append(targets, ClientTarget{
				Component:    "osc",
				Name:         name,
				StatsPath:    statsPath,
				RpcStatsPath: rpcPath,
				BasePath:     dir,
			})
		}
	}

	return targets, nil
}
