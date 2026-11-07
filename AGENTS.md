# AGENTS.md

Guide for coding agents (and humans) working in this repository. `CLAUDE.md`
imports this file; keep the knowledge here.

## Overview and architecture

Prometheus exporter for Lustre **client-side** metrics. It reads procfs, sysfs
and debugfs and optionally runs `lnetctl` and `lpcc` to expose health,
performance and configuration. Metric names and labels are compatible with
[GSI-HPC/lustre_exporter](https://github.com/GSI-HPC/lustre_exporter), but the
project is client-only and the implementation is original: never copy, translate
or adapt GSI-HPC (or whamcloud) code, regexes, fixtures, dashboards or prose
(`docs/license-policy.md`). Server-side metrics (OST, MDT, MDS, MGS, quota,
recovery, exports, changelog, jobstats) are out of scope.

Every scrape runs this pipeline per collector (health, sptlrpc, lnet, client,
and the opt-in lpcc):

```
Discovery -> Reader -> Parser -> Mapper -> Emitter -> Prometheus
```

- **Discovery** (`internal/discovery/`) enumerates target files and paths
  (llite/mdc/osc globs, `health_check`, LNet params) and defines `PathConfig`.
  `stats` and `rpc_stats` of an import are discovered independently in procfs,
  sysfs, then debugfs order.
- **Reader** (`internal/reader/`) abstracts file reads, globs and command
  execution (`Reader`). `FSReader` is the real one; `FakeReader` serves fixtures.
- **Parser** (`internal/parser/`) turns raw text, JSON or YAML into
  `parser.Observation`s (metric ID, labels, value). No Prometheus dependency.
- **Mapper** (`internal/mapper/`) maps observations to public metrics through
  the `Registry` of `contract.go`, the single source of truth for public names,
  types and label keys. The `MetricType` of an observation is not used: the
  Registry's type is the contract.
- **Emitter** (`internal/emitter/`) builds `prometheus.Metric`s, caching
  `prometheus.Desc`s. It returns an error, never panics, for an invalid series.
- **Collectors** (`collector/`) orchestrate the pipeline per subsystem.
  `Registry` (`collector.go`) runs them concurrently with timeouts and emits the
  self-metrics `lustre_scrape_collector_success`,
  `lustre_scrape_collector_duration_seconds` and
  `lustre_exporter_scrape_duration_seconds`.
- **CLI** (`cmd/lustre_client_exporter/`): flags (standard `flag` package),
  logger, HTTP server.

Failure handling: a collector error sets its success metric to 0 and the scrape
serves the other collectors; `-collector.strict` turns it into a failed scrape.
Which failures are skipped, logged or fatal is tabulated in `docs/metrics.md`
("Exporter scrape quality").

## Directory layout

| Path | Contents |
|---|---|
| `cmd/lustre_client_exporter/` | `main`: flags, logger, HTTP server |
| `collector/` | Collectors, the concurrent `Registry`, contract tests |
| `internal/discovery/` | Lustre paths and target discovery |
| `internal/reader/` | `Reader`, `FSReader`, `FakeReader` |
| `internal/parser/` | Parsers, their unit tests, fuzz targets (`fuzz_test.go`) and benchmarks |
| `internal/mapper/` | `contract.go` (public metric contract) and `Map` |
| `internal/emitter/` | Observation to `prometheus.Metric` |
| `testdata/` | Hand-written or anonymized Lustre fixtures, one directory per subsystem |
| `docs/` | `design.md` (design and CLI), `metrics.md` (meaning and use of every metric), `license-policy.md`, `research.md` |
| `.github/workflows/` | `ci.yml`, `security.yml`, `complexity.yml`, `release.yml` |

## Build

```sh
make build   # ./lustre_client_exporter (go build of ./cmd/lustre_client_exporter)
```

Go is pinned in `go.mod` and `mise.toml` (1.27.3); bump them together. The
target is Linux (`CGO_ENABLED=0`, amd64 and arm64 via `.goreleaser.yml`); the
code has no build constraints, so it builds anywhere. Releases are built by
`.github/workflows/release.yml` (goreleaser) from `v*` tags.

## Test

```sh
make test         # go test ./...
make test-race    # go test -race ./...
make test-fuzz    # every fuzz target for FUZZTIME (default 30s) each
make bench        # benchmarks into bench.txt (BENCH_COUNT=6); compare with benchstat
go test ./internal/parser/...                         # one package
go test -run TestParseLLiteStats ./internal/parser/   # one test
go test -run='^$' -fuzz='^FuzzParseRPCStats$' -fuzztime=30s ./internal/parser  # one fuzz target
```

- No test needs root, Lustre, network services or Docker. Collector tests feed
  fixtures through `reader.FakeReader`; never read the real `/proc` in tests.
  `internal/reader/reader_unix_test.go` runs `sh` and creates a FIFO (Unix only).
- `collector` and `internal/reader` verify goroutine leaks with goleak in
  `TestMain`: a test that leaves a goroutine behind fails the package.
- Fuzz targets (`internal/parser/fuzz_test.go`) cover every parser. Seeds come
  from `testdata/`; `checkParse` asserts the properties: no panic, an error
  comes without observations, bounded output, every observation is in the
  contract Registry with exactly its label keys, finite and non-negative values
  where the input format guarantees them, and Map plus Emit succeed for valid
  UTF-8 input. A crasher lands in `internal/parser/testdata/fuzz/<Target>/`;
  commit it with the fix as a regression seed.
- There are no integration or e2e tests.

## Static analysis

```sh
make lint          # golangci-lint run (.golangci.yml) + go mod tidy -diff
make fmt           # golangci-lint fmt (gofmt)
make lint-actions  # actionlint + zizmor on .github/ (offline unless GH_TOKEN is set)
make vuln          # govulncheck ./...
make vet           # go vet ./...
make complexity    # cccc ranking (needs cccc); CI tracks it with octocov, no gate
```

Tool versions are pinned at the top of the `Makefile` and, for
golangci-lint, in `ci.yml`; bump them together. A `//nolint` needs the linter
name and a reason; prefer fixing the code.

## Coding conventions

- Logging: `log/slog` with key-value pairs. Absent optional sources are logged
  at Debug, sources that fail to read or parse at Warn.
- Errors: wrap with `%w` and context. Parsers return `nil, err` (never partial
  observations with an error); tests rely on errors coming back unwrapped where
  they say so (`StrictReturnsUnwrappedReadError`).
- Parsers stay Prometheus-agnostic; only the mapper knows public names.
- The `component` label of client metrics is always `"client"` (llite, mdc and
  osc alike).
- Comments and docs in English. Comments say why, not what.
- Every source file is Apache-2.0 (`LICENSE`, `NOTICE`).
- Commits: short scoped subjects such as `collector: ...`, `parser: ...`,
  `build: ...`, `ci: ...`, `docs: ...`.

## Compatibility contracts

Do not change these silently; call any change out in the PR.

- **Metric names, types and label keys**: `internal/mapper/contract.go`
  (GSI-HPC compatible), guarded by `collector/contract_test.go` and documented
  in `docs/metrics.md` and `docs/design.md`. HELP strings are original and not
  part of the compatibility guarantee. Explicitly excluded names (e.g.
  `lustre_available_kilobytes`, `lustre_health_healthy`, `target_info`) must
  stay absent. `lustre_stats_total` must not gain a `type` label.
- **Self-metrics**: `lustre_scrape_collector_success`,
  `lustre_scrape_collector_duration_seconds`,
  `lustre_exporter_scrape_duration_seconds{source,result}` and their `source`
  values (`docs/metrics.md`).
- **CLI flags**: defined in `cmd/lustre_client_exporter/main.go`; the README flag
  table is authoritative. `docs/design.md` also lists planned flags
  (`--collector.disable-defaults`, `--no-collector.*`) that are not implemented.
  Unknown flags fail startup; there are deliberately no server-side collector
  flags.
- **Input formats**: Lustre procfs/sysfs/debugfs files, `lnetctl stats show`,
  `lnetctl net show -v 3` (fallback `net show`; never `peer show`) and `lpcc
  status` JSON. A format change needs a fixture under `testdata/`.

## Concurrency and resource management

- `Registry.Collect` runs each collector in its own goroutine under a context
  with the scrape timeout and, per collector, the source timeout, and waits for
  all of them. Collectors must honour their context, or the scrape blocks.
- `FSReader.ReadFile` and `Glob` run the blocking call in a goroutine and
  return when the context is done. A read blocked in the kernel (a hung Lustre
  proc file) cannot be interrupted: its goroutine stays until the read returns,
  then exits (the result channel is buffered). Each later scrape of that file
  adds one such goroutine.
- `FSReader.RunCommand` kills the command when the context is done and, through
  `cmd.WaitDelay`, stops waiting for its output pipes shortly after. A
  grandchild (behind a wrapper script) is not killed and keeps running.
- The emitter's `Desc` cache is guarded by an `RWMutex`; the duration totals of
  `Registry` by `durationMu`. There are no background goroutines outside a
  scrape.
- Nothing may panic in a scrape path: collectors run in goroutines, where a
  panic crashes the exporter. Return errors instead.

## Performance constraints

- All work happens at scrape time: one discovery plus a few reads per
  llite/mdc/osc target. Clients can have hundreds of OSC imports, so keep the
  per-target cost of the parsers low (`make bench` before and after changing a
  parser, compared with benchstat).
- Cardinality: labels are bounded by targets, operations, RPC size buckets and
  local NIDs. Never add per-process, per-job, per-peer or per-path labels;
  peer health (`lnetctl peer show`) is excluded for this reason.

## Before opening a PR

1. `make check` (lint, lint-actions, race tests, build) passes.
2. Touching a parser or a fixture: `make test-fuzz FUZZTIME=30s` and `make bench`.
3. Touching metrics, flags or failure behaviour: update `README.md`,
   `docs/metrics.md` and `docs/design.md`, and the contract tests.
4. Bumping dependencies: `make vuln`.

## Rules for AI agents

Forbidden:

- Disabling, skipping or weakening tests, fuzz properties or lint rules to get
  green; hiding failures (`|| true`, `continue-on-error`, blanket excludes).
- Editing generated files by hand (none are committed today) or anything under
  `vendor/`.
- Adding dependencies without a justification in the PR (test-only ones too).
- Changing a compatibility contract (above) without calling it out.
- Loosening workflow permissions, unpinning actions (every `uses:` is pinned to
  a commit SHA) or adding `pull_request_target`.
- Copying code or text from GSI-HPC or other projects (`docs/license-policy.md`).
- Committing secrets, rewriting history, force-pushing, tagging or publishing
  releases.

## Changes that need human review

Even when CI is green: workflows and release configuration
(`.github/`, `.goreleaser.yml`), permissions and secrets, the parsers of
untrusted input and the command execution in `internal/reader`, the metric
contract (`contract.go`) and CLI flags, dependency additions, and large
refactors.
