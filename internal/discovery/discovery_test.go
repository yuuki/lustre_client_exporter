package discovery

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yuuki/lustre_client_exporter/internal/reader"
)

func TestDiscoverClientsDiscoversRPCStatsWithoutStatsFile(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/llite/*/stats"] = nil
	r.Globs["/proc/fs/lustre/mdc/*/stats"] = nil
	r.Globs["/proc/fs/lustre/osc/*/stats"] = nil
	r.Globs["/proc/fs/lustre/mdc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/mdc/nonexistent-MDT9999-mdc-0000000000000000/rpc_stats",
	}
	r.Globs["/proc/fs/lustre/osc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/osc/nonexistent-OST9999-osc-0000000000000000/rpc_stats",
	}

	targets, err := DiscoverClients(context.Background(), r, DefaultPathConfig())
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]ClientTarget{}
	for _, target := range targets {
		found[target.Name] = target
	}

	mdc := found["nonexistent-MDT9999-mdc-0000000000000000"]
	if mdc.Component != "mdc" {
		t.Fatalf("mdc component = %q, want mdc", mdc.Component)
	}
	if mdc.RpcStatsPath != "/proc/fs/lustre/mdc/nonexistent-MDT9999-mdc-0000000000000000/rpc_stats" {
		t.Fatalf("mdc rpc path = %q", mdc.RpcStatsPath)
	}

	osc := found["nonexistent-OST9999-osc-0000000000000000"]
	if osc.Component != "osc" {
		t.Fatalf("osc component = %q, want osc", osc.Component)
	}
	if osc.RpcStatsPath != "/proc/fs/lustre/osc/nonexistent-OST9999-osc-0000000000000000/rpc_stats" {
		t.Fatalf("osc rpc path = %q", osc.RpcStatsPath)
	}
}

func TestDiscoverClientsPrefersProcThenSysThenDebug(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/osc/*/stats"] = []string{
		"/proc/fs/lustre/osc/fs-OST0000-osc-aaaa/stats",
	}
	r.Globs["/sys/fs/lustre/osc/*/stats"] = []string{
		"/sys/fs/lustre/osc/fs-OST0000-osc-aaaa/stats",
		"/sys/fs/lustre/osc/fs-OST0001-osc-bbbb/stats",
	}
	r.Globs["/sys/kernel/debug/lustre/mdc/*/stats"] = []string{
		"/sys/kernel/debug/lustre/mdc/fs-MDT0000-mdc-cccc/stats",
	}
	r.Globs["/proc/fs/lustre/osc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/osc/fs-OST0000-osc-aaaa/rpc_stats",
	}
	r.Globs["/sys/fs/lustre/osc/*/rpc_stats"] = []string{
		"/sys/fs/lustre/osc/fs-OST0000-osc-aaaa/rpc_stats",
		"/sys/fs/lustre/osc/fs-OST0001-osc-bbbb/rpc_stats",
	}
	r.Globs["/sys/kernel/debug/lustre/mdc/*/rpc_stats"] = []string{
		"/sys/kernel/debug/lustre/mdc/fs-MDT0000-mdc-cccc/rpc_stats",
	}

	targets, err := DiscoverClients(context.Background(), r, DefaultPathConfig())
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]ClientTarget{}
	for _, target := range targets {
		found[target.Component+"/"+target.Name] = target
	}
	osc0 := found["osc/fs-OST0000-osc-aaaa"]
	if osc0.StatsPath != "/proc/fs/lustre/osc/fs-OST0000-osc-aaaa/stats" {
		t.Fatalf("osc0 path = %q", osc0.StatsPath)
	}
	osc1 := found["osc/fs-OST0001-osc-bbbb"]
	if osc1.StatsPath != "/sys/fs/lustre/osc/fs-OST0001-osc-bbbb/stats" {
		t.Fatalf("osc1 path = %q", osc1.StatsPath)
	}
	mdc := found["mdc/fs-MDT0000-mdc-cccc"]
	if mdc.StatsPath != "/sys/kernel/debug/lustre/mdc/fs-MDT0000-mdc-cccc/stats" {
		t.Fatalf("mdc path = %q", mdc.StatsPath)
	}
	if mdc.BasePath != "/proc/fs/lustre/mdc/fs-MDT0000-mdc-cccc" {
		t.Fatalf("mdc BasePath = %q, must not be debugfs", mdc.BasePath)
	}
	if len(mdc.ParamRoots) != 2 || mdc.ParamRoots[1] != "/sys/fs/lustre/mdc/fs-MDT0000-mdc-cccc" {
		t.Fatalf("mdc ParamRoots = %v", mdc.ParamRoots)
	}
	if osc0.RpcStatsPath != "/proc/fs/lustre/osc/fs-OST0000-osc-aaaa/rpc_stats" {
		t.Fatalf("osc0 rpc = %q", osc0.RpcStatsPath)
	}
	if osc1.RpcStatsPath != "/sys/fs/lustre/osc/fs-OST0001-osc-bbbb/rpc_stats" {
		t.Fatalf("osc1 rpc = %q", osc1.RpcStatsPath)
	}
	if mdc.RpcStatsPath != "/sys/kernel/debug/lustre/mdc/fs-MDT0000-mdc-cccc/rpc_stats" {
		t.Fatalf("mdc rpc = %q", mdc.RpcStatsPath)
	}
}

func TestDiscoverClientsStatsInDebugFSParamsInSysFS(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/sys/kernel/debug/lustre/llite/*/stats"] = []string{
		"/sys/kernel/debug/lustre/llite/scratch-ffff0001/stats",
	}

	targets, err := DiscoverClients(context.Background(), r, DefaultPathConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("got %d targets", len(targets))
	}
	if targets[0].StatsPath != "/sys/kernel/debug/lustre/llite/scratch-ffff0001/stats" {
		t.Fatalf("stats = %q", targets[0].StatsPath)
	}
	if targets[0].ParamRoots[0] != "/proc/fs/lustre/llite/scratch-ffff0001" {
		t.Fatalf("param0 = %q", targets[0].ParamRoots[0])
	}
	if targets[0].ParamRoots[1] != "/sys/fs/lustre/llite/scratch-ffff0001" {
		t.Fatalf("param1 = %q", targets[0].ParamRoots[1])
	}
}

func TestDiscoverClientsDiscoversStatsAndRPCStatsIndependently(t *testing.T) {
	const (
		procRoot  = "/proc/fs/lustre"
		sysRoot   = "/sys/fs/lustre"
		debugRoot = "/sys/kernel/debug/lustre"
	)
	tests := []struct {
		name      string
		statsRoot string
		rpcRoots  []string
	}{
		{"debug_stats_proc_rpc", debugRoot, []string{procRoot}},
		{"proc_stats_sys_rpc", procRoot, []string{sysRoot, debugRoot}},
		{"sys_stats_debug_rpc", sysRoot, []string{debugRoot}},
		{"proc_siblings", procRoot, []string{procRoot}},
		{"sys_siblings", sysRoot, []string{sysRoot}},
		{"debug_siblings", debugRoot, []string{debugRoot}},
		{"proc_rpc_only", "", []string{procRoot}},
		{"sys_rpc_only", "", []string{sysRoot}},
		{"debug_rpc_only", "", []string{debugRoot}},
		{"missing_rpc", debugRoot, nil},
		{"proc_rpc_preferred_over_sibling", debugRoot, []string{procRoot, sysRoot, debugRoot}},
		{"sys_rpc_preferred_over_sibling", debugRoot, []string{sysRoot, debugRoot}},
	}
	for _, component := range []string{"mdc", "osc"} {
		for _, tt := range tests {
			t.Run(component+"/"+tt.name, func(t *testing.T) {
				r := reader.NewFakeReader()
				const name = "scratch-target-ffff0001"
				var wantStats, wantRPC string
				if tt.statsRoot != "" {
					wantStats = filepath.Join(tt.statsRoot, component, name, "stats")
					r.Globs[filepath.Join(tt.statsRoot, component, "*", "stats")] = []string{wantStats}
				}
				for _, root := range tt.rpcRoots {
					path := filepath.Join(root, component, name, "rpc_stats")
					r.Globs[filepath.Join(root, component, "*", "rpc_stats")] = []string{path}
					if wantRPC == "" {
						wantRPC = path
					}
				}

				targets, err := DiscoverClients(context.Background(), r, DefaultPathConfig())
				if err != nil {
					t.Fatal(err)
				}
				if len(targets) != 1 {
					t.Fatalf("got %d targets, want 1", len(targets))
				}
				target := targets[0]
				if target.Component != component || target.Name != name {
					t.Fatalf("unexpected target: %+v", target)
				}
				if target.StatsPath != wantStats {
					t.Errorf("stats = %q, want %q", target.StatsPath, wantStats)
				}
				if target.RpcStatsPath != wantRPC {
					t.Errorf("rpc = %q, want %q", target.RpcStatsPath, wantRPC)
				}
				if target.BasePath != filepath.Join(procRoot, component, name) {
					t.Errorf("BasePath = %q", target.BasePath)
				}
				if len(target.ParamRoots) != 2 || target.ParamRoots[0] != target.BasePath ||
					target.ParamRoots[1] != filepath.Join(sysRoot, component, name) {
					t.Errorf("ParamRoots = %v", target.ParamRoots)
				}
			})
		}
	}
}
