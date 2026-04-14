# Lustre Client Exporter

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

### SPTLRPC

Reads `sptlrpc/encrypt_page_pools` from debugfs, falling back to `/proc/fs/lustre/sptlrpc/encrypt_page_pools`, for encryption page pool metrics.

### LNet

Reads debugfs LNet stats and parameter files, falling back to `/proc/sys/lnet/*` where available. The `lnetctl` source reads `lnetctl stats show` and also uses `lnetctl net show` for per-NID send, receive, and drop counters when available.

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
