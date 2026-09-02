# Client RCA Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Client から見た latency、瞬間 concurrency、writeback、target 接続、LNet NI health を、現行の GSI 互換契約を壊さずに出す。

**Architecture:** Parser は Prometheus 非依存の `Observation` を返す。公開名は `internal/mapper/contract.go` だけが知る。PR1 は発見済みの mdc/osc `stats` と、現行 parser が捨てている `rpc_stats` スカラー行を読む。stats の親と tunables の親は分ける。`ParamRoots` は proc、次に sys のみ。PR2 は `ParamRoots` の単一値ファイルと `state` を読む。PR3 は `ParseLNetCtlNetStats` と `ParseLNetCtlNetNI` を分け、`auto` の supplemental は後者だけを足す。

**Tech Stack:** Go 1.26、既存の `discovery` / `reader` / `parser` / `mapper` / `emitter` / `collector`、`go test ./...`

**Spec:** `docs/superpowers/specs/2026-09-02-client-rca-metrics.md`

## Global Constraints

- client-only。server の recovery、exports、jobstats、BRW、MDS/OSS service stats は出さない。
- 既存 GSI 互換名と `lustre_stats_total` の label set（`component`, `target`, `operation`）は変えない。
- collector が渡す `component` は `"client"`。mdc/osc の区別は新しい family の `type` ラベル。
- 時間は seconds に正規化する。単位名は `usec`/`usecs`、`msec`、`nsec`、`sec`/`secs` を受け付ける。
- 生涯 min/max と sum-of-squares は v1 で出さない。
- 発見済みの `StatsPath` / `RpcStatsPath` の ReadFile 失敗は、`--collector.strict` ならエラー、そうでなければ warn。
- ParamRoots 上の optional tunables 欠落は、strict でもエラーにしない。parse 失敗だけ strict で落とす。
- LNet `source=debugfs` は lnetctl を呼ばない。`auto` の supplemental は NI extras だけ。
- 同一公開名で label set の違う系列を同じ scrape に出さない。
- GSI-HPC / whamcloud のコードは転記しない。
- コミット subject は `parser: ...` / `collector: ...` / `docs: ...`。
- ブランチ名は `cursor/` で始めない。

## File Structure

PR1（Task 1–5）:

- Create: `internal/parser/obd_stats.go`
- Create: `internal/parser/obd_stats_test.go`
- Modify: `testdata/osc/stats.txt`
- Modify: `testdata/mdc/stats.txt`
- Modify: `testdata/osc/rpc_stats.txt`
- Modify: `testdata/mdc/rpc_stats.txt`
- Modify: `internal/parser/rpc.go`
- Modify: `internal/parser/rpc_test.go`
- Modify: `internal/mapper/contract.go`
- Modify: `internal/mapper/mapper_test.go`
- Modify: `internal/discovery/discovery.go`
- Modify: `internal/discovery/discovery_test.go`
- Modify: `collector/client.go`
- Modify: `collector/client_test.go`
- Modify: `collector/contract_test.go`
- Modify: `docs/design.md`
- Modify: `README.md`

PR2（Task 6–8）:

- Create: `testdata/osc/cur_dirty_bytes`, `max_dirty_mb`, `max_pages_per_rpc`, `max_rpcs_in_flight`, `active`, `state.txt`
- Create: `testdata/mdc/max_rpcs_in_flight`, `max_mod_rpcs_in_flight`, `active`, `state.txt`
- Create: `internal/parser/target.go`
- Create: `internal/parser/target_test.go`
- Modify: `collector/client.go`, tests, `contract.go`, docs

PR3（Task 9–11）:

- Create: `testdata/lnet/lnetctl_net_show_verbose.yaml`
- Modify: `internal/parser/lnet.go`, `lnet_test.go`
- Modify: `collector/lnet.go`, `lnet_test.go`
- Modify: `collector/contract_test.go`（FakeReader に `-v 3` を載せてから expected names を足す）
- Modify: `docs/design.md`, `README.md`
- Do not modify: `cmd/lustre_client_exporter/main.go`

PR4 は仕様の P1 であり、この計画の実行対象ではない。実機 fixture を取ってから別計画を書く。

---

### Task 1: OBD stats parser

**Files:**
- Create: `internal/parser/obd_stats.go`
- Create: `internal/parser/obd_stats_test.go`
- Modify: `testdata/osc/stats.txt`
- Modify: `testdata/mdc/stats.txt`

**Interfaces:**
- Consumes: 既存 `parser.Observation`, `Counter`, `Gauge`
- Produces: `ParseOBDStats(data []byte, source, component, target, typ string) ([]Observation, error)`

- [ ] **Step 1: fixture を実データに近づける**

`testdata/osc/stats.txt` を次にする。既存の数値は残し、行を足す。

```text
snapshot_time             1681000000.123456789 secs.nsecs
req_waittime              200 samples [usec] 5 10000 500000 1234567890
req_qdepth                200 samples [reqs] 0 8 400
req_active                200 samples [reqs] 1 16 800
req_timeout               200 samples [secs] 1 64 2000
ost_read                  1000 samples [usec] 10 50000 5000000
ost_write                 800 samples [usec] 10 30000 3000000
write_bytes               800 samples [bytes] 4096 1048576 838860800
obd_ping                  50 samples [usecs] 100 2000 25000
```

`testdata/mdc/stats.txt` を次にする。

```text
snapshot_time             1681000000.123456789 secs.nsecs
req_waittime              100 samples [usec] 10 5000 250000
req_active                100 samples [reqs] 1 8 400
mds_getattr               500 samples [usec] 5 2000 100000
mds_close                 300 samples [usec] 10 3000 150000
ldlm_cancel               50 samples [usec] 5 1000 25000
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/parser/obd_stats_test.go`:

```go
package parser

import (
	"os"
	"testing"
)

func TestParseOBDStats_OSC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/stats.txt")
	if err != nil {
		t.Fatal(err)
	}

	obs, err := ParseOBDStats(data, "test", "client", "scratch-OST0000-osc-ffff0001", "osc")
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]float64{}
	seconds := map[string]float64{}
	var writeBytes float64
	for _, o := range obs {
		switch o.MetricID {
		case "stats_total":
			counts[o.Labels["operation"]] = o.Value
			if _, ok := o.Labels["type"]; ok {
				t.Fatalf("stats_total must not have type label, got %v", o.Labels)
			}
		case "stats_seconds_sum":
			seconds[o.Labels["operation"]] = o.Value
			if o.Labels["type"] != "osc" {
				t.Fatalf("stats_seconds_sum type = %q", o.Labels["type"])
			}
			if o.MetricType != Counter {
				t.Fatalf("stats_seconds_sum type = %v, want Counter", o.MetricType)
			}
		case "write_bytes_total":
			writeBytes = o.Value
		}
		if o.Labels["component"] != "client" {
			t.Fatalf("component = %q", o.Labels["component"])
		}
	}

	if counts["req_waittime"] != 200 {
		t.Errorf("req_waittime count = %v, want 200", counts["req_waittime"])
	}
	if counts["req_qdepth"] != 200 {
		t.Errorf("req_qdepth count = %v, want 200", counts["req_qdepth"])
	}
	if _, ok := seconds["req_qdepth"]; ok {
		t.Fatal("req_qdepth must not emit seconds_sum")
	}
	if seconds["req_waittime"] != 0.5 {
		t.Errorf("req_waittime seconds_sum = %v, want 0.5", seconds["req_waittime"])
	}
	if seconds["req_timeout"] != 2000 {
		t.Errorf("req_timeout seconds_sum = %v, want 2000", seconds["req_timeout"])
	}
	if seconds["obd_ping"] != 0.025 {
		t.Errorf("obd_ping seconds_sum = %v, want 0.025", seconds["obd_ping"])
	}
	if writeBytes != 838860800 {
		t.Errorf("write_bytes_total = %v, want 838860800", writeBytes)
	}
	if _, ok := counts["write_bytes"]; ok {
		t.Fatal("write_bytes must not emit stats_total")
	}
}

func TestParseOBDStats_MDC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/mdc/stats.txt")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseOBDStats(data, "test", "client", "scratch-MDT0000-mdc-ffff0001", "mdc")
	if err != nil {
		t.Fatal(err)
	}
	seconds := map[string]float64{}
	for _, o := range obs {
		if o.MetricID == "stats_seconds_sum" {
			seconds[o.Labels["operation"]] = o.Value
			if o.Labels["type"] != "mdc" {
				t.Fatalf("type = %q", o.Labels["type"])
			}
		}
	}
	if seconds["mds_getattr"] != 0.1 {
		t.Errorf("mds_getattr seconds_sum = %v, want 0.1", seconds["mds_getattr"])
	}
}

func TestParseOBDStats_Empty(t *testing.T) {
	obs, err := ParseOBDStats([]byte(""), "test", "client", "t", "osc")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Fatalf("got %d, want 0", len(obs))
	}
}

func TestParseOBDStats_MalformedCount(t *testing.T) {
	_, err := ParseOBDStats([]byte("req_waittime not_a_number samples [usec] 1 2 3\n"), "test", "client", "t", "osc")
	if err == nil {
		t.Fatal("expected error for malformed count")
	}
}
```

- [ ] **Step 3: テストが失敗することを確認する**

Run: `go test -run TestParseOBDStats ./internal/parser/`
Expected: FAIL（`ParseOBDStats` 未定義）

- [ ] **Step 4: parser を実装する**

`internal/parser/obd_stats.go`:

```go
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseOBDStats parses mdc/osc stats files.
// Every named line emits stats_total unless it is read_bytes/write_bytes.
// Time-unit lines also emit stats_seconds_sum.
// read_bytes/write_bytes emit the existing GSI size metrics.
func ParseOBDStats(data []byte, source, component, target, typ string) ([]Observation, error) {
	var observations []Observation
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "snapshot_time" {
			continue
		}
		name := fields[0]
		if name == "read_bytes" || name == "write_bytes" {
			obs, err := parseBytesStat(fields, source, component, target)
			if err != nil {
				return nil, fmt.Errorf("obd stats %s: %w", name, err)
			}
			observations = append(observations, obs...)
			continue
		}
		count, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("obd stats %s: parsing count: %w", name, err)
		}
		base := map[string]string{
			"component": component,
			"target":    target,
			"operation": name,
		}
		observations = append(observations, Observation{
			Collector:  "client",
			Source:     source,
			MetricID:   "stats_total",
			MetricType: Counter,
			Labels:     base,
			Value:      count,
		})
		unit, sum, ok := statsSum(fields)
		if !ok {
			continue
		}
		seconds, ok := unitToSeconds(unit, sum)
		if !ok {
			continue
		}
		labels := map[string]string{
			"component": component,
			"target":    target,
			"type":      typ,
			"operation": name,
		}
		observations = append(observations, Observation{
			Collector:  "client",
			Source:     source,
			MetricID:   "stats_seconds_sum",
			MetricType: Counter,
			Labels:     labels,
			Value:      seconds,
		})
	}
	return observations, nil
}

// statsSum reads optional "samples [unit] min max sum [sumsq]".
func statsSum(fields []string) (unit string, sum float64, ok bool) {
	// name count samples [unit] min max sum [sumsq]
	if len(fields) < 7 || fields[2] != "samples" {
		return "", 0, false
	}
	unit = strings.Trim(fields[3], "[]")
	sum, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return "", 0, false
	}
	return unit, sum, true
}

func unitToSeconds(unit string, sum float64) (float64, bool) {
	switch strings.ToLower(unit) {
	case "nsec", "nsecs":
		return sum / 1e9, true
	case "usec", "usecs":
		return sum / 1e6, true
	case "msec", "msecs":
		return sum / 1e3, true
	case "sec", "secs":
		return sum, true
	default:
		return 0, false
	}
}
```

既存の `parseBytesStat` を再利用する。`llite.go` から動かさない。

- [ ] **Step 5: テストが通ることを確認する**

Run: `go test -run 'TestParseOBDStats|TestParseLLiteStats' ./internal/parser/`
Expected: PASS。llite の既存テストも退行していない。

- [ ] **Step 6: Commit**

```bash
git add testdata/osc/stats.txt testdata/mdc/stats.txt \
  internal/parser/obd_stats.go internal/parser/obd_stats_test.go
git commit -m "$(cat <<'EOF'
parser: parse mdc/osc stats count and latency sums

EOF
)"
```

---

### Task 2: `stats_seconds_sum` を契約に足す

**Files:**
- Modify: `internal/mapper/contract.go`（`stats_total` の直後）

**Interfaces:**
- Consumes: Task 1 の MetricID `stats_seconds_sum`
- Produces: Registry エントリ。公開名 `lustre_stats_seconds_sum`、label keys `component`, `target`, `type`, `operation`

- [ ] **Step 1: 失敗する mapper テストを書く**

`internal/mapper/mapper_test.go` に追加する。

```go
func TestMap_StatsSecondsSum(t *testing.T) {
	obs := []parser.Observation{
		{
			MetricID:   "stats_seconds_sum",
			MetricType: parser.Counter,
			Labels: map[string]string{
				"component": "client",
				"target":    "scratch-OST0000-osc-ffff0001",
				"type":      "osc",
				"operation": "req_waittime",
			},
			Value: 0.5,
		},
	}
	mapped, err := Map(obs)
	if err != nil {
		t.Fatal(err)
	}
	if mapped[0].Def.Name != "lustre_stats_seconds_sum" {
		t.Fatalf("name = %q", mapped[0].Def.Name)
	}
	if got := strings.Join(mapped[0].Def.LabelKeys, ","); got != "component,target,type,operation" {
		t.Fatalf("labels = %q", got)
	}
}
```

`strings` import を足す。

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test -run TestMap_StatsSecondsSum ./internal/mapper/`
Expected: FAIL（`unknown metric ID: stats_seconds_sum`）

- [ ] **Step 3: Registry に足す**

`stats_total` の直後:

```go
	"stats_seconds_sum": {
		Name:      "lustre_stats_seconds_sum",
		Help:      "Sum of Lustre client stats values whose source unit is a time unit, converted to seconds.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target", "type", "operation"},
	},
```

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/mapper/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mapper/contract.go internal/mapper/mapper_test.go
git commit -m "$(cat <<'EOF'
mapper: add lustre_stats_seconds_sum contract

EOF
)"
```

---

### Task 3: ClientCollector が mdc/osc `stats` を読む

**Files:**
- Modify: `internal/discovery/discovery.go`
- Modify: `internal/discovery/discovery_test.go`
- Modify: `collector/client.go`
- Modify: `collector/client_test.go`
- Modify: `collector/contract_test.go`

**Interfaces:**
- Consumes: `ParseOBDStats`, `ClientTarget.StatsPath`, `ClientTarget.Component`, 新しい `ClientTarget.ParamRoots`
- Produces: scrape 時に `lustre_stats_total` と `lustre_stats_seconds_sum` が出る。debugfs の stats でも tunables は proc/sys を読む

`ClientTarget` を次に変える。

```go
type ClientTarget struct {
	Component    string
	Name         string
	StatsPath    string
	RpcStatsPath string
	BasePath     string   // ParamRoots[0]。既存テスト互換
	ParamRoots   []string // proc, then sys. never debugfs
}

func clientParamRoots(cfg PathConfig, component, name string) []string {
	return []string{
		filepath.Join(cfg.ProcFS, "fs", "lustre", component, name),
		filepath.Join(cfg.SysFS, "fs", "lustre", component, name),
	}
}
```

新規 target を作るとき、必ず `ParamRoots` と `BasePath = ParamRoots[0]` をセットする。
`StatsPath` が debugfs でも `BasePath` は `/proc/fs/lustre/<component>/<name>` のままにする。

- [ ] **Step 1: discovery を sysfs と debugfs にも広げるテストを書く**

`internal/discovery/discovery_test.go` に追加する。

```go
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
```

- [ ] **Step 2: discovery を実装する**

`DiscoverClients` の component loop を、次の 3 pattern を順に glob する形へ変える。
既存の `seen` map で同一 `component/name` を潰す。

```go
func clientStatsPatterns(cfg PathConfig, component string) []string {
	return []string{
		filepath.Join(cfg.ProcFS, "fs", "lustre", component, "*", "stats"),
		filepath.Join(cfg.SysFS, "fs", "lustre", component, "*", "stats"),
		filepath.Join(cfg.DebugFS, "lustre", component, "*", "stats"),
	}
}
```

stats で見つけた mdc/osc は、第二 glob が無くても sibling を必ずセットする。
`newTestClientFakeReader` は `rpc_stats` glob を持たない。sibling 代入を消すと `RpcStatsPath == ""` になり、`collectRPC` が `rpc_stats` を読まない。
`TestContract_AllExpectedMetricsPresent` が `lustre_rpcs_in_flight` などで落ちる。
`newTestClientFakeReader` に `rpc_stats` glob を足してはならない。現行の sibling 代入を残す。

```go
ct := ClientTarget{
	Component:  component,
	Name:       filepath.Base(dir),
	StatsPath:  statsPath,
	ParamRoots: clientParamRoots(cfg, component, filepath.Base(dir)),
}
ct.BasePath = ct.ParamRoots[0]
if component == "mdc" || component == "osc" {
	ct.RpcStatsPath = filepath.Join(dir, "rpc_stats")
}
```

`rpc_stats` の第二 loop も proc / sys / debug の順にする。
`TestDiscoverClientsDiscoversRPCStatsWithoutStatsFile` 用である。
既存 target に rpc_stats だけ足すときは `ParamRoots` を上書きしない。
第二 loop だけで default fixture の `RpcStatsPath` を賄おうとしてはならない。

- [ ] **Step 3: collector の失敗テストを書く**

`collector/client_test.go` の `TestClientCollector` のあとに追加する。

```go
func TestClientCollector_MDCOSCStats(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMetric(t, metrics, "lustre_stats_total", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"operation": "req_waittime",
	}, 200)
	assertMetric(t, metrics, "lustre_stats_seconds_sum", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"type":      "osc",
		"operation": "req_waittime",
	}, 0.5)
	assertMetric(t, metrics, "lustre_stats_seconds_sum", map[string]string{
		"component": "client",
		"target":    "scratch-MDT0000-mdc-ffff0001",
		"type":      "mdc",
		"operation": "mds_getattr",
	}, 0.1)
}
```

- [ ] **Step 4: テストが失敗することを確認する**

Run: `go test -run TestClientCollector_MDCOSCStats ./collector/`
Expected: FAIL（metric not found）

- [ ] **Step 5: `collectRPC` で StatsPath を読む**

`collector/client.go` の `collectRPC` を次の順にする。

1. `t.StatsPath != ""` なら `ReadFile` → `parser.ParseOBDStats(data, t.StatsPath, "client", t.Name, t.Component)`
2. 現行どおり `rpc_stats`

発見済み `StatsPath` の ReadFile 失敗は Debug ではない。strict なら return、そうでなければ warn。
`StatsPath == ""`（rpc_stats だけの target）は読まない。
parse エラーも同じ strict 規則。

```go
func (c *ClientCollector) collectRPC(ctx context.Context, t discovery.ClientTarget) ([]parser.Observation, error) {
	var allObs []parser.Observation

	if t.StatsPath != "" {
		data, err := c.reader.ReadFile(ctx, t.StatsPath)
		if err != nil {
			if c.strict {
				return nil, err
			}
			c.logger.Warn("obd stats read failed", "component", t.Component, "target", t.Name, "error", err)
		} else {
			obs, err := parser.ParseOBDStats(data, t.StatsPath, "client", t.Name, t.Component)
			if err != nil {
				if c.strict {
					return nil, err
				}
				c.logger.Warn("failed to parse obd stats", "component", t.Component, "target", t.Name, "error", err)
			} else {
				allObs = append(allObs, obs...)
			}
		}
	}

	if t.RpcStatsPath != "" {
		// existing rpc_stats block; ReadFile failure already Debug in current code.
		// Change discovered rpc_stats ReadFile failure to the same strict/warn rule.
	}
	return allObs, nil
}
```

`collectLLite` の単一値ファイル読みも `t.BasePath` だけにしない。
`readParamFile(ctx, t, name)` を足し、`t.ParamRoots` を順に試す。
両方無ければ現行どおり Debug skip。
PR1 で llite tunables が debugfs-only stats でも sysfs から読めることを、collector テスト 1 本で固定する。

```go
func TestClientCollector_LLiteParamsFromSysFSWhenStatsInDebugFS(t *testing.T) {
	r := reader.NewFakeReader()
	r.Globs["/sys/kernel/debug/lustre/llite/*/stats"] = []string{
		"/sys/kernel/debug/lustre/llite/scratch-ffff0001/stats",
	}
	loadFixture(t, r, "/sys/kernel/debug/lustre/llite/scratch-ffff0001/stats", "../testdata/llite/stats.txt")
	loadFixture(t, r, "/sys/fs/lustre/llite/scratch-ffff0001/blocksize", "../testdata/llite/blocksize")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetric(t, metrics, "lustre_blocksize_bytes", map[string]string{
		"component": "client",
		"target":    "scratch-ffff0001",
	}, 4194304)
}
```

`TestClientCollector_StrictReturnsErrorOnTargetFailure` の隣に、発見済み mdc stats の ReadFile 失敗が strict でエラーになるテストを足す。

```go
func TestClientCollector_StrictReturnsErrorOnDiscoveredStatsRead(t *testing.T) {
	r := newTestClientFakeReader(t)
	r.Errors["/proc/fs/lustre/mdc/scratch-MDT0000-mdc-ffff0001/stats"] = errors.New("permission denied")
	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
```

`errors` を `collector/client_test.go` に import する。
strict は `NewClientCollectorWithStrict` で入れる。新しい setter を発明しない。

- [ ] **Step 6: contract テストを更新する**

`collector/contract_test.go` の `expectedMetricNames` に `"lustre_stats_seconds_sum": true` を足す。

- [ ] **Step 7: テストが通ることを確認する**

Run: `go test ./internal/discovery/ ./collector/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/discovery/discovery.go internal/discovery/discovery_test.go \
  collector/client.go collector/client_test.go collector/contract_test.go
git commit -m "$(cat <<'EOF'
collector: scrape mdc and osc stats for operation latency

EOF
)"
```

---

### Task 4: `rpc_stats` の瞬間値

**Files:**
- Modify: `internal/parser/rpc.go`
- Modify: `internal/parser/rpc_test.go`
- Modify: `testdata/osc/rpc_stats.txt`
- Modify: `testdata/mdc/rpc_stats.txt`
- Modify: `internal/mapper/contract.go`
- Modify: `collector/client_test.go`
- Modify: `collector/contract_test.go`

**Interfaces:**
- Consumes: 現行 `ParseRPCStats`。histogram の section 判定は維持する
- Produces: MetricID `rpcs_current`, `pending_pages`

histogram 用の `read RPCs in flight:` 行は、これまでどおり section header でもある。
同時に colon の後ろの数値を gauge として出す。

既存テストは「通る」と書いてはならない。次は必ず直す。

| テスト | 今の前提 | 直し方 |
|---|---|---|
| `TestParseRPCStats_MDC` | `len==24`, component=`mdc` | 件数を 24+5=29 にする。histogram 件数は変えない |
| `TestParseRPCStats_OSC` | `len==26` | 26+6=32 |
| `TestParseRPCStats_SkipsScalarRpcsInFlightLines` | `len==1` かつ bucket だけ | 改名して `len==2`。bucket 1 + `rpcs_current{read}=0` |
| `TestParseRPCStats_RpcsInFlightReadWriteScalarSubsections` | `len==2` かつ全部 `rpcs_in_flight` | histogram 2 本を MetricID で濾す。加えて current read/write |
| `TestParseRPCStats_MDCRpcsInFlightModifyAndReadWriteSections` | 全部 `rpcs_in_flight` | 同上。current modify/read/write と pending 2 本を別に assert |
| `TestParseRPCStats_MDCRpcsInFlightModifyBuckets` | histogram 以外は `continue` 済み | current/pending を別に assert する。histogram expected は維持 |

- [ ] **Step 1: fixture 先頭にスカラー行を足す**

`testdata/osc/rpc_stats.txt` の `snapshot_time` の直後:

```text
read RPCs in flight:  3
write RPCs in flight: 5
dio read RPCs in flight: 1
dio write RPCs in flight: 0
pending read pages:   8
pending write pages:  120
```

`testdata/mdc/rpc_stats.txt` の直後:

```text
modify_RPCs_in_flight: 2
read RPCs in flight:  0
write RPCs in flight: 0
pending write pages:  0
pending read pages:   0
```

既存 bucket 行は変えない。

- [ ] **Step 2: 失敗する parser テストを書く**

`internal/parser/rpc_test.go` に追加する。

```go
func TestParseRPCStats_CurrentScalarsOSC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/rpc_stats.txt")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseRPCStats(data, "test", "client", "scratch-OST0000-osc-ffff0001", "osc")
	if err != nil {
		t.Fatal(err)
	}
	current := map[string]float64{}
	pending := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_current":
			current[o.Labels["operation"]] = o.Value
			if o.Labels["type"] != "osc" || o.MetricType != Gauge {
				t.Fatalf("bad rpcs_current: %+v", o)
			}
		case "pending_pages":
			pending[o.Labels["operation"]] = o.Value
		}
	}
	if current["read"] != 3 || current["write"] != 5 || current["dio_read"] != 1 || current["dio_write"] != 0 {
		t.Fatalf("current = %v", current)
	}
	if pending["read"] != 8 || pending["write"] != 120 {
		t.Fatalf("pending = %v", pending)
	}
}
```

既存の `TestParseRPCStats_OSC` の want 件数は 26+6=32 にする（histogram 26 + current 4 + pending 2）。
MDC fixture に dio が無ければ 24+5=29 にする（histogram 24 + modify/read/write current + pending 2）。
histogram 件数は変えない。

- [ ] **Step 3: テストが失敗することを確認する**

Run: `go test -run TestParseRPCStats_CurrentScalarsOSC ./internal/parser/`
Expected: FAIL

- [ ] **Step 4: スカラー行を parse する**

`ParseRPCStats` の switch で、section を切り替える前に数値を読む。

```go
func parseRPCScalar(line string) (metricID, operation string, value float64, ok bool) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return "", "", 0, false
	}
	label := strings.ToLower(strings.TrimSpace(line[:colon]))
	fields := strings.Fields(line[colon+1:])
	if len(fields) == 0 {
		return "", "", 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", "", 0, false
	}
	switch label {
	case "modify_rpcs_in_flight":
		return "rpcs_current", "modify", v, true
	case "dio read rpcs in flight":
		return "rpcs_current", "dio_read", v, true
	case "dio write rpcs in flight":
		return "rpcs_current", "dio_write", v, true
	case "read rpcs in flight":
		return "rpcs_current", "read", v, true
	case "write rpcs in flight":
		return "rpcs_current", "write", v, true
	case "pending read pages":
		return "pending_pages", "read", v, true
	case "pending write pages":
		return "pending_pages", "write", v, true
	default:
		return "", "", 0, false
	}
}
```

スカラー行は observation を足したあと、現行どおり section header としても扱う。
`pending * pages` は section を変えない（header ではない）。
`modify_RPCs_in_flight` も section を変えない。

`rpcs_current` / `pending_pages` の labels は `component`, `target`, `type`（引数 `rpcType`）, `operation`。
collector は component=`client`, rpcType=`t.Component` を渡す。

- [ ] **Step 5: 契約を足す**

```go
	"rpcs_current": {
		Name:      "lustre_rpcs_current",
		Help:      "Number of RPCs currently in flight on a Lustre client import.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target", "type", "operation"},
	},
	"pending_pages": {
		Name:      "lustre_pending_pages",
		Help:      "Number of pages waiting to be sent on a Lustre client import.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target", "type", "operation"},
	},
```

`expectedMetricNames` にも両方足す。

- [ ] **Step 6: collector テストを足す**

```go
func TestClientCollector_RPCCurrentScalars(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetric(t, metrics, "lustre_rpcs_current", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"type":      "osc",
		"operation": "write",
	}, 5)
	assertMetric(t, metrics, "lustre_pending_pages", map[string]string{
		"component": "client",
		"target":    "scratch-OST0000-osc-ffff0001",
		"type":      "osc",
		"operation": "write",
	}, 120)
}
```

既存の MDC scalar テスト（`modify_RPCs_in_flight: 0`）が gather で duplicate にならないことを確認する。
histogram の `lustre_rpcs_in_flight` と名前が違うので衝突しない。

- [ ] **Step 7: 既存テストを直したあと通ることを確認する**

`TestParseRPCStats_SkipsScalarRpcsInFlightLines` を次に置き換える。

```go
func TestParseRPCStats_ScalarRpcsInFlightIsNotABucket(t *testing.T) {
	data := []byte(`
rpcs in flight        rpcs   % cum %
read RPCs in flight:  0
0:                    7 100 100
`)
	obs, err := ParseRPCStats(data, "test", "mdc", "target", "mdc")
	if err != nil {
		t.Fatal(err)
	}
	var buckets, currents int
	for _, o := range obs {
		switch o.MetricID {
		case "rpcs_in_flight":
			buckets++
			if o.Labels["size"] != "0" || o.Value != 7 {
				t.Fatalf("bucket = %+v", o)
			}
		case "rpcs_current":
			currents++
			if o.Labels["operation"] != "read" || o.Value != 0 {
				t.Fatalf("current = %+v", o)
			}
		default:
			t.Fatalf("unexpected %s", o.MetricID)
		}
	}
	if buckets != 1 || currents != 1 {
		t.Fatalf("buckets=%d currents=%d", buckets, currents)
	}
}
```

`TestParseRPCStats_RpcsInFlightReadWriteScalarSubsections` は `len(obs) != 2` を消す。
スカラーを出したあとは 4 本以上になる。
histogram は `MetricID == "rpcs_in_flight"` だけで数える。
`MetricID != "rpcs_in_flight"` をエラーにせず `continue` する。
2 本の MDC modify テストも同じである。
histogram の expected map は維持する。
current / pending を同じテスト内で assert する。

Run: `go test ./internal/parser/ ./internal/mapper/ ./collector/`
Expected: PASS。`TestClientCollector_MDCRPCStatsWithScalarInFlightLines` の gather も通る（名前が違うので duplicate にならない）。

- [ ] **Step 8: Commit**

```bash
git add testdata/osc/rpc_stats.txt testdata/mdc/rpc_stats.txt \
  internal/parser/rpc.go internal/parser/rpc_test.go \
  internal/mapper/contract.go collector/client_test.go collector/contract_test.go
git commit -m "$(cat <<'EOF'
parser: export current RPC in-flight and pending pages

EOF
)"
```

---

### Task 5: PR1 の文書

**Files:**
- Modify: `docs/design.md`
- Modify: `README.md`

- [ ] **Step 1: design.md の Client Core Metrics に次を足す**

```text
- `lustre_stats_seconds_sum`
- `lustre_rpcs_current`
- `lustre_pending_pages`
```

Excluded の「MDS or OSS service statistics」は残す。
client の `req_waittime` は service statistics ではない、と一文添える。

- [ ] **Step 2: README の Client collector 節を更新する**

mdc / osc の `stats`（operation count と latency sum）と、`rpc_stats` の現在 in-flight / pending pages を読む、と書く。
`component` は `client`、latency と瞬間値は `type=mdc|osc`、と書く。

例:

```promql
rate(lustre_stats_seconds_sum{type="osc",operation="req_waittime"}[5m])
/
rate(lustre_stats_total{operation="req_waittime"}[5m])
```

この値が「この client から見た RPC 待ち」であり server queue ではない、と書く。

- [ ] **Step 3: 全テストと vet**

Run: `go test ./... && go vet ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add docs/design.md README.md
git commit -m "$(cat <<'EOF'
docs: describe client-observed RPC latency metrics

EOF
)"
```

ここまでが PR1。レビュー可能な単位として push / PR してよい。

---

### Task 6: OSC / MDC 単一値ファイルの parser

**Files:**
- Create: `testdata/osc/cur_dirty_bytes`（`1048576\n`）
- Create: `testdata/osc/max_dirty_mb`（`32\n`）
- Create: `testdata/osc/max_pages_per_rpc`（`256\n`）
- Create: `testdata/osc/max_rpcs_in_flight`（`8\n`）
- Create: `testdata/osc/active`（`1\n`）
- Create: `testdata/mdc/max_rpcs_in_flight`（`8\n`）
- Create: `testdata/mdc/max_mod_rpcs_in_flight`（`16\n`）
- Create: `testdata/mdc/active`（`1\n`）
- Create: `internal/parser/target.go`
- Create: `internal/parser/target_test.go`
- Modify: `internal/mapper/contract.go`

**Interfaces:**
- Produces: `ParseClientSingleFile(data []byte, source, fileName, component, target, typ string) ([]Observation, error)`

| fileName | MetricID | 変換 | labels |
|---|---|---|---|
| `cur_dirty_bytes` | `osc_dirty_bytes` | そのまま | component, target |
| `max_dirty_mb` | `osc_max_dirty_bytes` | `* 1024 * 1024` | component, target |
| `max_pages_per_rpc` | `max_pages_per_rpc` | そのまま | component, target, type |
| `max_rpcs_in_flight` | `max_rpcs_in_flight` | そのまま | component, target, type |
| `max_mod_rpcs_in_flight` | `max_mod_rpcs_in_flight` | そのまま | component, target, type |
| `active` | `target_active` | そのまま | component, target, type |

未知ファイルは `nil, nil`。
`ParseFloat` を使う。

契約の Name は仕様表どおり。`max_mod_rpcs_in_flight` の Help は `Maximum modify RPCs allowed in flight on a Lustre MDC import.`。

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestParseClientSingleFile(t *testing.T) {
	tests := []struct {
		file     string
		typ      string
		metricID string
		value    float64
		hasType  bool
	}{
		{"cur_dirty_bytes", "osc", "osc_dirty_bytes", 1048576, false},
		{"max_dirty_mb", "osc", "osc_max_dirty_bytes", 33554432, false},
		{"max_pages_per_rpc", "osc", "max_pages_per_rpc", 256, true},
		{"max_rpcs_in_flight", "osc", "max_rpcs_in_flight", 8, true},
		{"max_mod_rpcs_in_flight", "mdc", "max_mod_rpcs_in_flight", 16, true},
		{"active", "osc", "target_active", 1, true},
	}
	for _, tt := range tests {
		dir := "osc"
		if tt.typ == "mdc" {
			dir = "mdc"
		}
		data, err := os.ReadFile("../../testdata/" + dir + "/" + tt.file)
		if err != nil {
			t.Fatal(err)
		}
		obs, err := ParseClientSingleFile(data, "test", tt.file, "client", "tgt", tt.typ)
		if err != nil {
			t.Fatal(err)
		}
		if len(obs) != 1 || obs[0].MetricID != tt.metricID || obs[0].Value != tt.value {
			t.Fatalf("%s: %+v", tt.file, obs)
		}
		if _, ok := obs[0].Labels["type"]; ok != tt.hasType {
			t.Fatalf("%s type label present=%v", tt.file, ok)
		}
	}
}

func TestParseClientSingleFile_FractionalMB(t *testing.T) {
	obs, err := ParseClientSingleFile([]byte("29.15\n"), "test", "max_dirty_mb", "client", "tgt", "osc")
	if err != nil {
		t.Fatal(err)
	}
	want := 29.15 * 1024 * 1024
	if obs[0].Value != want {
		t.Fatalf("got %v want %v", obs[0].Value, want)
	}
}

func TestParseClientSingleFile_Unknown(t *testing.T) {
	obs, err := ParseClientSingleFile([]byte("1\n"), "test", "unknown", "client", "tgt", "osc")
	if err != nil || len(obs) != 0 {
		t.Fatalf("got %v %v", obs, err)
	}
}
```

- [ ] **Step 2:** `go test -run TestParseClientSingleFile ./internal/parser/` → FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4:** PASS
- [ ] **Step 5: Commit** — `parser: parse OSC dirty bytes and RPC stream limits`

---

### Task 7: `current_state` parser

**Files:**
- Create: `testdata/osc/state.txt`
- Create: `testdata/mdc/state.txt`
- Modify: `internal/parser/target.go`
- Modify: `internal/parser/target_test.go`
- Modify: `internal/mapper/contract.go`

**Interfaces:**
- Produces: `ParseClientState(data []byte, source, component, target, typ string) ([]Observation, error)`

`testdata/osc/state.txt`:

```text
current_state: FULL
state_history:
 - [ 1681000000, CONNECTING ]
 - [ 1681000001, FULL ]
```

`testdata/mdc/state.txt`:

```text
current_state: DISCONN
```

単語だけの旧形式 `FULL\n` も受け付ける。
`current_state:` が無ければ、空行と `state_history:` と `- [` で始まる行を除いた最初の token を state にする。
history 行は出さない。
state が空なら observation 無し。

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestParseClientState_CurrentStateOnly(t *testing.T) {
	data, err := os.ReadFile("../../testdata/osc/state.txt")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseClientState(data, "test", "client", "tgt", "osc")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].MetricID != "target_state" || obs[0].Value != 1 {
		t.Fatalf("%+v", obs)
	}
	if obs[0].Labels["state"] != "FULL" || obs[0].Labels["type"] != "osc" {
		t.Fatalf("labels = %v", obs[0].Labels)
	}
}

func TestParseClientState_BareToken(t *testing.T) {
	obs, err := ParseClientState([]byte("FULL\n"), "test", "client", "tgt", "osc")
	if err != nil || len(obs) != 1 || obs[0].Labels["state"] != "FULL" {
		t.Fatalf("%v %v", obs, err)
	}
}

func TestParseClientState_Empty(t *testing.T) {
	obs, err := ParseClientState([]byte("\n"), "test", "client", "tgt", "osc")
	if err != nil || len(obs) != 0 {
		t.Fatalf("%v %v", obs, err)
	}
}
```

契約:

```go
	"target_state": {
		Name:      "lustre_target_state",
		Help:      "Current Lustre client import state. The series for the observed state is 1.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target", "type", "state"},
	},
```

- [ ] **Step 2:** FAIL を確認
- [ ] **Step 3: 実装する**
- [ ] **Step 4:** PASS
- [ ] **Step 5: Commit** — `parser: parse client import current_state`

---

### Task 8: collector が tunables と state を読む

**Files:**
- Modify: `collector/client.go`
- Modify: `collector/client_test.go`
- Modify: `collector/contract_test.go`
- Modify: `docs/design.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: Task 6–7 の parser。`readParamFile`（Task 3）で `ParamRoots` を順に読む

OSC が読む: `cur_dirty_bytes`, `max_dirty_mb`, `max_pages_per_rpc`, `max_rpcs_in_flight`, `active`, `state`
MDC が読む: `max_rpcs_in_flight`, `max_mod_rpcs_in_flight`, `max_pages_per_rpc`, `active`, `state`

`max_pages_per_rpc` は両方試す。MDC に無ければ skip。
欠落は Debug skip。strict でも欠落はエラーにしない。
parse 失敗は strict なら return。

`newTestClientFakeReader` に fixture を載せる。
ディスク上の fixture 名は `testdata/osc/state.txt` である。
scrape する仮想 path は `/proc/fs/lustre/osc/<name>/state` である。
`loadFixture` でその path に `state.txt` を載せる。
`fileName == "state"` のときだけ `ParseClientState` を呼ぶ。
他の tunables は `ParseClientSingleFile` である。

OSC の仮想 path は次である。

```text
/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/cur_dirty_bytes      <- testdata/osc/cur_dirty_bytes
/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/max_dirty_mb         <- testdata/osc/max_dirty_mb
/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/max_pages_per_rpc    <- testdata/osc/max_pages_per_rpc
/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/max_rpcs_in_flight   <- testdata/osc/max_rpcs_in_flight
/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/active               <- testdata/osc/active
/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/state                <- testdata/osc/state.txt
```

MDC も同じ規則である。`state.txt` だけ拡張子が仮想 path と違う。

`expectedMetricNames` に PR2 の公開名を全部足す。
`newFullFakeReader` は `newTestClientFakeReader` 経由なので、fixture を載せれば契約テストが名前を見つけられる。

- [ ] **Step 1: 失敗する collector テストを書く**

```go
func TestClientCollector_WritebackAndState(t *testing.T) {
	r := newTestClientFakeReader(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := NewClientCollector(r, discovery.DefaultPathConfig(), logger)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	osc := "scratch-OST0000-osc-ffff0001"
	mdc := "scratch-MDT0000-mdc-ffff0001"
	assertMetric(t, metrics, "lustre_osc_dirty_bytes", map[string]string{"component": "client", "target": osc}, 1048576)
	assertMetric(t, metrics, "lustre_osc_max_dirty_bytes", map[string]string{"component": "client", "target": osc}, 33554432)
	assertMetric(t, metrics, "lustre_max_rpcs_in_flight", map[string]string{"component": "client", "target": osc, "type": "osc"}, 8)
	assertMetric(t, metrics, "lustre_max_mod_rpcs_in_flight", map[string]string{"component": "client", "target": mdc, "type": "mdc"}, 16)
	assertMetric(t, metrics, "lustre_target_active", map[string]string{"component": "client", "target": osc, "type": "osc"}, 1)
	assertMetric(t, metrics, "lustre_target_state", map[string]string{"component": "client", "target": osc, "type": "osc", "state": "FULL"}, 1)
}

func TestClientCollector_MissingOptionalTunableIsNotStrictError(t *testing.T) {
	r := newTestClientFakeReader(t)
	delete(r.Files, "/proc/fs/lustre/osc/scratch-OST0000-osc-ffff0001/cur_dirty_bytes")
	c := NewClientCollectorWithStrict(r, discovery.DefaultPathConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
	if _, err := c.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: `collectRPC` の末尾で単一値ファイルを読む**
- [ ] **Step 3: contract / README / design.md を更新する**
- [ ] **Step 4:** `go test ./... && go vet ./...`
- [ ] **Step 5: Commit** — `collector: export OSC writeback limits and import state`

ここまでが PR2。

---

### Task 9: `ParseLNetCtlNetNI` を新設する

**Files:**
- Create: `testdata/lnet/lnetctl_net_show_verbose.yaml`
- Modify: `internal/parser/lnet.go`
- Modify: `internal/parser/lnet_test.go`
- Modify: `internal/mapper/contract.go`

**Interfaces:**
- `ParseLNetCtlNetStats` は nid 付き send/recv/drop だけ。現行 `len==6` テストは変えない。
- `ParseLNetCtlNetNI(data, source) ([]Observation, error)` が status と health を返す。

`testdata/lnet/lnetctl_net_show_verbose.yaml`:

```yaml
net:
    - net type: lo
      local NI(s):
        - nid: 0@lo
          status: up
          statistics:
              send_count: 180076
              recv_count: 180072
              drop_count: 4
          health stats:
              health value: 0
              interrupts: 0
              dropped: 0
              aborted: 0
              no route: 0
              timeouts: 0
              error: 0
    - net type: o2ib
      local NI(s):
        - nid: 10.200.200.54@o2ib
          status: down
          statistics:
              send_count: 521383624
              recv_count: 694477804
              drop_count: 3749557
          health stats:
              health value: 800
              interrupts: 1
              dropped: 136
              aborted: 2
              no route: 3
              timeouts: 4
              error: 5
```

`health value: 0` を意図的に入れる。`!= 0` でブロック判定してはならない。
struct tag は次を全部付ける。

```go
HealthValue float64 `yaml:"health value"`
Interrupts  float64 `yaml:"interrupts"`
Dropped     float64 `yaml:"dropped"`
Aborted     float64 `yaml:"aborted"`
NoRoute     float64 `yaml:"no route"`
Timeouts    float64 `yaml:"timeouts"`
Error       float64 `yaml:"error"`
```

`no route` と `error` を付け忘れると、それらのカウンタが常に 0 になる。
presence はポインタまたは `yaml.Node` ではなく、専用 struct を `*healthStats` にして `nil` なら health 系を出さない。
status 文字列があれば `lnet_ni_up` を出す。`up` / `UP` が 1。

- [ ] **Step 1: 現行 YAML の件数 6 がまだ通ることを確認する**

Run: `go test -run TestParseLNetCtlNetStats_YAML ./internal/parser/`
Expected: PASS

- [ ] **Step 2: 失敗する NI テストを書く**

```go
func TestParseLNetCtlNetNI_Verbose(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_net_show_verbose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseLNetCtlNetNI(data, "test")
	if err != nil {
		t.Fatal(err)
	}
	up := map[string]float64{}
	health := map[string]float64{}
	for _, o := range obs {
		switch o.MetricID {
		case "lnet_ni_up":
			up[o.Labels["nid"]] = o.Value
		case "lnet_ni_health":
			health[o.Labels["nid"]] = o.Value
		}
	}
	if up["0@lo"] != 1 || up["10.200.200.54@o2ib"] != 0 {
		t.Fatalf("up = %v", up)
	}
	if health["0@lo"] != 0 || health["10.200.200.54@o2ib"] != 800 {
		t.Fatalf("health = %v", health)
	}
	wantCounters := map[string]float64{
		"lnet_ni_health_interrupts_total": 1,
		"lnet_ni_health_dropped_total":    136,
		"lnet_ni_health_aborted_total":    2,
		"lnet_ni_health_no_route_total":   3,
		"lnet_ni_health_timeouts_total":   4,
		"lnet_ni_health_errors_total":     5,
	}
	for id, want := range wantCounters {
		var got float64
		var found bool
		for _, o := range obs {
			if o.MetricID == id && o.Labels["nid"] == "10.200.200.54@o2ib" {
				got = o.Value
				found = true
			}
		}
		if !found || got != want {
			t.Fatalf("%s = %v found=%v want %v", id, got, found, want)
		}
	}
}

func TestParseLNetCtlNetNI_StatusWithoutHealth(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lnet/lnetctl_net_show.yaml")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseLNetCtlNetNI(data, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("got %d, want 2 up gauges", len(obs))
	}
	for _, o := range obs {
		if o.MetricID != "lnet_ni_up" {
			t.Fatalf("unexpected %s", o.MetricID)
		}
	}
}
```

契約に仕様表の LNet NI MetricID を全部足す。

- [ ] **Step 3: `ParseLNetCtlNetNI` を実装する。`ParseLNetCtlNetStats` には足さない**
- [ ] **Step 4:** `go test ./internal/parser/ ./internal/mapper/`
- [ ] **Step 5: Commit** — `parser: export LNet local NI health from verbose net show`

---

### Task 10: LNet collector が `-v 3` を使う

**Files:**
- Modify: `collector/lnet.go`
- Modify: `collector/lnet_test.go`
- Modify: `collector/contract_test.go`
- Do not modify the assertions in: `collector/failure_test.go` の `TestLNetCollector_Auto_FallsBackToLNetCtl`

**Interfaces:**
- `source=lnetctl`: `stats show`（失敗は scrape 失敗のまま）+ `net show -v 3`。失敗時 `net show`。`ParseLNetCtlNetStats` と `ParseLNetCtlNetNI` の両方
- `source=auto`: `collectFromDebugFS()` 成功なら count を出したあと `net show -v 3` を非致命で呼び、`ParseLNetCtlNetNI` だけ append する。nid 付き count は足さない。`collectFromDebugFS()` 失敗なら現行どおり `recordScrapeSource(ctx, "lctl")` して `collectFromLNetCtl`（両方の parser）。`TestLNetCollector_Auto_FallbackToLNetCtl`、`TestLNetCollector_AutoFallbackReportsLctlScrapeSource`、`failure_test.go` の fallback テストは変えない
- `source=debugfs`: `RunCommand` を呼ばない

`collectFromDebugFS()` の成功は現行どおり `ReadFirstAvailable(..., LNetStatsPaths)` の成功である。
`/proc/sys/lnet/stats` が読めたら成功である。
`/sys/kernel/debug/lnet/stats` だけを成功条件にしてはならない。
`newTestLNetFakeReader` は proc stats だけを載せる。
`TestLNetCollector_AutoSupplementalNIDoesNotMixCountLabels` と `TestContract_*` の Auto に `lnetctl stats show` を stub してはならない。
成功判定を debugfs ファイル限定にすると、それらのテストが `collectFromLNetCtl` に落ち、`stats show` 無しで失敗する。

`dropGlobalLNetCountStats` を MetricID 判定に直す。

```go
func dropGlobalLNetCountStats(obs []parser.Observation) []parser.Observation {
	filtered := make([]parser.Observation, 0, len(obs))
	for _, o := range obs {
		switch o.MetricID {
		case "send_count_total", "receive_count_total", "drop_count_total":
			continue
		}
		filtered = append(filtered, o)
	}
	return filtered
}
```

`len(o.Labels) == 0` の分岐は残さない。
`ParseLNetCtlStats` は常に `{component,target}` を付けるので、空ラベル判定では global が残る。
nid 付き count の MetricID は `send_count_by_nid_total` などであり、この switch では落ちない。

`collectFromLNetCtl` は次の順である。`stats show` の失敗は scrape 失敗のまま。

```go
netData, err := c.reader.RunCommand(ctx, c.lnetctlBin, "net", "show", "-v", "3")
if err != nil {
	c.logger.Debug("lnetctl net show -v 3 not available", "error", err)
	netData, err = c.reader.RunCommand(ctx, c.lnetctlBin, "net", "show")
	if err != nil {
		c.logger.Debug("lnetctl net show not available", "error", err)
		return obs, nil
	}
}
countObs, err := parser.ParseLNetCtlNetStats(netData, "lnetctl net show")
if err != nil {
	c.logger.Warn("failed to parse lnetctl net show", "error", err)
	return obs, nil
}
niObs, err := parser.ParseLNetCtlNetNI(netData, "lnetctl net show")
if err != nil {
	c.logger.Warn("failed to parse lnetctl net show NI", "error", err)
	niObs = nil
}
if len(countObs) > 0 {
	obs = dropGlobalLNetCountStats(obs)
	obs = append(obs, countObs...)
}
return append(obs, niObs...), nil
```

global を落とすのは nid 付き count があるときだけである。
NI extras だけ（現行 YAML の status のみ）では `send_count_total` を残す。
`-v 3` も `net show` も無いときは Debug-and-drop で `stats show` の 11 本だけを返す。
そのため `TestLNetCollector_LNetCtl` の 22 と 3 本の fallback（11）は維持される。

command 失敗は Debug log。
FakeReader key は `lnetctl net show -v 3`。

`newTestLNetFakeReader` には verbose YAML を載せない。
`TestLNetCollector_LNetCtl` は今どおり `stats show` だけを stub し、件数 22 のままにする。
`-v 3` が無いときは Debug-and-drop なので、この件数は維持される。

`TestLNetCollector_DebugFS` の件数 22 は維持する（command 無し）。
`TestLNetCollector_LNetCtlNetShowAddsNIDLabels` の key を `-v 3` にし、そのテストだけに verbose fixture を載せる。
nid count と NI extras の両方を assert する。
`lustre_send_count_total` に nid 無し系列が残っていないことを assert する。
そのテストで `reg.Gather()` する。
`dropGlobalLNetCountStats` を MetricID 判定に直さないと Gather が落ちる。
`len(Labels) == 0` のまま Gather を足してはならない。

`newFullFakeReader` に verbose YAML を `r.Commands["lnetctl net show -v 3"]` として載せる。
`TestContract_*` 4 本の LNet collector をすべて `LNetSourceAuto` に変える。
そのあとで `expectedMetricNames` に `lustre_lnet_ni_*` を足す。
名前を先に足して reader を後回しにしてはならない。

- [ ] **Step 1: debugfs-only が 22 のまま、lnetctl を呼ばないテストを残す**
- [ ] **Step 2: auto + debugfs + `-v 3` の Gather テストを書く**

```go
func TestLNetCollector_AutoSupplementalNIDoesNotMixCountLabels(t *testing.T) {
	r := newTestLNetFakeReader(t)
	verbose, err := os.ReadFile("../testdata/lnet/lnetctl_net_show_verbose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r.Commands["lnetctl net show -v 3"] = verbose
	c := NewLNetCollector(r, discovery.DefaultPathConfig(), discovery.LNetSourceAuto, "lnetctl", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertMetric(t, metrics, "lustre_lnet_ni_up", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "10.200.200.54@o2ib",
	}, 0)
	assertMetric(t, metrics, "lustre_lnet_ni_health", map[string]string{
		"component": "lnet", "target": "lnet", "nid": "0@lo",
	}, 0)
	assertMetric(t, metrics, "lustre_send_count_total", map[string]string{
		"component": "lnet", "target": "lnet",
	}, 512)
	for _, m := range metrics {
		if extractMetricName(m.Desc().String()) != "lustre_send_count_total" {
			continue
		}
		var dm dto.Metric
		if err := m.Write(&dm); err != nil {
			t.Fatal(err)
		}
		for _, label := range dm.GetLabel() {
			if label.GetName() == "nid" {
				t.Fatal("auto supplemental must not add nid-labeled send_count")
			}
		}
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewRegistry(slog.New(slog.NewTextHandler(os.Stderr, nil)), 0, 0, c))
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
	}
}

type commandSpyReader struct {
	*reader.FakeReader
	cmds []string
}

func (r *commandSpyReader) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.cmds = append(r.cmds, name+" "+strings.Join(args, " "))
	return r.FakeReader.RunCommand(ctx, name, args...)
}

func TestLNetCollector_DebugFSDoesNotRunLnetctl(t *testing.T) {
	spy := &commandSpyReader{FakeReader: newTestLNetFakeReader(t)}
	spy.Commands["lnetctl net show -v 3"] = []byte("not: [ yaml")
	c := NewLNetCollector(spy, discovery.DefaultPathConfig(), discovery.LNetSourceDebugFS, "lnetctl", slog.New(slog.NewTextHandler(os.Stderr, nil)))
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 22 {
		t.Fatalf("got %d, want 22", len(metrics))
	}
	if len(spy.cmds) != 0 {
		t.Fatalf("debugfs source must not call RunCommand, got %v", spy.cmds)
	}
	for _, m := range metrics {
		name := extractMetricName(m.Desc().String())
		if len(name) >= 15 && name[:15] == "lustre_lnet_ni_" {
			t.Fatalf("debugfs source must not emit %s", name)
		}
	}
}
```

`collector/lnet_test.go` に `strings` を import する。

- [ ] **Step 3: collector を実装する**
- [ ] **Step 4: 契約テストを Auto + command fixture に変えてから expected names を足す**
- [ ] **Step 5:** `go test ./collector/ ./internal/parser/`
- [ ] **Step 6: Commit** — `collector: scrape LNet NI health via lnetctl net show -v 3`

---

### Task 11: PR3 の文書

**Files:**
- Modify: `docs/design.md`
- Modify: `README.md`

`main.go` は触らない。
README に `source` ごとの表を書く。
`auto` は `collectFromDebugFS()`（`LNetStatsPaths` の ReadFirstAvailable。proc も成功）が成功したあと NI extras だけ足す。
debugfs 失敗時は現行どおり lnetctl へ落とす。
`debugfs` は `RunCommand` を呼ばない。
peer show は呼ばない。
`-v 3` を使う理由（health stats、`-v 4` との違い）を一文書く。

- [ ] **Step 1: 文書を更新する**
- [ ] **Step 2:** `go test ./... && go vet ./...`
- [ ] **Step 3: Commit** — `docs: describe LNet local NI health metrics`

ここまでが PR3。

---

## Self-review

1. Spec coverage: PR1–3 の契約は Task 1–11 にある。`ParamRoots`、LNet 関数分割、`max_mod_rpcs_in_flight`、契約テストの順序も仕様と一致する。
2. Placeholder scan: 「分けてもよい」は残していない。PR3 は `main.go` を触らない。
3. Type consistency: `ParseOBDStats` の `typ`、`ParseRPCStats` の `rpcType`、`ParseLNetCtlNetNI`、collector の `t.Component` は一致している。
4. stats 発見は sibling `RpcStatsPath` を残す。`collectFromDebugFS` の成功は `LNetStatsPaths` 全体。`dropGlobalLNetCountStats` は MetricID 判定。

## 実行順

Task 1 から 5 までを PR1 としてまとめる。
Task 6 から 8 が PR2、Task 9 から 11 が PR3 である。
PR を跨いで実装しない。
各 PR のあと `go test ./...` と `go vet ./...` を通す。
