# Lustre Client Metrics

This guide explains the Prometheus metrics emitted by
`lustre_client_exporter` and how to use them in operations. It describes
the public metric contract as implemented on `main`, not a server-side
Lustre exporter and not a theoretical catalog of every Lustre proc file.

The exporter reads client-visible state from procfs, sysfs, debugfs, and
optionally `lnetctl` and `lpcc`. Public names and labels follow
[GSI-HPC/lustre_exporter](https://github.com/GSI-HPC/lustre_exporter) where
the same client-side quantity already exists. The implementation itself is
original. The single source of truth for names, types, help text, and label
keys is `internal/mapper/contract.go`.

Server-side Lustre data is intentionally out of scope: OST, MDT, MDS, MGS,
quota, recovery, exports, changelog, jobstats, and server BRW or service
statistics are not collected. That is a design choice, not a missing
feature. The value of this exporter is that a client node can still answer
"is this node the straggler, and where on the client path is it waiting?"
without access to MDS or OSS metrics.

## Collectors and what is not exported

Four collectors are enabled by default: `client`, `lnet`, `health`, and
`sptlrpc`. The LPCC collector is off unless you pass `-collector.lpcc`.

The HTTP handler uses a dedicated `prometheus.Registry`. The usual Go
runtime and process families (`go_*`, `process_*`) are therefore absent.
Scrape quality is covered by the exporter's own `lustre_scrape_*` and
`lustre_exporter_scrape_duration_seconds` metrics, described at the end of
this document.

Most client filesystem and RPC series carry `component="client"` plus a
`target` label. For llite, `target` is the client mount name. For MDC and
OSC, it is the import name. Families that must distinguish metadata from
object I/O also carry `type="mdc"` or `type="osc"`. LNet series use
`component="lnet"` and `target="lnet"`, and add `nid` when the source is
per-interface.

Not every metric deserves the same attention. Configuration gauges are
useful for drift detection. Counters of filesystem operations are useful
for workload characterization. A small set of health, import, RPC, and LNet
series is what you actually page on. The last section lists that set.

## Cluster health

`lustre_health_check` is a gauge from `/sys/fs/lustre/health_check`. The
parser treats the literal string `healthy` as `1` and every other value,
including empty or `NOT HEALTHY`, as `0`. Labels are `component="health"`
and `target="lustre"`.

This is Lustre's own health state, not a performance SLO. A node that is
still serving I/O can report unhealthy, and a node with terrible latency
can still report healthy. Use it as the first symptom detector: if the
gauge drops to `0`, start a deeper look at import state, LNet NI health,
and RPC pressure on that instance.

## Filesystem capacity and inodes

The client collector reads the single-value llite files under each mount
(`kbytesavail`, `kbytesfree`, `kbytestotal`, `filesfree`, `filestotal`,
`blocksize`) and publishes them as gauges with `component="client"` and
`target` set to the mount.

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_blocksize_bytes` | Filesystem block size | Configuration check. Rarely useful as an alert. |
| `lustre_available_kibibytes` | Usable capacity after reserved space | Remaining space that users can actually write. |
| `lustre_free_kibibytes` | Free capacity | Physical free space, including reserved area. |
| `lustre_capacity_kibibytes` | Total capacity | Denominator for capacity ratio. |
| `lustre_inodes_free` | Free inodes | Inode exhaustion. |
| `lustre_inodes_maximum` | Inode limit | Denominator for inode ratio. |

Capacity and inode utilization are therefore:

```promql
1 - lustre_available_kibibytes / lustre_capacity_kibibytes
```

```promql
1 - lustre_inodes_free / lustre_inodes_maximum
```

`available` and `free` are not interchangeable. Prefer `available` when
asking whether users can still create files or write data. These series
describe the filesystem as the client sees it through statfs; they do not
replace server-side OSD space metrics if you have them.

## Client I/O

Read and write traffic come from `read_bytes` and `write_bytes` lines in
`llite/*/stats`. The same four-metric mapping is applied when those lines
appear in MDC or OSC `stats`. Each line's count, min, max, and sum become
separate series. The min and max values are lifetime extrema since the
stats file was last reset, not extrema over the scrape interval.

| Metric | Type | Meaning | Typical use |
|---|---|---|---|
| `lustre_read_samples_total` | Counter | Completed read operations | Read IOPS-like activity. |
| `lustre_read_bytes_total` | Counter | Bytes read | Read throughput. |
| `lustre_read_minimum_size_bytes` | Gauge | Smallest observed read | Small-I/O detection. |
| `lustre_read_maximum_size_bytes` | Gauge | Largest observed read | I/O size characterization. |
| `lustre_write_samples_total` | Counter | Completed write operations | Write IOPS-like activity. |
| `lustre_write_bytes_total` | Counter | Bytes written | Write throughput. |
| `lustre_write_minimum_size_bytes` | Gauge | Smallest observed write | Small-write workloads. |
| `lustre_write_maximum_size_bytes` | Gauge | Largest observed write | I/O size characterization. |

Throughput is the most useful derived signal:

```promql
rate(lustre_read_bytes_total[5m])
```

```promql
rate(lustre_write_bytes_total[5m])
```

Read operation rate is:

```promql
rate(lustre_read_samples_total[5m])
```

These counts are Lustre client filesystem operations, not block-layer I/O.
They will not necessarily match `node_exporter` disk stats on the same
host. A cached write can increment llite write counters before an OSC RPC
leaves the node; conversely, writeback can generate OSC traffic after the
application has already returned.

## Operation counts and client-observed latency

### `lustre_stats_total`

`lustre_stats_total` is a counter labeled by `component`, `target`, and
`operation`. For llite, every `stats` line except `read_bytes`,
`write_bytes`, and `snapshot_time` is mapped here. For MDC and OSC, every
named line except `read_bytes` and `write_bytes` is mapped the same way.
Operation names are taken from the kernel file as-is, so the set grows and
shrinks with the Lustre version without an exporter change.

That makes this family the right place to find metadata-heavy work that
bandwidth metrics hide: `open`, `close`, `getattr`, `setattr`, `statfs`,
`inode_permission`, `mds_getattr`, `mds_close`, `ldlm_cancel`, and similar
names. A useful overview is:

```promql
sum by (operation) (
  rate(lustre_stats_total[5m])
)
```

To compare metadata versus data-plane activity on one client, break the
same rate down by `target` or `instance` rather than summing the whole
cluster first.

`lustre_stats_total` does **not** carry a `type` label. Adding one would
break the existing llite series, which share the same metric family. When
you need MDC versus OSC, use the import `target` name or join against a
family that does have `type`.

### `lustre_stats_seconds_sum`

MDC and OSC `stats` lines whose unit is a time unit also emit
`lustre_stats_seconds_sum`. The exporter converts nsec, usec, msec, and
sec (including the plural forms) to seconds and stores the cumulative sum.
Labels are `component`, `target`, `type`, and `operation`.

The ratio of the two families is the mean client-observed duration for
that operation. The README example for OSC RPC wait is the one operators
should start with:

```promql
rate(lustre_stats_seconds_sum{type="osc",operation="req_waittime"}[5m])
/ ignoring(type)
rate(lustre_stats_total{operation="req_waittime"}[5m])
```

`ignoring(type)` is required because only the numerator has `type`. The
same pattern works for `mds_getattr`, `ost_read`, `ost_write`,
`ldlm_cancel`, and other timed operations.

This average is what the client recorded from send to reply. It is not
server-side queue depth, and it is not a percentile. Network RTT, server
processing, server queues, and LNet router delay are all folded together.
A rising `req_waittime` mean tells you that *this client* is waiting
longer on that import. It does not, by itself, prove that an OSS thread
pool is saturated.

## Client tunables

llite runtime settings are exported as gauges on the same
`component`/`target` labels as the mount they belong to.

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_checksum_pages_enabled` | Data-page checksums on or off | Integrity policy and node-to-node drift. |
| `lustre_default_ea_size_bytes` | Default extended-attribute size | EA-related client tuning. |
| `lustre_maximum_ea_size_bytes` | Maximum extended-attribute size | xattr-heavy workload configuration. |
| `lustre_lazystatfs_enabled` | Lazy statfs on or off | Statfs load and configuration drift. |
| `lustre_maximum_read_ahead_megabytes` | Read-ahead ceiling | Sequential-read analysis. |
| `lustre_maximum_read_ahead_per_file_megabytes` | Per-file read-ahead ceiling | Large sequential-file tuning. |
| `lustre_maximum_read_ahead_whole_megabytes` | Whole-file read-ahead ceiling | Small and medium file workloads. |
| `lustre_statahead_agl_enabled` | Asynchronous glimpse-lock statahead | Directory metadata configuration. |
| `lustre_statahead_maximum` | Statahead request limit | Metadata concurrency tuning. |
| `lustre_xattr_cache_enabled` | xattr cache on or off | xattr-intensive performance analysis. |

These are configuration observations. They are a poor anomaly detector and
a good drift detector: the interesting question is usually "why is this
node different from the rest of the rack?" rather than "did the value
change in the last five minutes?"

## MDC and OSC RPC

This is the highest-leverage part of the client collector for incident
work. The source is `mdc/*/rpc_stats` and `osc/*/rpc_stats`.

### Historical RPC distributions

Three families record bucketed history from the `pages per rpc`,
`rpcs in flight`, and `offset` sections. The `size` label is the bucket,
not a measured byte count.

| Metric | Labels | Meaning | Typical use |
|---|---|---|---|
| `lustre_pages_per_rpc_total` | `operation`, `size` | Cumulative pages-per-RPC distribution | Whether RPCs have become small and inefficient. |
| `lustre_rpcs_in_flight` | `operation`, `size`, `type` | Historical in-flight RPC distribution | Concurrency pattern over time, not the live count. |
| `lustre_rpcs_offset` | `operation`, `size` | RPC offset distribution | Access locality and offset pattern. |

`lustre_rpcs_in_flight` is easy to misread. It is not "how many RPCs are
in flight right now." The current count is `lustre_rpcs_current`.

The RPC parser records `rpcs_in_flight` and `rpcs_offset` observations as
gauges, but the public contract defines both families as counters, and the
emitter uses the contract type. Treat the exposed type as Counter.

### Current RPC pressure

Scalar lines at the top of `rpc_stats` become gauges:

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_rpcs_current` | RPCs currently in flight | Congestion and RPC saturation. |
| `lustre_pending_pages` | Pages waiting to be sent | Client-side backlog. |

Both carry `component`, `target`, `type`, and `operation`. Observed
`operation` values include `read`, `write`, `modify`, `dio_read`, and
`dio_write`. Those names come from the file (`read RPCs in flight`,
`modify_RPCs_in_flight`, `pending write pages`, and so on).

These two gauges are the right starting point for a straggler that is
still connected to its targets:

```promql
max by (instance, target, type, operation) (
  lustre_rpcs_current
)
```

```promql
max_over_time(lustre_pending_pages[10m])
```

A useful pairing is high `pending_pages` with `lustre_rpcs_current` stuck
near `lustre_max_rpcs_in_flight` (or, for MDC modify, near
`lustre_max_mod_rpcs_in_flight`). That pattern means the client is issuing
RPCs at its configured ceiling and the far side or the path is not keeping
up. High pending pages with *low* in-flight RPCs points somewhere else:
import state, locks, or a concurrency setting that is too small.

Do not compare MDC `operation="modify"` against `lustre_max_rpcs_in_flight`.
Modify concurrency is limited by `lustre_max_mod_rpcs_in_flight`.

## Writeback, RPC limits, and import state

OSC and MDC single-value files supply the current writeback cache, the
configured RPC ceilings, and whether the import is usable.

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_osc_dirty_bytes` | Dirty bytes currently cached on the OSC | Writeback stall detection. |
| `lustre_osc_max_dirty_bytes` | Dirty-cache ceiling | Denominator for dirty utilization. Read from `max_dirty_mb` and converted to bytes. |
| `lustre_max_pages_per_rpc` | Maximum pages per RPC | RPC size tuning and drift. |
| `lustre_max_rpcs_in_flight` | Maximum concurrent RPCs | RPC concurrency limit. |
| `lustre_max_mod_rpcs_in_flight` | Maximum concurrent MDC modify RPCs | Metadata-modify concurrency. |
| `lustre_target_active` | Import active (`1`) or inactive (`0`) | Lost OST or MDT connectivity. |
| `lustre_target_state` | Current import state; the observed state series is `1` | Recovery and state-transition investigation. |

OSC contributes `cur_dirty_bytes`, `max_dirty_mb`, `max_pages_per_rpc`,
`max_rpcs_in_flight`, `active`, and `state`. MDC contributes the RPC
limits (including `max_mod_rpcs_in_flight`), `active`, and `state`. MDC
has no dirty-cache files; their absence is normal.

Dirty-cache utilization is:

```promql
lustre_osc_dirty_bytes / lustre_osc_max_dirty_bytes
```

A ratio that stays near `1` while write latency and `lustre_pending_pages`
rise is a strong writeback-pressure signal. New writes can stall once the
client is at the dirty ceiling.

`lustre_target_active == 0` is a hard fault signal for that import.
`lustre_target_state` is a state-set gauge: only the current state is
emitted, with value `1`. History rows from the Lustre `state` file are
discarded. After a transition the previous state series goes stale, so
alerts should match the live state rather than expecting a full enum of
zeroes. A practical check is:

```promql
lustre_target_state{state!~"FULL|IDLE"} == 1
```

Observed states include `FULL`, `IDLE`, `CONNECTING`, `DISCONN`, `REPLAY`,
`REPLAY_LOCKS`, `REPLAY_WAIT`, `RECOVER`, `CLOSED`, and `EVICTED`. The
exporter does not collapse those into healthy versus unhealthy; that
decision belongs in the query.

## LDLM callback activity

`lustre_ldlm_cbd_stats` counts operations from the LDLM callback daemon
(`ldlm/services/ldlm_cbd/stats`). The only label is `operation`. Typical
names include `req_waittime`, `ldlm_bl_callback`, `ldlm_cp_callback`, and
`ldlm_gl_callback`.

LDLM is Lustre's distributed lock manager. This family is an entry point
for callback, revoke, and cancel activity, not a lock-namespace inventory.
When metadata work is slow, compare:

```promql
sum by (operation) (
  rate(lustre_ldlm_cbd_stats[5m])
)
```

against llite operation rates and MDC or OSC `req_waittime`. A rise in
blocking callbacks together with rising getattr or unlink rates is a lock
contention hypothesis. The exporter does not yet expose per-namespace
grant and cancel rates; those remain future work.

## LNet traffic

LNet message counters come from debugfs or `/proc/sys/lnet/stats`, or from
`lnetctl stats show` when that path is selected or used as a fallback.

| Metric | Type | Meaning | Typical use |
|---|---|---|---|
| `lustre_allocated` | Gauge | LNet messages currently allocated | Message-resource pressure. |
| `lustre_maximum` | Gauge | Peak allocated LNet messages | Peak pressure. |
| `lustre_errors_total` | Counter | LNet errors | Network and LNet error rate. |
| `lustre_send_count_total` | Counter | Messages sent | Message rate. |
| `lustre_receive_count_total` | Counter | Messages received | Message rate. |
| `lustre_route_count_total` | Counter | Messages routed | Router use. |
| `lustre_drop_count_total` | Counter | Messages dropped | Path-failure indicator. |
| `lustre_send_bytes_total` | Counter | Bytes sent | LNet transmit throughput. |
| `lustre_receive_bytes_total` | Counter | Bytes received | LNet receive throughput. |
| `lustre_route_bytes_total` | Counter | Bytes routed | Router traffic. |
| `lustre_drop_bytes_total` | Counter | Bytes dropped | Impact of drops. |

The usual dashboard set is:

```promql
rate(lustre_send_bytes_total[5m])
```

```promql
rate(lustre_receive_bytes_total[5m])
```

```promql
rate(lustre_drop_count_total[5m])
```

```promql
rate(lustre_errors_total[5m])
```

When `-collector.lnet.source=lnetctl` and `lnetctl net show` returns
per-NID counters, `send_count_total`, `receive_count_total`, and
`drop_count_total` are published with a `nid` label. The matching global
series from `lnetctl stats show` are dropped so the same names do not
appear with two label sets in one scrape. In that mode you can look for
NIC or LNet-interface imbalance with:

```promql
sum by (nid) (
  rate(lustre_send_count_total[5m])
)
```

The default `auto` source does not do that. If debugfs or
`/proc/sys/lnet/stats` can be read, `auto` keeps the global counters and
only appends NI health extras from `lnetctl net show`. Per-NID send,
receive, and drop counts are added only on the full `lnetctl` path.

Byte counters stay global in all sources. `lustre_lnet_ni_health_dropped_total`
is a different quantity from `lustre_drop_count_total`: the former is an
NI health-stat drop, the latter is an LNet message drop.

## LNet network-interface health

Local NI health is available only when `lnetctl net show -v 3` succeeds.
Verbose level 3 is required because that is the level that includes health
stats; `-v 4` is not used. If `-v 3` fails, the exporter falls back to
plain `lnetctl net show`, which can still provide `status` and therefore
`lustre_lnet_ni_up`, but not the health counters. `-collector.lnet.source=debugfs`
never runs `lnetctl`, so these series are absent. The exporter does not
call `lnetctl peer show`.

All of the following carry a `nid` label:

| Metric | Type | Meaning | Typical use |
|---|---|---|---|
| `lustre_lnet_ni_up` | Gauge | NI up (`1`) or down (`0`) | Direct NIC or LNet-interface failure. |
| `lustre_lnet_ni_health` | Gauge | NI health value, maximum `1000` | Degraded interface. |
| `lustre_lnet_ni_health_interrupts_total` | Counter | Health interrupts | Cause analysis for NI degradation. |
| `lustre_lnet_ni_health_dropped_total` | Counter | Health-related drops | Packet or message loss on that NI. |
| `lustre_lnet_ni_health_aborted_total` | Counter | Aborted operations | Transport faults. |
| `lustre_lnet_ni_health_no_route_total` | Counter | No-route events | Routing or path configuration failure. |
| `lustre_lnet_ni_health_timeouts_total` | Counter | Timeouts | Congestion, path failure, or NIC trouble. |
| `lustre_lnet_ni_health_errors_total` | Counter | NI health errors | Per-NI error detector. |

`lustre_lnet_ni_up` is derived from `status: up`. Health counters are
emitted only when a `health stats` block is present. A health value of `0`
is a real observation and is not treated as "missing."

For HPC or AI training nodes, these series are often the difference
between "the filesystem is slow" and "this client's o2ib or kfi path is
degraded." Two cheap predicates are:

```promql
lustre_lnet_ni_health < 1000
```

```promql
rate(lustre_lnet_ni_health_timeouts_total[5m]) > 0
```

Follow both by `nid` and `instance`. If only one client and one NID move,
the incident is local. If every NID on a rack drops together, look at the
shared fabric before looking at a single OST.

## LNet configuration and internal state

Parameter files under debugfs and `/proc/sys/lnet` become the following
gauges and counters. All use `component="lnet"` and `target="lnet"`.

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_console_backoff_enabled` | Console-message backoff | Configuration audit. |
| `lustre_console_max_delay_centiseconds` | Maximum console delay | Logging configuration. |
| `lustre_console_min_delay_centiseconds` | Minimum console delay | Logging configuration. |
| `lustre_console_ratelimit_enabled` | Console rate limit | Configuration audit. |
| `lustre_debug_megabytes` | Debug buffer size in megabytes | Debug configuration. |
| `lustre_panic_on_lbug_enabled` | Panic-on-LBUG setting | Safety policy and drift. |
| `lustre_watchdog_ratelimit_enabled` | Watchdog rate limit | Watchdog configuration. |
| `lustre_catastrophe_enabled` | Catastrophic error flag | Strong fault signal. |
| `lustre_lnet_memory_used_bytes` | Current LNet memory use | LNet memory pressure. |
| `lustre_fail_error_total` | LNet fail-error counter | Failure and debugging context. |
| `lustre_fail_maximum` | LNet fail maximum | Failure and debugging context. |

In ordinary monitoring, `lustre_catastrophe_enabled` and
`lustre_lnet_memory_used_bytes` are the two that earn a panel. The rest
are context: useful when you are already on a node, noisy if you alert on
them.

## SPTLRPC encryption page pools

The `sptlrpc` collector reads `sptlrpc/encrypt_page_pools` from debugfs,
falling back to procfs. The series describe the page pool used for secure
RPC encryption. They have no labels; `lustre_cache_access_total` and
`lustre_cache_miss_total` in this exporter refer only to that pool, not to
OST cache statistics.

| Metric | Type | Meaning | Typical use |
|---|---|---|---|
| `lustre_physical_pages` | Gauge | System physical pages | Baseline for pool sizing. |
| `lustre_pages_per_pool` | Gauge | Pages per pool | Pool geometry. |
| `lustre_maximum_pages` | Gauge | Maximum pages in the pools | Capacity ceiling. |
| `lustre_maximum_pools` | Gauge | Maximum number of pools | Capacity ceiling. |
| `lustre_pages_in_pools` | Gauge | Pages currently in pools | Utilization. |
| `lustre_free_pages` | Gauge | Free pages | Pool pressure. |
| `lustre_maximum_pages_reached_total` | Counter | Times the page limit was hit | Pool saturation. |
| `lustre_grows_total` | Counter | Pool growth events | Dynamic expansion. |
| `lustre_grows_failure_total` | Counter | Failed growth attempts | Allocation failure. |
| `lustre_shrinks_total` | Counter | Pool shrink events | Resizing activity. |
| `lustre_cache_access_total` | Counter | Pool cache accesses | Cache workload. |
| `lustre_cache_miss_total` | Counter | Pool cache misses | Cache effectiveness. |
| `lustre_free_page_low` | Gauge | Free-page low watermark | Historical pressure. |
| `lustre_maximum_waitqueue_depth` | Gauge | Maximum wait-queue depth | Encryption resource contention. |
| `lustre_out_of_memory_request_total` | Counter | Out-of-memory conditions | Resource exhaustion. |

If the site uses SPTLRPC encryption, watch growth failures, OOM requests,
and page-limit hits:

```promql
rate(lustre_grows_failure_total[5m])
```

```promql
rate(lustre_out_of_memory_request_total[5m])
```

```promql
rate(lustre_maximum_pages_reached_total[5m])
```

On clusters that do not encrypt client RPCs, this collector can stay
enabled at a low priority. The files are cheap to read; the series are
rarely the first explanation for a training-step straggler.

## LPCC

Persistent Client Cache metrics are collected only when `-collector.lpcc`
is set. The collector runs `lpcc status` and parses its JSON. Percentage
fields from that command are divided by 100 and exported as ratios in
`[0, 1]`.

### Per cache

Labels are `mount` and `cache`.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_pcc_status` | Gauge | Cache process running (`1`) or not. |
| `lustre_pcc_purge_status` | Gauge | Purge process running (`1`) or not. |
| `lustre_pcc_cache_usage_ratio` | Gauge | Current cache usage, `0`–`1`. |
| `lustre_pcc_purge_high_usage_ratio` | Gauge | Purge-start high watermark. |
| `lustre_pcc_purge_low_usage_ratio` | Gauge | Purge-stop low watermark. |
| `lustre_pcc_purge_interval_seconds` | Gauge | Purge scan interval. |
| `lustre_pcc_purge_scan_threads` | Gauge | Purge thread count. |
| `lustre_pcc_purge_scan_times_total` | Counter | Completed purge scans. |
| `lustre_pcc_purge_total_purged_objs_total` | Counter | Objects purged. |
| `lustre_pcc_purge_total_failed_objs_total` | Counter | Objects that failed to purge. |
| `lustre_pcc_purge_scanned_objs` | Gauge | Objects scanned in the last cycle. |
| `lustre_pcc_purge_purged_objs` | Gauge | Objects purged in the last cycle. |
| `lustre_pcc_cached_files` | Gauge | Files currently in cache. |
| `lustre_pcc_cached_bytes` | Gauge | Bytes currently in cache. |
| `lustre_pcc_min_cached_file_size_bytes` | Gauge | Smallest cached file. |
| `lustre_pcc_max_cached_file_size_bytes` | Gauge | Largest cached file. |
| `lustre_pcc_average_age_seconds` | Gauge | Mean age of cached files. |

### Per mount

Labels are `mount` only. These series answer whether PCC is reducing
remote Lustre I/O, not merely whether a cache exists.

| Metric | Type | Meaning | Typical use |
|---|---|---|---|
| `lustre_pcc_fs_open_count_total` | Counter | File opens | Workload volume. |
| `lustre_pcc_fs_real_hit_total` | Counter | PCC hits | Cache effectiveness. |
| `lustre_pcc_fs_open_hit_ratio` | Gauge | Open hit ratio | Direct PCC benefit. |
| `lustre_pcc_fs_real_hit_bytes_total` | Counter | Bytes served from PCC | Offload volume. |
| `lustre_pcc_fs_total_read_bytes_total` | Counter | Total bytes read | Denominator for byte hit ratio. |
| `lustre_pcc_fs_read_hit_bytes_ratio` | Gauge | Read-byte hit ratio | Data-volume view of PCC benefit. |

A cache that is `running` with a high `lustre_pcc_cache_usage_ratio` and a
near-zero `lustre_pcc_fs_read_hit_bytes_ratio` is occupying local disk
without shielding the fabric. That is a tuning problem, not a Lustre
outage.

## Exporter scrape quality

Three families describe whether the exporter itself is healthy. They are
the right place to look before trusting a gap in Lustre series.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_scrape_collector_success` | Gauge | Collector succeeded (`1`) or failed (`0`). Labeled `collector`. |
| `lustre_scrape_collector_duration_seconds` | Gauge | Last scrape duration for that collector. Labeled `collector`. |
| `lustre_exporter_scrape_duration_seconds` | Summary | Scrape duration by `source` and `result`. |

The summary is exposed as the usual `_count` and `_sum` series:

```text
lustre_exporter_scrape_duration_seconds_count
lustre_exporter_scrape_duration_seconds_sum
```

`result` is `success` or `error`. `source` records how the data was read,
not only the collector name. Current values include `sysfs` (health),
`procfs` (client), `sys` (sptlrpc, and LNet when debugfs or proc stats
succeed), `lctl` (LNet after an `lnetctl` fallback or an explicit
`lnetctl` source), and `lpcc`. When LNet `auto` mode fails over from
debugfs to `lnetctl`, the recorded source changes from `sys` to `lctl`.

In non-strict mode a failed collector still leaves the scrape HTTP-200 and
sets `lustre_scrape_collector_success` to `0`. In `-collector.strict` mode
a source failure fails the scrape. Partial data is the default; an alert
on collector success is what tells you that the silence is an exporter
problem rather than a quiet filesystem.

## What to look at first

You do not need every series on a default dashboard. For client faults and
stragglers, this mapping is enough:

| Question | Start here |
|---|---|
| Has this client lost a Lustre target? | `lustre_target_active`, `lustre_target_state`, `lustre_health_check` |
| Has storage throughput dropped? | `rate(lustre_read_bytes_total[5m])`, `rate(lustre_write_bytes_total[5m])` |
| Is RPC wait rising on this client? | `lustre_stats_seconds_sum / lustre_stats_total` for `req_waittime` |
| Is the client RPC-saturated? | `lustre_rpcs_current`, `lustre_pending_pages` |
| Is writeback stuck? | `lustre_osc_dirty_bytes / lustre_osc_max_dirty_bytes` |
| Is a local LNet interface down or degraded? | `lustre_lnet_ni_up`, `lustre_lnet_ni_health` |
| Is the LNet path shedding work? | NI timeout, drop, abort, and error counters |
| Is traffic pinned to one NID? | Per-NID `lustre_send_count_total` / `lustre_receive_count_total` (lnetctl source) |
| Is the workload metadata-heavy? | `lustre_stats_total` by `operation` |
| Is lock-callback activity rising? | `lustre_ldlm_cbd_stats` |
| Is the exporter the broken piece? | `lustre_scrape_collector_success` |

On an AI or HPC cluster the common complaint is that one node’s training
step is slower than the rest. Align these time series for that instance:

1. OSC and MDC `req_waittime` means
2. `lustre_rpcs_current` and `lustre_pending_pages`
3. OSC dirty-cache ratio
4. `lustre_lnet_ni_health` and NI timeouts
5. per-NID send and receive rates, when available
6. `lustre_target_active` and `lustre_target_state`

That sequence stays entirely on the client. It can tell you whether the
node has lost an import, filled its writeback cache, run into its RPC
ceiling, or is talking through a degraded LNet interface. It cannot tell
you server queue depth, quota exhaustion, or MDS recovery. Those answers
need a server exporter. If throughput is down and client wait, RPC
pressure, LNet health, and import state are all normal, the stall is
probably above Lustre: the application, local CPU, or I/O that never left
the node.
