package discovery

import (
	"context"
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
