# AGENTS.md

This file holds only what the code does not show: the rules, contracts and
policies for changing lustre_client_exporter. Design decisions are in
[docs/design.md](docs/design.md).

## Rules the code does not show

- Client side only: server-side metrics (OST, MDT, MDS, MGS, quota, recovery,
  exports, changelog, jobstats) and their collector flags are out of scope.
- Clean room: never copy, translate or adapt code, regexes, fixtures,
  dashboards or prose from GSI-HPC/lustre_exporter or whamcloud
  ([docs/license-policy.md](docs/license-policy.md)).
- Public names, types and label keys come only from
  `internal/mapper/contract.go`; parsers stay Prometheus-agnostic.
- Never panic in a scrape path (collectors run in goroutines: a panic kills
  the exporter) and always honour the context (or the whole scrape blocks).
- A new input format needs a fixture under `testdata/`, hand-written or
  captured with hostnames, NIDs and filesystem names anonymized; tests never
  read the real `/proc`.
- Cardinality: no per-process, per-job, per-peer or per-path labels, hence
  never `lnetctl peer show`.
- Flags: the README table is authoritative; `--collector.disable-defaults`
  and `--no-collector.*` in docs/design.md are not implemented.
- English for code, comments, docs and commit messages.

## Compatibility contracts

Call any change out in the pull request.

- Metric names, types and label keys (GSI-HPC compatible; HELP strings are
  not): `internal/mapper/contract.go`, guarded by
  `collector/contract_test.go`. `lustre_stats_total` never gains a `type`
  label.
- Self-metrics and their `source` label values: `collector/collector.go`.
- CLI flags: `cmd/lustre_client_exporter/main.go` and the README flag table.
- Accepted input formats (Lustre procfs/sysfs/debugfs files,
  `lnetctl stats show`, `lnetctl net show -v 3`, `lpcc status` JSON):
  `internal/parser/`.

## Before opening a pull request

- `make check`.
- Parser or fixture changed: `make test-fuzz FUZZTIME=30s` and `make bench`
  (`benchstat` before/after); commit fuzz crashers as regression seeds.
- Metrics, flags or failure behaviour changed: update `README.md`,
  `docs/metrics.md`, `docs/design.md` and the contract tests.
- Dependencies changed: `make vuln`.

## Git conventions

Branches: `<type>/<issue>-<summary>`, or `<type>/<summary>` without an issue
(`fix/166-connect-errors`, `feat/qp-counters`). `<type>` is a commit type
below; `<summary>` is 2–5 lowercase ASCII words joined by `-`. Branches named
by tools keep their names (`agent/*`, `claude/*`, `copilot/*`,
`dependabot/*`). Never push to the default branch.

Commits follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

    <type>(<scope>)!: <subject>

    <body>

    <footer>

- `type`: `feat` (new user-visible behaviour), `fix` (bug fix), `perf`,
  `refactor` (no behaviour change), `test`, `docs`, `build` (build system,
  dependencies, packaging), `ci` (workflows), `chore` (anything else, e.g.
  `chore(release): prepare v1.2.3`), `revert`.
- `scope`: optional; the package or area changed: `collector`, `parser`,
  `discovery`, `reader`, `mapper`, `emitter`.
- `!` and a `BREAKING CHANGE: <what changed and how to migrate>` footer for
  any change to a compatibility contract above.
- `subject`: imperative, lower case (except identifiers), no final period,
  at most 72 characters including the prefix.
- `body`: why the change is needed and what it affects, wrapped at 72
  columns; required unless the subject says everything.
- `footer`: `Fixes #N` / `Refs #N`, `BREAKING CHANGE:`, trailers such as
  `Co-Authored-By:`.
- One logical change per commit; regenerated files go in the commit that
  changes their source. Pull request titles use the same format (they become
  the squash commit).

## Forbidden

- Disabling, skipping or weakening tests, fuzz properties or lint rules, or
  hiding failures (`|| true`, `continue-on-error`, blanket excludes), to get
  green. A `//nolint` names the linter and the reason.
- Adding dependencies, test-only ones included, without a justification in
  the pull request.
- Changing a compatibility contract without calling it out.
- Loosening workflow permissions, unpinning actions or adding
  `pull_request_target`.
- Committing secrets, rewriting history, force-pushing, tagging or publishing
  releases.

## Needs human review

Even with green CI: `.github/` and `.goreleaser.yml`; permissions and
secrets; the parsers of untrusted input and the command execution in
`internal/reader`; the metric contract and CLI flags; dependency additions;
large refactors.

## Environment notes

- With `GH_TOKEN` set but the GitHub API unreachable, run
  `env -u GH_TOKEN make check`.
