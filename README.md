# Lustre Client Exporter

Prometheus exporter for Lustre client-side metrics. Reads from procfs, sysfs, debugfs, and optionally lnetctl to expose Lustre health, performance, and configuration as Prometheus metrics.

This exporter emits metric names and labels compatible with [GSI-HPC/lustre_exporter](https://github.com/GSI-HPC/lustre_exporter) for operational continuity.

## Installation

```sh
go install github.com/yuuki/lustre_exporter/cmd/lustre_client_exporter@latest
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
| `-web.config.file` | | Path to TLS/auth configuration file |
| `-collector.client` | `true` | Enable the client (llite/mdc/osc) collector |
| `-collector.lnet` | `true` | Enable the LNet collector |
| `-collector.health` | `true` | Enable the health collector |
| `-collector.sptlrpc` | `true` | Enable the sptlrpc collector |
| `-collector.lnet.source` | `auto` | LNet data source: auto, debugfs, or lnetctl |
| `-collector.scrape-timeout` | `30s` | Maximum duration of a scrape |
| `-collector.source-timeout` | `10s` | Timeout for individual source reads |
| `-collector.strict` | `false` | Fail the scrape if any source is unavailable |
| `-path.rootfs` | `/` | Root filesystem path prefix |
| `-path.procfs` | `/proc` | procfs mount point |
| `-path.sysfs` | `/sys` | sysfs mount point |
| `-path.debugfs` | `/sys/kernel/debug` | debugfs mount point |
| `-path.lnetctl` | `lnetctl` | Path to the lnetctl binary |
| `-log.level` | `info` | Log level: debug, info, warn, error |

## Collectors

### Health

Reads `/sys/fs/lustre/health_check`. Emits `lustre_health_check` (1 = healthy, 0 = unhealthy).

### Client (llite / mdc / osc)

Reads client filesystem stats, capacity, tunables, and RPC statistics from `/proc/fs/lustre/llite/*/`, `/proc/fs/lustre/mdc/*/`, and `/proc/fs/lustre/osc/*/`.

### SPTLRPC

Reads `/sys/kernel/debug/lustre/sptlrpc/encrypt_page_pools` for encryption page pool metrics.

### LNet

Reads `/proc/sys/lnet/stats` and individual parameter files, or uses `lnetctl stats show` as an alternative source.

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
