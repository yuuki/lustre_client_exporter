package parser_test

// Fuzz targets for every parser of Lustre procfs/sysfs/debugfs text and of
// lnetctl and lpcc output. Seeds come from the fixtures in testdata/. Beyond
// "does not panic", every target checks the properties of checkParse, so the
// whole Parser -> Mapper -> Emitter pipeline is exercised on arbitrary input.
//
// Run one target with, for example:
//
//	go test -run='^$' -fuzz='^FuzzParseRPCStats$' -fuzztime=30s ./internal/parser
//
// or every target with `make test-fuzz`. A crasher lands in
// testdata/fuzz/<FuzzTarget>/; keep it there as a regression seed once fixed.

import (
	"bytes"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	dto "github.com/prometheus/client_model/go"
	"github.com/yuuki/lustre_client_exporter/internal/emitter"
	"github.com/yuuki/lustre_client_exporter/internal/mapper"
	"github.com/yuuki/lustre_client_exporter/internal/parser"
)

const fixtureDir = "../../testdata/"

// Fixed labels a collector would pass, so that only the input varies.
const (
	fuzzTarget = "scratch-OST0000-osc-ffff0001"
	fuzzSource = "fuzz"
)

// readFixture returns the content of a fixture under testdata/.
func readFixture(f *testing.F, name string) []byte {
	f.Helper()
	data, err := os.ReadFile(fixtureDir + name)
	if err != nil {
		f.Fatal(err)
	}
	return data
}

// addFixtures seeds f with the content of each fixture, plus the empty input.
func addFixtures(f *testing.F, names ...string) {
	f.Helper()
	f.Add([]byte{})
	for _, name := range names {
		f.Add(readFixture(f, name))
	}
}

// addNamedFixtures seeds a target that takes (content, pick) with each fixture
// under dir whose name is in names; pick selects that name in fuzzName.
func addNamedFixtures(f *testing.F, dir string, names []string) {
	f.Helper()
	for i, name := range names {
		data, err := os.ReadFile(fixtureDir + dir + name)
		if err != nil {
			continue // not every known file has a fixture
		}
		f.Add(data, uint8(i))
	}
	f.Add([]byte("1\n"), uint8(len(names))) // a file the parser does not know
}

// fuzzName picks a known file name, or one the parser does not know.
func fuzzName(names []string, pick uint8) string {
	if i := int(pick) % (len(names) + 1); i < len(names) {
		return names[i]
	}
	return "unknown_file"
}

// lines returns the number of lines a line-oriented parser can see in data.
func lines(data []byte) int {
	return bytes.Count(data, []byte("\n")) + 1
}

// structuredLimit bounds the observations of a JSON or YAML document: each
// one needs a few bytes of input (an object, a list item or an alias). A
// larger result means the parser amplifies its input.
func structuredLimit(data []byte) int {
	return 8 * (len(data) + 1)
}

// checkParse asserts the properties shared by every parser:
//   - an error comes without observations;
//   - the number of observations is at most maxObs, a bound derived from the
//     size of the input;
//   - every observation maps to a metric of the public contract
//     (mapper.Registry) and carries exactly the contract's label keys, so the
//     mapper neither drops a label nor fills one in with "";
//   - values are finite when the input spells only plain decimals (the
//     Lustre formats print integers and fixed-point numbers), and never
//     negative when the input has no minus sign;
//   - mapping and emitting never panic, succeed whenever the input is valid
//     UTF-8, and produce metrics that serialize.
func checkParse(t *testing.T, data []byte, obs []parser.Observation, err error, maxObs int) {
	t.Helper()
	if err != nil {
		if obs != nil {
			t.Fatalf("got %d observations alongside error %v", len(obs), err)
		}
		return
	}
	if len(obs) > maxObs {
		t.Fatalf("got %d observations from %d bytes, want at most %d", len(obs), len(data), maxObs)
	}

	plain := plainDecimal(data)
	unsigned := !bytes.ContainsRune(data, '-')
	for i, o := range obs {
		def, ok := mapper.Registry[o.MetricID]
		if !ok {
			t.Fatalf("[%d] metric ID %q is not in mapper.Registry", i, o.MetricID)
		}
		if !sameKeys(o.Labels, def.LabelKeys) {
			t.Fatalf("[%d] %s has labels %v, the contract has keys %v", i, def.Name, o.Labels, def.LabelKeys)
		}
		if plain && (math.IsNaN(o.Value) || math.IsInf(o.Value, 0)) {
			t.Fatalf("[%d] %s = %v from plain decimal input", i, def.Name, o.Value)
		}
		if unsigned && o.Value < 0 {
			t.Fatalf("[%d] %s = %v from input without a minus sign", i, def.Name, o.Value)
		}
	}

	mapped, err := mapper.Map(obs)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	metrics, err := emitter.Emit(mapped)
	if err != nil {
		// Only input that is not valid UTF-8 may yield an invalid label value.
		// YAML can also spell arbitrary bytes as a !!binary scalar.
		if utf8.Valid(data) && !bytes.Contains(data, []byte("binary")) {
			t.Fatalf("Emit failed for valid UTF-8 input: %v", err)
		}
		if metrics != nil {
			t.Fatalf("Emit returned %d metrics alongside error %v", len(metrics), err)
		}
		return
	}
	if len(metrics) != len(obs) {
		t.Fatalf("Emit returned %d metrics for %d observations", len(metrics), len(obs))
	}
	for i, m := range metrics {
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatalf("[%d] Write: %v", i, err)
		}
		for _, l := range dm.GetLabel() {
			if !utf8.ValidString(l.GetValue()) {
				t.Fatalf("[%d] label %s=%q is not valid UTF-8", i, l.GetName(), l.GetValue())
			}
		}
	}
}

// sameKeys reports whether labels has exactly the keys in keys.
func sameKeys(labels map[string]string, keys []string) bool {
	if len(labels) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := labels[k]; !ok {
			return false
		}
	}
	return true
}

// plainDecimal reports whether every number that data can spell is a plain
// decimal with at most 20 digits: no NaN or Inf literal, no exponent and no
// hexadecimal prefix, which strconv.ParseFloat, encoding/json and YAML would
// otherwise accept. Such numbers are below 1e21, so the parsers, which only
// divide them or scale them by at most 2^20, must produce finite values.
func plainDecimal(data []byte) bool {
	s := bytes.ToLower(data)
	if bytes.Contains(s, []byte("nan")) || bytes.Contains(s, []byte("inf")) {
		return false
	}
	digits := 0
	for i, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digits++
			if digits > 20 {
				return false
			}
			if c == '0' && i+1 < len(s) && s[i+1] == 'x' {
				return false
			}
		case c == 'e' && i > 0 && (s[i-1] == '.' || (s[i-1] >= '0' && s[i-1] <= '9')):
			return false
		case c != '.':
			digits = 0
		}
	}
	return true
}

func FuzzParseHealth(f *testing.F) {
	addFixtures(f, "health/healthy.txt", "health/unhealthy.txt", "health/empty.txt")
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseHealth(data, fuzzSource)
		if err != nil {
			t.Fatalf("ParseHealth never fails, got %v", err)
		}
		checkParse(t, data, obs, err, 1)
		if len(obs) != 1 {
			t.Fatalf("got %d observations, want 1", len(obs))
		}
		want := 0.0
		if strings.TrimSpace(string(data)) == "healthy" {
			want = 1
		}
		if obs[0].Value != want {
			t.Fatalf("health_check = %v for %q, want %v", obs[0].Value, data, want)
		}
	})
}

func FuzzParseEncryptPagePools(f *testing.F) {
	addFixtures(f, "sptlrpc/encrypt_page_pools.txt")
	f.Add([]byte("physical pages: not_a_number\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseEncryptPagePools(data, fuzzSource)
		checkParse(t, data, obs, err, lines(data))
	})
}

func FuzzParseLLiteStats(f *testing.F) {
	addFixtures(f, "llite/stats.txt")
	f.Add([]byte("read_bytes 1 samples [bytes] 4096\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseLLiteStats(data, fuzzSource, "client", fuzzTarget)
		checkParse(t, data, obs, err, 4*lines(data))
	})
}

func FuzzParseLLiteSingleFile(f *testing.F) {
	names := parser.LLiteSingleFileNames
	addNamedFixtures(f, "llite/", names)
	f.Fuzz(func(t *testing.T, data []byte, pick uint8) {
		obs, err := parser.ParseLLiteSingleFile(data, fuzzSource, fuzzName(names, pick), "client", fuzzTarget)
		checkParse(t, data, obs, err, 1)
	})
}

func FuzzParseOBDStats(f *testing.F) {
	addFixtures(f, "mdc/stats.txt", "osc/stats.txt", "ldlm/ldlm_cbd_stats.txt")
	f.Add([]byte("req_waittime 3 samples [nsec] 1 2 1e308\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseOBDStats(data, fuzzSource, "client", fuzzTarget, "osc")
		checkParse(t, data, obs, err, 4*lines(data))
	})
}

func FuzzParseRPCStats(f *testing.F) {
	addFixtures(f, "mdc/rpc_stats.txt", "osc/rpc_stats.txt")
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseRPCStats(data, fuzzSource, "client", fuzzTarget, "osc")
		checkParse(t, data, obs, err, 2*lines(data))
	})
}

func FuzzParseClientSingleFile(f *testing.F) {
	names := parser.ClientSingleFileNames
	addNamedFixtures(f, "osc/", names)
	addNamedFixtures(f, "mdc/", names)
	f.Fuzz(func(t *testing.T, data []byte, pick uint8) {
		obs, err := parser.ParseClientSingleFile(data, fuzzSource, fuzzName(names, pick), "client", fuzzTarget, "osc")
		checkParse(t, data, obs, err, 1)
	})
}

func FuzzParseClientState(f *testing.F) {
	addFixtures(f, "osc/state.txt", "mdc/state.txt")
	f.Add([]byte("FULL\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseClientState(data, fuzzSource, "client", fuzzTarget, "osc")
		checkParse(t, data, obs, err, 1)
	})
}

func FuzzParseLDLMCBDStats(f *testing.F) {
	addFixtures(f, "ldlm/ldlm_cbd_stats.txt")
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseLDLMCBDStats(data, fuzzSource)
		checkParse(t, data, obs, err, lines(data))
	})
}

func FuzzParseLNetStats(f *testing.F) {
	addFixtures(f, "lnet/stats.txt")
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseLNetStats(data, fuzzSource)
		checkParse(t, data, obs, err, 11)
	})
}

func FuzzParseLNetParam(f *testing.F) {
	names := parser.LNetParamNames
	addNamedFixtures(f, "lnet/params/", names)
	f.Fuzz(func(t *testing.T, data []byte, pick uint8) {
		obs, err := parser.ParseLNetParam(data, fuzzSource, fuzzName(names, pick))
		checkParse(t, data, obs, err, 1)
	})
}

func FuzzParseLNetCtlStats(f *testing.F) {
	addFixtures(f, "lnet/lnetctl_stats.yaml", "lnet/lnetctl_stats.json")
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseLNetCtlStats(data, fuzzSource)
		checkParse(t, data, obs, err, 11)
		if err == nil && len(obs) != 11 {
			t.Fatalf("got %d observations, want all 11 statistics", len(obs))
		}
	})
}

// The collector parses one `lnetctl net show` output with both parsers, so
// one target covers both.
func FuzzParseLNetCtlNetShow(f *testing.F) {
	addFixtures(f, "lnet/lnetctl_net_show.yaml", "lnet/lnetctl_net_show_verbose.yaml")
	f.Add([]byte(`{"net":[{"local NI(s)":[{"nid":"0@lo","status":"up","statistics":{"send_count":1}}]}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseLNetCtlNetStats(data, fuzzSource)
		checkParse(t, data, obs, err, structuredLimit(data))

		obs, err = parser.ParseLNetCtlNetNI(data, fuzzSource)
		checkParse(t, data, obs, err, structuredLimit(data))
	})
}

func FuzzParseLpccStatus(f *testing.F) {
	addFixtures(f, "lpcc/status.json")
	f.Add([]byte(`{"/mnt/lustre":{"pcc":[{"cache":"/mnt/pcc","status":"running"}]}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		obs, err := parser.ParseLpccStatus(data, fuzzSource)
		checkParse(t, data, obs, err, structuredLimit(data))
	})
}

// Every name the single-value fuzz targets pick from must be a file the
// collectors read; an empty list would silently fuzz only "unknown_file".
func TestFuzzNameListsAreNotEmpty(t *testing.T) {
	for name, names := range map[string][]string{
		"llite":  parser.LLiteSingleFileNames,
		"client": parser.ClientSingleFileNames,
		"lnet":   parser.LNetParamNames,
	} {
		if len(names) == 0 || !slices.IsSorted(names) {
			t.Errorf("%s single-file names = %v, want a sorted, non-empty list", name, names)
		}
	}
}
