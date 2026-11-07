package parser

import (
	"os"
	"testing"
)

// Benchmarks of the parsers of the largest per-scrape inputs. A scrape parses
// stats and rpc_stats once per mdc/osc import, so on a client with hundreds of
// OSTs these dominate its CPU time. Run them with `make bench`.

func benchFixture(b *testing.B, name string) []byte {
	b.Helper()
	data, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	return data
}

func BenchmarkParseRPCStats(b *testing.B) {
	data := benchFixture(b, "osc/rpc_stats.txt")
	for b.Loop() {
		if _, err := ParseRPCStats(data, "bench", "client", "scratch-OST0000-osc-ffff0001", "osc"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseOBDStats(b *testing.B) {
	data := benchFixture(b, "osc/stats.txt")
	for b.Loop() {
		if _, err := ParseOBDStats(data, "bench", "client", "scratch-OST0000-osc-ffff0001", "osc"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLLiteStats(b *testing.B) {
	data := benchFixture(b, "llite/stats.txt")
	for b.Loop() {
		if _, err := ParseLLiteStats(data, "bench", "client", "scratch-ffff0001"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLDLMCBDStats(b *testing.B) {
	data := benchFixture(b, "ldlm/ldlm_cbd_stats.txt")
	for b.Loop() {
		if _, err := ParseLDLMCBDStats(data, "bench"); err != nil {
			b.Fatal(err)
		}
	}
}

// YAML decoding makes `lnetctl net show -v 3` the most expensive input per
// byte; the collector parses it twice (counters and NI health).
func BenchmarkParseLNetCtlNetNI(b *testing.B) {
	data := benchFixture(b, "lnet/lnetctl_net_show_verbose.yaml")
	for b.Loop() {
		if _, err := ParseLNetCtlNetNI(data, "bench"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLpccStatus(b *testing.B) {
	data := benchFixture(b, "lpcc/status.json")
	for b.Loop() {
		if _, err := ParseLpccStatus(data, "bench"); err != nil {
			b.Fatal(err)
		}
	}
}
