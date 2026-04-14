# Lustre Client Exporter Design

## Overview

This project will provide a Prometheus exporter dedicated to Lustre client
nodes. It will expose metrics that are compatible with the metric names,
labels, and metric types used by `GSI-HPC/lustre_exporter` where the metrics
describe the same client-side behavior.

Compatibility is intentionally limited to the Prometheus metric contract. The
implementation, parser structure, regular expressions, fixtures, dashboards,
documentation prose, and HELP strings must be original.

The MVP focuses on client-visible Lustre state:

- `llite/*` client filesystem statistics and tunables
- `mdc/*` metadata client RPC statistics
- `osc/*` object storage client RPC statistics
- `/sys/fs/lustre/health_check`
- `sptlrpc/encrypt_page_pools`
- client-side LNet statistics from debugfs and, where useful, `lnetctl`

The MVP does not collect server-side Lustre data such as OST, MDT, MDS, MGS,
quota, recovery, exports, changelog, BRW, or jobstats metrics.

## Command-Line Interface

The CLI follows common Prometheus exporter conventions while staying small for
the MVP. The exporter starts directly without subcommands:

```text
lustre_client_exporter [flags]
```

### Web Flags

| Flag | Default | Purpose |
|---|---:|---|
| `--web.listen-address` | `:9169` | Address and port for the HTTP server. |
| `--web.telemetry-path` | `/metrics` | Path that exposes Prometheus metrics. |
| `--web.config.file` | empty | Unsupported in the current implementation; setting it fails startup instead of silently serving without TLS/auth. |

### Collector Flags

Collectors are enabled by default unless `--collector.disable-defaults=true` is
set.

| Flag | Purpose |
|---|---|
| `--collector.disable-defaults` | Start with all collectors disabled. |
| `--collector.client` | Enable client filesystem and RPC metrics. |
| `--collector.lnet` | Enable LNet metrics. |
| `--collector.health` | Enable Lustre health metrics. |
| `--collector.sptlrpc` | Enable `sptlrpc` encryption page pool metrics. |
| `--no-collector.client` | Disable client metrics. |
| `--no-collector.lnet` | Disable LNet metrics. |
| `--no-collector.health` | Disable health metrics. |
| `--no-collector.sptlrpc` | Disable `sptlrpc` metrics. |

Collector tuning flags:

| Flag | Default | Purpose |
|---|---:|---|
| `--collector.lnet.source` | `auto` | LNet source selection: `auto`, `debugfs`, or `lnetctl`. |
| `--collector.scrape-timeout` | `30s` | Upper bound for a full scrape. |
| `--collector.source-timeout` | `10s` | Upper bound for one collector source. |
| `--collector.strict` | `false` | If true, source failures fail the scrape instead of returning partial data. |

The exporter intentionally does not accept GSI-HPC server collector flags such
as `--collector.ost`, `--collector.mdt`, `--collector.mgs`,
`--collector.mds`, or `--collector.generic`. Unknown flags should fail at
startup. This prevents accidental use as a server-side exporter.

### Path Flags

| Flag | Default | Purpose |
|---|---:|---|
| `--path.rootfs` | `/` | Root filesystem prefix for containerized deployments. |
| `--path.procfs` | `/proc` | procfs mount point. |
| `--path.sysfs` | `/sys` | sysfs mount point. |
| `--path.debugfs` | `/sys/kernel/debug` | debugfs mount point. |
| `--path.lnetctl` | `lnetctl` | Path or command name for `lnetctl`. |

### Logging

| Flag | Default | Purpose |
|---|---:|---|
| `--log.level` | `info` | Log level: `debug`, `info`, `warn`, or `error`. |

The exporter logs to stdout/stderr. Log file management is left to systemd,
containers, or the caller.

## Metric Contract

The metric contract follows GSI-HPC naming for shared client-side metrics.
HELP strings are original and are not part of the compatibility guarantee.

The base labels are:

- `component`
- `target`

Operation statistics add:

- `operation`

RPC histogram-like metrics add:

- `operation`
- `size`

`lustre_rpcs_in_flight` also adds:

- `type`, with values such as `mdc` or `osc`

### Client Core Metrics

The MVP should include these client-side metric families when the corresponding
Lustre files are present:

- `lustre_blocksize_bytes`
- `lustre_inodes_free`
- `lustre_inodes_maximum`
- `lustre_available_kibibytes`
- `lustre_free_kibibytes`
- `lustre_capacity_kibibytes`
- `lustre_read_samples_total`
- `lustre_read_minimum_size_bytes`
- `lustre_read_maximum_size_bytes`
- `lustre_read_bytes_total`
- `lustre_write_samples_total`
- `lustre_write_minimum_size_bytes`
- `lustre_write_maximum_size_bytes`
- `lustre_write_bytes_total`
- `lustre_stats_total`
- `lustre_ldlm_cbd_stats`
- `lustre_pages_per_rpc_total`
- `lustre_rpcs_in_flight`
- `lustre_rpcs_offset`

### Client Tunable Metrics

The MVP should include these extended client-side tunables when available:

- `lustre_checksum_pages_enabled`
- `lustre_default_ea_size_bytes`
- `lustre_lazystatfs_enabled`
- `lustre_maximum_ea_size_bytes`
- `lustre_maximum_read_ahead_megabytes`
- `lustre_maximum_read_ahead_per_file_megabytes`
- `lustre_maximum_read_ahead_whole_megabytes`
- `lustre_statahead_agl_enabled`
- `lustre_statahead_maximum`
- `lustre_xattr_cache_enabled`

### SPTLRPC Metrics

The `sptlrpc` collector should expose:

- `lustre_physical_pages`
- `lustre_pages_per_pool`
- `lustre_maximum_pages`
- `lustre_maximum_pools`
- `lustre_pages_in_pools`
- `lustre_free_pages`
- `lustre_maximum_pages_reached_total`
- `lustre_grows_total`
- `lustre_grows_failure_total`
- `lustre_shrinks_total`
- `lustre_cache_access_total`
- `lustre_cache_miss_total`
- `lustre_free_page_low`
- `lustre_maximum_waitqueue_depth`
- `lustre_out_of_memory_request_total`

In this exporter, `lustre_cache_access_total` and
`lustre_cache_miss_total` refer only to `sptlrpc/encrypt_page_pools`. They do
not refer to OST server-side cache statistics.

### Health Metrics

The health collector exposes:

- `lustre_health_check`

The value is `1` for healthy and `0` for unhealthy or unknown.

### LNet Metrics

The LNet collector should expose the GSI-HPC compatible LNet names when the
data is available:

- `lustre_send_count_total`
- `lustre_receive_count_total`
- `lustre_drop_count_total`
- `lustre_send_bytes_total`
- `lustre_receive_bytes_total`
- `lustre_drop_bytes_total`
- `lustre_allocated`
- `lustre_maximum`
- `lustre_errors_total`
- `lustre_route_count_total`
- `lustre_route_bytes_total`
- `lustre_fail_error_total`
- `lustre_fail_maximum`
- `lustre_console_backoff_enabled`
- `lustre_console_max_delay_centiseconds`
- `lustre_console_min_delay_centiseconds`
- `lustre_console_ratelimit_enabled`
- `lustre_debug_megabytes`
- `lustre_panic_on_lbug_enabled`
- `lustre_watchdog_ratelimit_enabled`
- `lustre_catastrophe_enabled`
- `lustre_lnet_memory_used_bytes`

The `auto` LNet source mode should prefer debugfs for compatibility, fall back
to legacy `/proc/sys/lnet` paths when needed, and use `lnetctl` where it gives
structured data that fills gaps safely. When `lnetctl net show` is available,
send, receive, and drop counters may include a `nid` label.

### Explicitly Excluded Metrics

The MVP must not expose:

- `lustre_available_kilobytes`
- `lustre_free_kilobytes`
- `lustre_capacity_kilobytes`
- `lustre_health_healthy`
- `lustre_lnet_mem_used`
- `target_info`
- `lustre_job_*`
- quota metrics
- recovery metrics
- exports metrics
- changelog metrics
- server-side BRW metrics
- MDS or OSS service statistics

## Internal Architecture

The implementation should keep collection, parsing, mapping, and emission
separate.

### Discovery

Discovery enumerates enabled collectors and their target files:

- `llite/*`
- `mdc/*`
- `osc/*`
- `sptlrpc/encrypt_page_pools`
- `health_check`
- LNet debugfs files
- optional `lnetctl` command availability

Missing globs or missing optional files are normal and should not fail a
scrape.

### Reader

Reader abstracts file reads and command execution. Tests should be able to
inject fixture readers without changing parser behavior.

### Parser

Parsers return normalized observations, not Prometheus metrics. A normalized
observation should include:

- metric identifier
- metric type
- labels
- numeric value
- collector name
- source path or command

### Mapper

The mapper converts normalized observations into the GSI-HPC compatible metric
contract. It is the only layer that knows the public metric names.

### Emitter

The emitter converts mapped observations into Prometheus metrics. It validates
metric type, label order, duplicate time series, and invalid numeric values.

### Collector Runtime

Collectors run independently and can be executed in parallel. Source failures
are logged and recorded in exporter self-metrics. With
`--collector.strict=false`, the exporter returns partial metrics. With
`--collector.strict=true`, source failures fail the scrape.

## Test Strategy

The MVP test suite should include:

- Parser tests for original fixtures from anonymized or hand-written Lustre
  client outputs.
- Contract tests for metric family names, label keys, and metric types.
- CLI tests for supported flags and unknown flag failure.
- Failure tests for missing files, missing debugfs, missing `lnetctl`,
  malformed numeric values, and empty client nodes.
- License guard tests to detect copied GPL headers, GSI-HPC copyright headers,
  copied dashboard JSON, and direct GPL/AGPL/LGPL dependencies.

Acceptance commands:

```sh
go test ./...
go test -race ./...
go build ./...
go-licenses check ./...
```

## References

- Prometheus writing exporters: <https://prometheus.io/docs/instrumenting/writing_exporters/>
- Prometheus node_exporter: <https://github.com/prometheus/node_exporter>
- Prometheus exporter-toolkit: <https://github.com/prometheus/exporter-toolkit>
