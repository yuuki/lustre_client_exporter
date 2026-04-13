package discovery

import (
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

// SptlrpcPath returns the path to the sptlrpc encrypt_page_pools file.
func SptlrpcPath(cfg PathConfig) string {
	return filepath.Join(cfg.DebugFS, "lustre", "sptlrpc", "encrypt_page_pools")
}

// LNetSource represents the source to use for LNet stats.
type LNetSource int

const (
	LNetSourceAuto    LNetSource = iota
	LNetSourceDebugFS
	LNetSourceLNetCtl
)

// LNetStatsPath returns the path to the LNet stats file.
func LNetStatsPath(cfg PathConfig) string {
	return filepath.Join(cfg.ProcFS, "sys", "lnet", "stats")
}

// LNetParamNames lists all known LNet parameter file names.
var LNetParamNames = []string{
	"console_backoff", "console_max_delay_centisecs", "console_min_delay_centisecs",
	"console_ratelimit", "debug_mb", "panic_on_lbug", "watchdog_ratelimit",
	"catastrophe", "lnet_memused", "fail_err", "fail_max",
}

// LNetParamPath returns the path for a specific LNet parameter.
func LNetParamPath(cfg PathConfig, paramName string) string {
	return filepath.Join(cfg.ProcFS, "sys", "lnet", paramName)
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
func DiscoverClients(r reader.Reader, cfg PathConfig) ([]ClientTarget, error) {
	var targets []ClientTarget

	for _, component := range []string{"llite", "mdc", "osc"} {
		pattern := filepath.Join(cfg.ProcFS, "fs", "lustre", component, "*", "stats")
		matches, err := r.Glob(pattern)
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
			targets = append(targets, ct)
		}
	}

	return targets, nil
}
