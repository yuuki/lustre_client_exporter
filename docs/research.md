# Research Notes

## Local Repository State

The current repository is intentionally minimal. At the time of the design
discussion, it only contained a root `LICENSE` file and no implementation.
This makes it suitable for a new, clean implementation rather than a fork or
patch of an existing exporter.

Two existing projects were inspected for design input:

- `../../whamcloud/lustrefs-exporter`
- `../../GSI-HPC/lustre_exporter`

The new exporter should not copy code from either project. The useful parts of
the investigation are architectural and compatibility-related.

## whamcloud/lustrefs-exporter

`whamcloud/lustrefs-exporter` is implemented in Rust. It uses a typed collector
library and a Prometheus exporter binary. The main collection path is centered
around:

- `lctl get_param`
- `lnetctl net show -v 4`
- `lnetctl stats show`
- typed parsing into internal records
- registration of Prometheus metrics after parsing

Its strengths are:

- a unified parsing model
- typed records before metric emission
- structured `lnetctl` parsing
- broad server-side Lustre coverage
- support for quota, recovery status, target metadata, service stats, and
  per-export statistics

Those strengths are useful as architectural input. The new client exporter
should adopt the separation between raw collection, typed parsing, normalized
records, and metric emission.

However, whamcloud's server-side scope is intentionally larger than this MVP.
The new exporter should not collect its quota, recovery, MDS, OSS, exports, or
server BRW metrics in the first version.

## GSI-HPC/lustre_exporter

`GSI-HPC/lustre_exporter` is implemented in Go. It reads Lustre data from:

- procfs
- sysfs
- debugfs
- limited `lctl` command output

Its collector model uses levels such as `core`, `extended`, and `disabled`.
It provides broad coverage across server and client metrics. The client-side
areas that matter for this project are:

- `llite/*` filesystem stats and tunables
- `mdc/*` RPC stats
- `osc/*` RPC stats
- `sptlrpc/encrypt_page_pools`
- LNet debugfs stats and tunables
- Lustre health status

Its metric names and labels are already deployed in production in the target
environment. Therefore, common client-side metric names, labels, and metric
types should be treated as the compatibility contract for this project.

The implementation style should not be copied. The new exporter should avoid
GSI-HPC's direct source-to-Prometheus channel pattern and should instead use a
clean internal observation model.

## Shared Metric Contract

The design discussion concluded that GSI-HPC compatibility should mean:

- same metric family names for shared client-side metrics
- same label keys and compatible label values
- same Prometheus metric types where applicable

It should not mean:

- same implementation
- same CLI flags
- same HELP text
- same parser structure
- same fixtures
- same dashboards
- same documentation wording

This distinction is important both for design quality and license safety.

## Selected Scope

The MVP is client-only. It includes:

- client filesystem capacity and inode metrics
- client read and write stats
- client operation stats
- client RPC statistics
- client tunables
- health status
- `sptlrpc` encryption page pool metrics
- LNet statistics

The MVP excludes:

- OST server metrics
- MDT server metrics
- MDS/MGS metrics
- quota metrics
- recovery status metrics
- exports metrics
- changelog metrics
- jobstats
- server-side BRW metrics
- MDS and OSS service statistics

Jobstats were explicitly discussed and rejected for the default client-only
MVP. They are server-side and can be expensive or privileged to collect.

## Naming Decisions

Where GSI-HPC and whamcloud use different names for the same concept, the MVP
uses the GSI-HPC name.

Examples:

| Meaning | Selected name |
|---|---|
| Available capacity | `lustre_available_kibibytes` |
| Free capacity | `lustre_free_kibibytes` |
| Total capacity | `lustre_capacity_kibibytes` |
| Health | `lustre_health_check` |
| LNet memory | `lustre_lnet_memory_used_bytes` |

The MVP does not emit whamcloud-only alternatives such as
`lustre_available_kilobytes`, `lustre_health_healthy`, or
`lustre_lnet_mem_used`.

## CLI Decisions

The CLI should follow common Prometheus exporter conventions rather than
GSI-HPC's exact flags. The selected style uses:

- `--web.*` for HTTP serving
- `--collector.*` and `--no-collector.*` for collector selection
- `--path.*` for filesystem and command path overrides
- `--log.level` for logging verbosity

The CLI should stay small for MVP and include only flags that are needed for
serving metrics, selecting collectors, overriding paths, setting timeouts, and
controlling log verbosity.

## Implementation Direction

The implementation should be a new Go exporter with these boundaries:

- discovery of Lustre client files and commands
- read abstraction for files and command output
- independent parsers for each input format
- normalized observations as internal data
- a mapper that applies the public GSI-HPC-compatible metric contract
- a Prometheus emitter
- collector-level timeout and partial-failure handling

This gives the project a clean architecture without inheriting implementation
or licensing risk from the existing exporters.

## References

- Prometheus writing exporters: <https://prometheus.io/docs/instrumenting/writing_exporters/>
- Prometheus node_exporter: <https://github.com/prometheus/node_exporter>
- Prometheus exporter-toolkit: <https://github.com/prometheus/exporter-toolkit>
- Apache License 2.0: <https://www.apache.org/licenses/LICENSE-2.0>
- GNU licenses: <https://www.gnu.org/licenses/>
