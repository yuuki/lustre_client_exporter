# AGENTS.md

## Rules
- Client side only: server-side metrics (OST, MDT, MDS, MGS, quota, recovery, exports, changelog, jobstats) and their collector flags are out of scope.
- Clean room: never copy, translate or adapt code, regexes, fixtures, dashboards or prose from GSI-HPC/lustre_exporter or whamcloud (`docs/license-policy.md`).
- Public metric names, types and label keys come only from `internal/mapper/contract.go`; parsers stay Prometheus-agnostic.
- The README flag table is authoritative: `--collector.disable-defaults` and `--no-collector.*` in `docs/design.md` are not implemented.

## Compatibility contracts
- Metric names, types and label keys (GSI-HPC compatible; HELP strings are not): `internal/mapper/contract.go`, guarded by `collector/contract_test.go`.
- Self-metrics and their `source` label values (`collector/collector.go`); CLI flags (`cmd/lustre_client_exporter/main.go`, README flag table).
- Accepted input formats (Lustre procfs/sysfs/debugfs files, `lnetctl stats show`, `lnetctl net show -v 3`, `lpcc status` JSON): `internal/parser/`.

## Before opening a pull request
- `make check`; also `make test-fuzz` and `make bench` (before and after) for a parser or fixture, and `make vuln` for a dependency change.

## Git conventions
- Branches: `<type>/<issue>-<summary>` (`<type>/<summary>` without an issue); tools' `agent/*`, `claude/*`, `copilot/*`, `dependabot/*` keep their names.
- Commits and PR titles: Conventional Commits 1.0.0, types `feat` `fix` `perf` `refactor` `test` `docs` `build` `ci` `chore` `revert`; scopes: `collector`, `parser`, `discovery`, `reader`, `mapper`, `emitter`.
