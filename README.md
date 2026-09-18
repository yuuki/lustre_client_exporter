# Lustre Client Exporter

[![AI Generated](https://img.shields.io/badge/AI%20Generated-Claude-orange?logo=anthropic)](https://claude.ai/claude-code)
[![License](https://img.shields.io/github/license/yuuki/lustre_client_exporter)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/yuuki/lustre_client_exporter)](https://github.com/yuuki/lustre_client_exporter/releases)
[![Go](https://img.shields.io/badge/Go-%3E%3D1.27-blue?logo=go)](https://go.dev)

Prometheus exporter for Lustre client-side metrics. Reads from procfs, sysfs, debugfs, and optionally lnetctl to expose Lustre health, performance, and configuration as Prometheus metrics.

This exporter emits metric names and labels compatible with [GSI-HPC/lustre_exporter](https://github.com/GSI-HPC/lustre_exporter) for operational continuity.

## Installation

### Pre-built binary (recommended)

Download the latest release from [GitHub Releases](https://github.com/yuuki/lustre_client_exporter/releases):

```sh
VERSION=0.1.0
ARCH=amd64  # or arm64
curl -fsSL "https://github.com/yuuki/lustre_client_exporter/releases/download/v${VERSION}/lustre_client_exporter_${VERSION}_linux_${ARCH}.tar.gz" \
  | tar xz lustre_client_exporter
sudo install -m 0755 lustre_client_exporter /usr/local/bin/
```

### From source

```sh
go install github.com/yuuki/lustre_client_exporter/cmd/lustre_client_exporter@latest
```

## Usage

```sh
lustre_client_exporter [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-web.listen-address` | `:9169` | Address to listen on for web interface and telemetry |
| `-web.telemetry-path` | `/metrics` | Path under which to expose metrics |
| `-web.config.file` | | Unsupported; setting this flag fails startup |
| `-collector.client` | `true` | Enable the client (llite/mdc/osc) collector |
| `-collector.lnet` | `true` | Enable the LNet collector |
| `-collector.health` | `true` | Enable the health collector |
| `-collector.sptlrpc` | `true` | Enable the sptlrpc collector |
| `-collector.lpcc` | `false` | Enable the LPCC (Lustre PCC) collector |
| `-collector.lnet.source` | `auto` | LNet data source: auto, debugfs, or lnetctl |
| `-collector.scrape-timeout` | `30s` | Maximum duration of a scrape |
| `-collector.source-timeout` | `10s` | Timeout for individual source reads |
| `-collector.strict` | `false` | Fail the scrape if any source is unavailable |
| `-path.rootfs` | `/` | Root filesystem path prefix |
| `-path.procfs` | `/proc` | procfs mount point |
| `-path.sysfs` | `/sys` | sysfs mount point |
| `-path.debugfs` | `/sys/kernel/debug` | debugfs mount point |
| `-path.lnetctl` | `lnetctl` | Path to the lnetctl binary |
| `-path.lpcc` | `lpcc` | Path to the lpcc binary |
| `-log.level` | `info` | Log level: debug, info, warn, error |

## Collectors

### Health

Reads `/sys/fs/lustre/health_check`. Emits `lustre_health_check` (1 = healthy, 0 = unhealthy).

### Client (llite / mdc / osc / ldlm_cbd)

Reads client filesystem stats, capacity, tunables, RPC statistics, and LDLM callback service stats from `/proc/fs/lustre/llite/*/`, `/proc/fs/lustre/mdc/*/`, `/proc/fs/lustre/osc/*/`, and `ldlm/services/ldlm_cbd/stats`.

For mdc and osc targets, the collector also reads `stats` (operation counts and latency sums) and `rpc_stats` (current in-flight RPCs and pending pages). OSC writeback uses `cur_dirty_bytes` and `max_dirty_mb`. Both imports expose RPC stream limits (`max_rpcs_in_flight`, and `max_mod_rpcs_in_flight` on MDC), `max_pages_per_rpc` when present, plus `active` and `state` (`current_state` only). All of these metrics use `component="client"`; latency sums, instantaneous RPC values, RPC limits, and import state also carry `type="mdc"` or `type="osc"`.

Average client-observed RPC wait time for an operation such as `req_waittime`:

```promql
rate(lustre_stats_seconds_sum{type="osc",operation="req_waittime"}[5m])
/ ignoring(type)
rate(lustre_stats_total{operation="req_waittime"}[5m])
```

Any `lustre_stats_seconds_sum` / `lustre_stats_total` ratio needs `ignoring(type)` because `lustre_stats_total` must not gain a `type` label.

This value reflects RPC wait as seen by this client, not server-side queue depth.

### SPTLRPC

Reads `sptlrpc/encrypt_page_pools` from debugfs, falling back to `/proc/fs/lustre/sptlrpc/encrypt_page_pools`, for encryption page pool metrics.

### LNet

Reads LNet send/receive/drop counters, parameter tunables, and local network
interface (NI) health. NI health metrics (`lustre_lnet_ni_up`,
`lustre_lnet_ni_health`, and `lustre_lnet_ni_health_*_total`) carry a `nid`
label and come from `lnetctl net show -v 3` when that command is available.
The exporter uses `-v 3` because that verbose level includes NI health stats;
`-v 4` is not required. When `-v 3` fails, it falls back to plain
`lnetctl net show`. The exporter does not call `lnetctl peer show`.

| Source | What is collected |
|---|---|
| `lnetctl` | `lnetctl stats show` (required) plus `lnetctl net show -v 3` (fallback `net show`). Global counters from `stats show`, per-NID send/receive/drop from `net show`, and NI health extras when verbose output is available. When `net show` returns per-NID counts, the global `send_count_total`, `receive_count_total`, and `drop_count_total` series from `stats show` are dropped to avoid duplicate series. |
| `auto` | On success reading LNet stats from debugfs or `/proc/sys/lnet/stats` (`ReadFirstAvailable` on `LNetStatsPaths`), emits those counters and non-fatally appends NI health from `net show -v 3` only. On failure, falls back to the full `lnetctl` path above. |
| `debugfs` | Stats and parameter files from debugfs and `/proc/sys/lnet/*` only. Never runs `lnetctl`. NI health metrics are not available. |

## Metrics

See [docs/metrics.md](docs/metrics.md) for the meaning and operational use of
each public metric family, including PromQL examples for client-side RCA.

## Development

```sh
make build    # Build the binary
make test     # Run tests
make vet      # Run go vet
```

```sh
go test -race ./...   # Test with race detector
```

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
