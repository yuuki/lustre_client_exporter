package discovery

import (
	"testing"

	"github.com/yuuki/lustre_exporter/internal/reader"
)

func TestDiscoverClientsDiscoversRPCStatsWithoutStatsFile(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/proc/fs/lustre/llite/*/stats"] = nil
	r.Globs["/proc/fs/lustre/mdc/*/stats"] = nil
	r.Globs["/proc/fs/lustre/osc/*/stats"] = nil
	r.Globs["/proc/fs/lustre/mdc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/mdc/lfs-dn-h-MDT0000-mdc-ff36f2b293cbd800/rpc_stats",
	}
	r.Globs["/proc/fs/lustre/osc/*/rpc_stats"] = []string{
		"/proc/fs/lustre/osc/lfs-dn-h-OST0000-osc-ff36f2b293cbd800/rpc_stats",
	}

	targets, err := DiscoverClients(r, DefaultPathConfig())
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]ClientTarget{}
	for _, target := range targets {
		found[target.Name] = target
	}

	mdc := found["lfs-dn-h-MDT0000-mdc-ff36f2b293cbd800"]
	if mdc.Component != "mdc" {
		t.Fatalf("mdc component = %q, want mdc", mdc.Component)
	}
	if mdc.RpcStatsPath != "/proc/fs/lustre/mdc/lfs-dn-h-MDT0000-mdc-ff36f2b293cbd800/rpc_stats" {
		t.Fatalf("mdc rpc path = %q", mdc.RpcStatsPath)
	}

	osc := found["lfs-dn-h-OST0000-osc-ff36f2b293cbd800"]
	if osc.Component != "osc" {
		t.Fatalf("osc component = %q, want osc", osc.Component)
	}
	if osc.RpcStatsPath != "/proc/fs/lustre/osc/lfs-dn-h-OST0000-osc-ff36f2b293cbd800/rpc_stats" {
		t.Fatalf("osc rpc path = %q", osc.RpcStatsPath)
	}
}
