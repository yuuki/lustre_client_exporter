package discovery

import (
	"context"
	"testing"

	"github.com/yuuki/lustre_exporter/internal/reader"
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
