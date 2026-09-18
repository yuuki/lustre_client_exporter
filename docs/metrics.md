# Lustre Client Metrics

This guide explains the Prometheus metrics emitted by
`lustre_client_exporter` and how to use them in operations. It describes
the public metric contract as implemented in this repository, not a
server-side Lustre exporter and not a catalog of every Lustre proc file.

The exporter reads client-visible state from procfs, sysfs, debugfs, and
optionally `lnetctl` and `lpcc`. Public names and labels follow
[GSI-HPC/lustre_exporter](https://github.com/GSI-HPC/lustre_exporter) where
the same client-side quantity already exists. The implementation itself is
original. The single source of truth for names, types, help text, and label
keys is `internal/mapper/contract.go`.

Server-side Lustre data is intentionally out of scope: OST, MDT, MDS, MGS,
quota, recovery, exports, changelog, jobstats, and server BRW or service
statistics are not collected. A client node can still answer "is this node
the straggler, and where on the client path is it waiting?" without MDS or
OSS metrics. It cannot answer server queue depth, quota exhaustion, or MDS
recovery.

## Collectors and scrape surface

Four collectors are enabled by default: `client`, `lnet`, `health`, and
`sptlrpc`. The LPCC collector is off unless you pass `-collector.lpcc`.

The HTTP handler uses a dedicated `prometheus.Registry`. The usual Go
runtime and process families (`go_*`, `process_*`) are therefore absent.
Scrape quality is covered by `lustre_scrape_*` and
`lustre_exporter_scrape_duration_seconds`, described at the end of this
document.

Health, import, RPC, and LNet series are the first investigation set, not
a paging recipe by themselves. Pair them with user impact (a slow step, a
failed open, a stalled write). `lustre_health_check` alone is not a page:
a node can keep doing I/O while that gauge is `0`, and it can be `1` with
terrible RPC wait.

## How series are keyed

Most client filesystem and RPC series carry `component="client"` plus a
`target` label. `component` does **not** distinguish llite from MDC from
OSC. The published value is always `"client"` for those families.

`target` is the directory name under `llite/`, `mdc/`, or `osc/`:

- For llite, that is the client mount identity.
- For MDC and OSC, that is the import name. Lustre usually embeds
  `-mdc-` or `-osc-` in those names (for example
  `scratch-OST0000-osc-ffff0001`). That pattern is a naming convention,
  not a label the exporter adds.

Families that the contract uses to separate metadata from object I/O also
carry `type="mdc"` or `type="osc"`. Several widely used families do
**not**: `lustre_read_bytes_total`, `lustre_write_bytes_total`,
`lustre_stats_total`, `lustre_pages_per_rpc_total`, and
`lustre_rpcs_offset`. `{type="osc"}` cannot select those series. Split
them with `target`.

LNet series use `component="lnet"` and `target="lnet"`, and add `nid`
when the source is per-interface. SPTLRPC series have no labels. LPCC
uses `mount` and, for per-cache series, `cache`.

When a PromQL example omits `instance`, add it in a multi-node query.

## Node health

`lustre_health_check` is a gauge from this node's
`/sys/fs/lustre/health_check`. The parser treats the literal string
`healthy` as `1` and every other value, including empty or
`NOT HEALTHY`, as `0`. Labels are `component="health"` and
`target="lustre"`. `target="lustre"` is a fixed string, not "the whole
filesystem is healthy."

This is Lustre's own per-node health state, not cluster, fabric, MDS, or
OSS health, and not a performance SLO. If the gauge is `0`, look next at
import state, LNet NI status, and RPC pressure on **this** instance. Do
not treat the gauge as an answer to "has this client lost a target." It
can stay `1` while an OSC import is `DISCONN`, and it can be `0` while
every import is `FULL`.

## Filesystem capacity and inodes

The client collector reads the single-value llite files under each mount
(`kbytesavail`, `kbytesfree`, `kbytestotal`, `filesfree`, `filestotal`,
`blocksize`) and publishes them as gauges with `component="client"` and
`target` set to the mount name.

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_blocksize_bytes` | Raw `blocksize` | Configuration check. Rarely useful as an alert. |
| `lustre_available_kibibytes` | Raw `kbytesavail` | Client-visible available KiB from statfs. |
| `lustre_free_kibibytes` | Raw `kbytesfree` | Client-visible free KiB from statfs. |
| `lustre_capacity_kibibytes` | Raw `kbytestotal` | Denominator for a client-side ratio. |
| `lustre_inodes_free` | Raw `filesfree` | Client-visible free inodes. |
| `lustre_inodes_maximum` | Raw `filestotal` | Denominator for a client-side inode ratio. |

The exporter copies those files. It does not apply a reserved-space
model, quota, or OST grant. `available` versus `free` is whatever the
kernel reported in those two files.

A client-side utilization sketch is:

```promql
1 - lustre_available_kibibytes / lustre_capacity_kibibytes
```

```promql
1 - lustre_inodes_free / lustre_inodes_maximum
```

Those ratios can look healthy while writes fail: quota, grant, a single
full OST behind an aggregated statfs, or a stale lazy statfs. If
`lustre_lazystatfs_enabled` is `1`, treat the capacity gauges as
potentially stale by design. Use server OSD space metrics when you have
them.

## Client I/O

`read_bytes` and `write_bytes` lines become four series each: sample
count, lifetime min size, lifetime max size, and byte sum. The mapping
is used for llite `stats` and again for MDC and OSC `stats` when those
files contain the same line names. The public names are shared. The only
distinguisher is `target`. There is no `type` label.

The same public names are used for both planes. The fixtures already
emit `write_bytes` on an llite mount and again on an OSC import. Adding
those series together double-counts the same user write.
`sum(rate(lustre_write_bytes_total[5m]))` is therefore not application
throughput. For application throughput, keep the llite mount `target`.
For wire or RPC bytes, keep OSC or MDC import `target`s. Do not add the
two planes.

Lustre import names usually contain `-mdc-` or `-osc-`. That is a
heuristic for splitting planes, not a contract label. If a site uses
atypical names, list the mount `target`s explicitly.

```promql
# Application / llite plane (heuristic: drop usual import names)
sum by (instance, target) (
  rate(lustre_write_bytes_total{target!~".*-(mdc|osc)-.*"}[5m])
)
```

```promql
# OSC import / wire plane
sum by (instance, target) (
  rate(lustre_write_bytes_total{target=~".*-osc-.*"}[5m])
)
```

| Metric | Type | Meaning | Typical use |
|---|---|---|---|
| `lustre_read_samples_total` | Counter | Samples from a `read_bytes` line | Read IOPS-like activity on that `target`. |
| `lustre_read_bytes_total` | Counter | Sum from a `read_bytes` line | Read bytes on that `target` only. |
| `lustre_read_minimum_size_bytes` | Gauge | Lifetime minimum read size | Not a scrape-window minimum. One tiny read at mount keeps the value small forever. |
| `lustre_read_maximum_size_bytes` | Gauge | Lifetime maximum read size | I/O size characterization since reset. |
| `lustre_write_samples_total` | Counter | Samples from a `write_bytes` line | Write IOPS-like activity on that `target`. |
| `lustre_write_bytes_total` | Counter | Sum from a `write_bytes` line | Write bytes on that `target` only. |
| `lustre_write_minimum_size_bytes` | Gauge | Lifetime minimum write size | Same lifetime caveat as the read minimum. |
| `lustre_write_maximum_size_bytes` | Gauge | Lifetime maximum write size | I/O size characterization since reset. |

These counts are not block-layer I/O and will not necessarily match
`node_exporter` disk stats. A cached write can increment an llite series
before an OSC RPC leaves the node; writeback can increment an OSC series
after the application has returned.

## Operation counts and client-observed latency

### `lustre_stats_total`

`lustre_stats_total` is a counter labeled `component`, `target`, and
`operation`. There is no `type` label. Adding one would break the llite
series, which share this family.

For llite, every `stats` line except `read_bytes`, `write_bytes`, and
`snapshot_time` is mapped here. For MDC and OSC, every named line except
`read_bytes`, `write_bytes`, and `snapshot_time` is mapped the same way.
Blank lines and lines with fewer than two fields are skipped. Operation
names are taken from the kernel file as-is.

Do not `sum by (operation)` across the whole family. The same
`operation` string can appear on every MDC and OSC import.
`req_waittime` and `req_active` in the fixtures are import RPC-service
counters, not user metadata ops, and they will dominate a cluster-wide
sum. llite `getattr` and MDC `mds_getattr` are different names; the
collision that makes a metadata-versus-data overview wrong is
`req_waittime`. Filter to a mount `target` or to imports whose names
contain `-mdc-` or `-osc-` before grouping:

```promql
sum by (instance, target, operation) (
  rate(lustre_stats_total{target=~".*-mdc-.*"}[5m])
)
```

### `lustre_stats_seconds_sum`

MDC and OSC `stats` lines whose unit is a time unit also emit
`lustre_stats_seconds_sum`. The exporter converts nsec/nsecs, usec/usecs,
msec/msecs, and sec/secs to seconds and stores the cumulative sum.
Non-time units (`reqs`, `bytes`, `bufs`, and anything else) do not emit
this family. llite never emits it. Labels are `component`, `target`,
`type`, and `operation`.

The mean over a window is the ratio of the two **rates**, with
`ignoring(type)` because only the numerator has `type`. Matching still
requires `target` (and `instance`, `component`, `operation`). That is
enough to keep one OSC import from joining a different MDC import unless
the two share a `target` string.

```promql
rate(lustre_stats_seconds_sum{type="osc",operation="req_waittime"}[5m])
/ ignoring(type)
rate(lustre_stats_total{operation="req_waittime"}[5m])
```

Without `ignoring(type)` the vector is empty, which looks like "wait is
not rising." Without `rate()` the ratio is the lifetime mean, not the
last five minutes. Do not `sum()` the two sides independently and then
divide: the untyped denominator includes every import that has that
operation.

The same pattern works for other timed operations if you change
**both** `operation` and `type`. `mds_getattr` and `ldlm_cancel` are
MDC (`type="mdc"`). `ost_read` and `ost_write` are OSC. Copying the OSC
selector onto an MDC operation yields an empty vector.

This average is what the client recorded from send to reply. It is not
server-side queue depth, and it is not a percentile. Network RTT, server
processing, server queues, and LNet router delay are folded together. A
rising `req_waittime` mean tells you that this client is waiting longer
on that import. It does not prove that an OSS thread pool is saturated.

## Client tunables

llite runtime settings are exported as gauges on the same
`component`/`target` labels as the mount they belong to. Missing files
are skipped even in strict mode.

| Metric | Meaning | Typical use |
|---|---|---|
| `lustre_checksum_pages_enabled` | Data-page checksums on or off | Integrity policy and node-to-node drift. |
| `lustre_default_ea_size_bytes` | Default extended-attribute size | EA-related client tuning. |
| `lustre_maximum_ea_size_bytes` | Maximum extended-attribute size | xattr-heavy workload configuration. |
| `lustre_lazystatfs_enabled` | Lazy statfs on or off | Configuration drift, and a reason capacity gauges may be stale. |
| `lustre_maximum_read_ahead_megabytes` | Read-ahead ceiling | Sequential-read analysis. |
| `lustre_maximum_read_ahead_per_file_megabytes` | Per-file read-ahead ceiling | Large sequential-file tuning. |
| `lustre_maximum_read_ahead_whole_megabytes` | Whole-file read-ahead ceiling | Small and medium file workloads. |
| `lustre_statahead_agl_enabled` | Asynchronous glimpse-lock statahead | Directory metadata configuration. |
| `lustre_statahead_maximum` | Statahead request limit | Metadata concurrency tuning. |
| `lustre_xattr_cache_enabled` | xattr cache on or off | xattr-intensive performance analysis. |

These are configuration observations. They are a poor anomaly detector
and a good drift detector: the interesting question is usually "why is
this node different from the rest of the rack?"

## MDC and OSC RPC

The source is `mdc/*/rpc_stats` and `osc/*/rpc_stats`. If an import is
found via `stats`, `rpc_stats` is the sibling of that `stats` file and
is not replaced by a later glob in another tree. The standalone
`rpc_stats` glob only adds imports that had no `stats` hit, or fills an
empty path. A missing `rpc_stats` is skipped even in strict mode.

### Historical RPC distributions

Three families record bucketed history from the `pages per rpc`,
`rpcs in flight`, and `offset` sections. The `size` label is the bucket
key from the file, not a measured byte count. The buckets are discrete
counts, not Prometheus `le` histograms. `histogram_quantile` is not
available from this exporter.

| Metric | Type | Label keys | Meaning |
|---|---|---|---|
| `lustre_pages_per_rpc_total` | Counter | `component`, `target`, `operation`, `size` | Cumulative pages-per-RPC distribution. No `type`. |
| `lustre_rpcs_in_flight` | Counter | `component`, `target`, `operation`, `size`, `type` | Historical in-flight RPC distribution. Not the live count. |
| `lustre_rpcs_offset` | Counter | `component`, `target`, `operation`, `size` | RPC offset distribution. No `type`. |

`lustre_rpcs_in_flight` is not "how many RPCs are in flight right now."
The current count is `lustre_rpcs_current`. Always keep `target` when
aggregating, or every import is mixed. On pages-per-RPC and offset,
MDC versus OSC is the import `target` name only.

The RPC parser records `rpcs_in_flight` and `rpcs_offset` observations
as gauges, but the public contract defines both families as counters,
and the emitter uses the contract type. The exposed TYPE is Counter.
`rate()` after an `rpc_stats` reset is a reset artifact, not a
concurrency change. Prefer `lustre_rpcs_current` for live pressure.

A `modify` header can produce `operation="modify"` on the
`lustre_rpcs_in_flight` histogram. That is still historical bucket
data, not pending pages.

### Current RPC pressure

Scalar lines at the top of `rpc_stats` become gauges labeled
`component`, `target`, `type`, and `operation`.

| Metric | Operations actually emitted | Meaning |
|---|---|---|
| `lustre_rpcs_current` | `read`, `write`, `dio_read`, `dio_write`, `modify` | RPCs currently in flight for that operation. From `read/write/dio read/dio write RPCs in flight` and `modify_RPCs_in_flight`. |
| `lustre_pending_pages` | `read`, `write` only | From `pending read pages` and `pending write pages`. There is no modify or DIO pending-pages scalar. |

`operation="write"` pending pages are dirty pages waiting to go out.
`operation="read"` pending pages are pages reserved for in-flight
reads, not a send backlog. For writeback RCA use
`lustre_pending_pages{operation="write"}`.

`lustre_max_rpcs_in_flight` is one published ceiling per import. The
exporter does not encode whether Lustre shares that ceiling across
read, write, and DIO. Comparing a single operation to the ceiling can
show headroom while the import is already at the limit. A conservative
check sums the non-modify operations on that import:

```promql
sum by (instance, job, component, target, type) (
  lustre_rpcs_current{operation=~"read|write|dio_read|dio_write"}
)
/ lustre_max_rpcs_in_flight
```

The `sum by` list must keep every label that
`lustre_max_rpcs_in_flight` still has (`instance`, `job`, `component`,
`target`, `type`). Dropping `job` or `component` makes the ratio empty,
which looks like headroom. Compare only
`lustre_rpcs_current{operation="modify"}` to
`lustre_max_mod_rpcs_in_flight`. The two MDC pools are independent:
both can be pegged at once. Never test modify against
`lustre_max_rpcs_in_flight`, and never pair modify with
`lustre_pending_pages`.

```promql
max_over_time(lustre_pending_pages{type="osc",operation="write"}[10m])
```

High write pending pages with the non-modify in-flight sum near
`lustre_max_rpcs_in_flight` means the client is issuing RPCs at the
published ceiling and the far side or the path is not keeping up. High
write pending pages with a low in-flight sum means you are not at that
ceiling; look at import state or locks.

## Writeback, RPC limits, and import state

OSC and MDC single-value files supply the current writeback cache, the
configured RPC ceilings, and whether the import is marked active.
Missing files are skipped even in strict mode.

| Metric | Labels | Meaning |
|---|---|---|
| `lustre_osc_dirty_bytes` | `component`, `target` | Raw `cur_dirty_bytes` on an OSC. No `type`. |
| `lustre_osc_max_dirty_bytes` | `component`, `target` | `max_dirty_mb` multiplied by `1024 * 1024`. Fractional MB is allowed. No `type`. |
| `lustre_max_pages_per_rpc` | `component`, `target`, `type` | Maximum pages per RPC when the file exists. |
| `lustre_max_rpcs_in_flight` | `component`, `target`, `type` | Published non-modify RPC ceiling. |
| `lustre_max_mod_rpcs_in_flight` | `component`, `target`, `type` | Published MDC modify ceiling. |
| `lustre_target_active` | `component`, `target`, `type` | Import `active` file: `1` or `0`. |
| `lustre_target_state` | `component`, `target`, `type`, `state` | Observed `current_state`; that series is `1`. |

OSC is read for `cur_dirty_bytes`, `max_dirty_mb`, `max_pages_per_rpc`,
`max_rpcs_in_flight`, `active`, and `state`. MDC is read for
`max_rpcs_in_flight`, `max_mod_rpcs_in_flight`, `max_pages_per_rpc`,
`active`, and `state`. MDC has no dirty-cache files; their absence is
normal. Joins only work when both sides have the same label set, or you
explicitly ignore extras. A matcher such as `{operation="write"}` does
**not** drop `operation`.

- `lustre_osc_dirty_bytes / lustre_osc_max_dirty_bytes` matches (same
  labels).
- `lustre_rpcs_current{operation="modify"} / lustre_max_mod_rpcs_in_flight`
  needs `ignoring(operation)` (or `sum without (operation)`) on the
  numerator.
- Combining dirty bytes with `lustre_pending_pages` or
  `lustre_rpcs_current` needs `ignoring(type, operation)` on the RPC
  side. An empty ratio is a label mismatch, not "no writeback."

Dirty-cache utilization on one OSC import is:

```promql
lustre_osc_dirty_bytes / lustre_osc_max_dirty_bytes
```

A ratio that stays near `1` while write wait and
`lustre_pending_pages{operation="write"}` rise is a writeback-pressure
signal. New writes can stall once the client is at the dirty ceiling.

`lustre_target_active` is the raw `active` file (`1` or `0`). A lost
connection is not the same signal: the MDC fixture is `DISCONN` with
`active` `1`. Use `lustre_target_state` for connection and recovery.
Treat `lustre_target_active == 0` as the import marked inactive.

`lustre_target_state` is not an enum the exporter fills with zeroes.
The parser emits the first token after `current_state:` as the `state`
label, with value `1`. If that key is absent, it falls back to the
first token of a non-history line. History rows are discarded. Unknown
strings are exported as-is. After a transition the previous `state`
series goes stale. Use an instant query for the live state. Range
functions such as `max_over_time(...[1h])` still see the old label for
Prometheus's lookback. Missing `state` files emit nothing, so an alert
on this family stays silent.

Lustre commonly uses values such as `FULL`, `IDLE`, `CONNECTING`,
`DISCONN`, `REPLAY`, `REPLAY_LOCKS`, `REPLAY_WAIT`, `RECOVER`,
`CLOSED`, and `EVICTED`. Those are examples, not a closed set. A
practical instant check is:

```promql
lustre_target_state{state!~"^(FULL|IDLE)$"} == 1
```

## LDLM callback service

`lustre_ldlm_cbd_stats` counts samples from the LDLM callback service
file `ldlm/services/ldlm_cbd/stats` (debugfs, then procfs). It is not a
userspace daemon and not a lock-namespace inventory. The only label is
`operation`. The file is a PTLRPC service stat: names include
`req_waittime`, `req_qdepth`, `req_active`, `reqbuf_avail`,
`ldlm_bl_callback`, `ldlm_cp_callback`, and `ldlm_gl_callback`.

```promql
sum by (operation) (
  rate(lustre_ldlm_cbd_stats[5m])
)
```

Use that as activity on the callback service, not as proof of lock
contention. `ldlm_gl_callback` often tracks glimpse/AGL work.
`req_waittime` here is a sample count on the callback service, not the
MDC/OSC latency family. The exporter does not expose per-namespace
grant or cancel rates.

## LNet traffic

LNet message counters come from debugfs or `/proc/sys/lnet/stats`, or
from `lnetctl stats show` when that path is selected or used as a
fallback. Parameter files are read only after that primary stats source
succeeds. If both debugfs/proc stats and `lnetctl stats show` fail, the
LNet collector returns an error and no LNet parameter series are
emitted.

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `lustre_allocated` | Gauge | `component`, `target` | LNet messages currently allocated. |
| `lustre_maximum` | Gauge | `component`, `target` | Peak allocated LNet messages. |
| `lustre_errors_total` | Counter | `component`, `target` | LNet errors. |
| `lustre_send_count_total` | Counter | `component`, `target`, optional `nid` | Messages sent. |
| `lustre_receive_count_total` | Counter | `component`, `target`, optional `nid` | Messages received. |
| `lustre_route_count_total` | Counter | `component`, `target` | Messages routed. |
| `lustre_drop_count_total` | Counter | `component`, `target`, optional `nid` | Messages dropped. |
| `lustre_send_bytes_total` | Counter | `component`, `target` | Bytes sent. Always global. |
| `lustre_receive_bytes_total` | Counter | `component`, `target` | Bytes received. Always global. |
| `lustre_route_bytes_total` | Counter | `component`, `target` | Bytes routed. Always global. |
| `lustre_drop_bytes_total` | Counter | `component`, `target` | Bytes dropped. Always global. |

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

`-collector.lnet.source` selects the path:

| Source | Counters | Per-NID send/receive/drop | NI extras |
|---|---|---|---|
| `lnetctl` | `lnetctl stats show` (required) | Yes, from `net show`, when any local NI has a non-empty `nid` (zero counts still count). Global send/receive/drop counts from `stats show` are then dropped. | From `net show -v 3`, falling back to `net show`. A failed `net show` still leaves `stats show` counters and collector success `1`. |
| `auto` (debugfs/proc stats file is readable) | Parsed global counters if the file is non-empty and well-formed. An empty file is a successful read with no counters and does **not** fall back to `lnetctl`. | No | Non-fatal `net show -v 3` (fallback `net show`). NI extras only. |
| `auto` (those files fail) | Same as `lnetctl` | Same as `lnetctl` | Same as `lnetctl` |
| `debugfs` | Files only | No | No. `lnetctl` is not run. |

Byte, error, route, allocated, and `lustre_maximum` series stay global
on every path. `lustre_lnet_ni_health_dropped_total` is an NI
health-stat drop, not `lustre_drop_count_total`.

Per-NID imbalance is only queryable when the `nid` label is present:

```promql
sum by (instance, nid) (
  rate(lustre_send_count_total{nid!=""}[5m])
)
```

On default `auto`, when debugfs or proc stats succeed, those three
count families have no `nid` label. `sum by (nid)` then yields one
series with `nid=""`, the global total, not an empty vector and not a
single-NID pin. Require `nid!=""` (or the full `lnetctl` counter path)
before calling the result imbalance.

## LNet network-interface health

`lustre_lnet_ni_up` is emitted when `lnetctl net show` (verbose or not)
includes a `status` field. `up` / `UP` become `1`; any other non-empty
status becomes `0`. Health counters and `lustre_lnet_ni_health` are
emitted only when a `health stats` block is present. That block is why
the exporter prefers `lnetctl net show -v 3`. If `-v 3` fails, both the
`lnetctl` path and the `auto` supplemental path run plain
`lnetctl net show`, which can still produce `lustre_lnet_ni_up`.
`-collector.lnet.source=debugfs` never runs `lnetctl`, so none of these
series appear. The exporter does not call `lnetctl peer show`.

All of the following carry `component`, `target`, and `nid`:

| Metric | Type | Meaning |
|---|---|---|
| `lustre_lnet_ni_up` | Gauge | Local NI `status` is up (`1`) or not (`0`). |
| `lustre_lnet_ni_health` | Gauge | Raw `health value`. The exporter does not clamp it. Lustre's usual maximum is 1000. |
| `lustre_lnet_ni_health_interrupts_total` | Counter | Health interrupts. |
| `lustre_lnet_ni_health_dropped_total` | Counter | Health-stat drops. |
| `lustre_lnet_ni_health_aborted_total` | Counter | Aborted operations. |
| `lustre_lnet_ni_health_no_route_total` | Counter | No-route events. |
| `lustre_lnet_ni_health_timeouts_total` | Counter | Timeouts. |
| `lustre_lnet_ni_health_errors_total` | Counter | NI health errors. |

A health value of `0` is a real observation. The verbose fixture uses
`0` for `0@lo`. `lustre_lnet_ni_health < 1000` therefore matches
loopback and also matches a 999 after one recovered timeout. Absent
series (debugfs source, or `net show` without `health stats`) do not
match that predicate, which looks like "all NIs healthy."

Prefer `lustre_lnet_ni_up == 0` and rising timeout, drop, abort, or
error counters. Use the health value as context. Ignore or special-case
`nid=~".*@lo"`.
`rate(lustre_lnet_ni_health_timeouts_total[5m]) > 0` is a symptom, not
a page: one recovered timeout in five minutes is enough to fire.

If only one instance and one non-loopback NID move, the incident is
local. If every NID on a rack drops together, look at the shared fabric
before looking at a single OST.

## LNet configuration and internal state

After a successful LNet stats source, parameter files under debugfs and
`/proc/sys/lnet` become the following series. All use
`component="lnet"` and `target="lnet"`. Missing parameter files, and
parameter parse failures, are skipped even in strict mode. `fail_val`
also tries `fail_max`.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_console_backoff_enabled` | Gauge | Console-message backoff. |
| `lustre_console_max_delay_centiseconds` | Gauge | Maximum console delay. |
| `lustre_console_min_delay_centiseconds` | Gauge | Minimum console delay. |
| `lustre_console_ratelimit_enabled` | Gauge | Console rate limit. |
| `lustre_debug_megabytes` | Gauge | Debug buffer size in megabytes. |
| `lustre_panic_on_lbug_enabled` | Gauge | Panic-on-LBUG setting. |
| `lustre_watchdog_ratelimit_enabled` | Gauge | Watchdog rate limit. |
| `lustre_catastrophe_enabled` | Gauge | Catastrophic error flag. |
| `lustre_lnet_memory_used_bytes` | Gauge | Current LNet memory use. |
| `lustre_fail_error_total` | Counter | LNet fail-error counter. |
| `lustre_fail_maximum` | Gauge | LNet fail maximum (`fail_val` or `fail_max`). |

In ordinary monitoring, `lustre_catastrophe_enabled` and
`lustre_lnet_memory_used_bytes` are the two that earn a panel. The rest
are context.

## SPTLRPC encryption page pools

The `sptlrpc` collector reads `sptlrpc/encrypt_page_pools` from
debugfs, falling back to procfs. If neither file can be read, `Collect`
returns an error and `lustre_scrape_collector_success{collector="sptlrpc"}`
is `0`. That is the default-on collector: a node without those files
looks like a failed scrape, not like "encryption is unused."

The series have no labels. `lustre_cache_access_total` and
`lustre_cache_miss_total` in this exporter refer only to this pool, not
to OST cache statistics.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_physical_pages` | Gauge | The `physical pages` field from `encrypt_page_pools`, not `/proc/meminfo`. |
| `lustre_pages_per_pool` | Gauge | Pages per pool. |
| `lustre_maximum_pages` | Gauge | Maximum pages in the pools. |
| `lustre_maximum_pools` | Gauge | Maximum number of pools. |
| `lustre_pages_in_pools` | Gauge | Pages currently in pools. |
| `lustre_free_pages` | Gauge | Free pages in the pools. |
| `lustre_maximum_pages_reached_total` | Counter | Times the page limit was hit. |
| `lustre_grows_total` | Counter | Pool growth events. |
| `lustre_grows_failure_total` | Counter | Failed growth attempts. |
| `lustre_shrinks_total` | Counter | Pool shrink events. |
| `lustre_cache_access_total` | Counter | Pool cache accesses. |
| `lustre_cache_miss_total` | Counter | Pool cache misses. |
| `lustre_free_page_low` | Gauge | Free-page low watermark from the file. |
| `lustre_maximum_waitqueue_depth` | Gauge | Maximum wait-queue depth. |
| `lustre_out_of_memory_request_total` | Counter | Out-of-memory conditions recorded by the pool. |

If the site uses SPTLRPC encryption, watch growth failures, OOM
requests, and page-limit hits:

```promql
rate(lustre_grows_failure_total[5m])
```

```promql
rate(lustre_out_of_memory_request_total[5m])
```

```promql
rate(lustre_maximum_pages_reached_total[5m])
```

On clusters that do not expose `encrypt_page_pools`, disable the
collector (`-collector.sptlrpc=false`) instead of alerting on scrape
success.

## LPCC

Persistent Client Cache metrics are collected only when `-collector.lpcc`
is set. The collector runs `lpcc status` and parses its JSON.

If that command fails, `Collect` logs a warning and returns `nil, nil`.
`lustre_scrape_collector_success{collector="lpcc"}` stays `1` and every
`lustre_pcc_*` series is absent. That is the same shape as "PCC is not
configured." Treat missing PCC series with the collector enabled as
"command failed or empty status," not as a zero hit ratio. JSON or
mapper errors return an error and set success to `0`.

These JSON fields are divided by 100 with no clamp and no scale
detection: `cache_usage_pct`, `high_usage`, `low_usage`,
`pcc_open_hit_pct`, and `pcc_read_hit_bytes_pct`. The repository fixture
already mixes a fractional `cache_usage_pct` (`0.85`), a
percentage-looking `cache_usage_pct` (`45.123`), and a fractional
`pcc_open_hit_pct` (`0.00136`). Do not assume the exported gauges are
true ratios in `[0, 1]`.

`lustre_pcc_status` and `lustre_pcc_purge_status` are `1` only when the
JSON string is exactly `running`.

### Per cache

Labels are `mount` and `cache`. `mount` is the JSON object key, not
necessarily the inner `pcc[].mount` field.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_pcc_status` | Gauge | `status` is `running`. |
| `lustre_pcc_purge_status` | Gauge | `purge` is `running`. |
| `lustre_pcc_cache_usage_ratio` | Gauge | `cache_usage_pct / 100`. |
| `lustre_pcc_purge_high_usage_ratio` | Gauge | `high_usage / 100`. |
| `lustre_pcc_purge_low_usage_ratio` | Gauge | `low_usage / 100`. |
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

Labels are `mount` only.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_pcc_fs_open_count_total` | Counter | File opens from `fs_stats`. |
| `lustre_pcc_fs_real_hit_total` | Counter | PCC hits. |
| `lustre_pcc_fs_open_hit_ratio` | Gauge | `pcc_open_hit_pct / 100`. |
| `lustre_pcc_fs_real_hit_bytes_total` | Counter | Bytes served from PCC. |
| `lustre_pcc_fs_total_read_bytes_total` | Counter | Total bytes read from `fs_stats`. |
| `lustre_pcc_fs_read_hit_bytes_ratio` | Gauge | `pcc_read_hit_bytes_pct / 100`. |

When the source percentages are on a consistent scale, the per-mount
series are the ones that describe whether PCC is serving opens and
bytes. They do not, by themselves, prove that remote Lustre I/O fell.

## Exporter scrape quality

Three families describe the collector function, not completeness of
every target.

| Metric | Type | Meaning |
|---|---|---|
| `lustre_scrape_collector_success` | Gauge | `Collect` returned no error (`1`) or returned an error (`0`). Label: `collector`. |
| `lustre_scrape_collector_duration_seconds` | Gauge | Last `Collect` duration. Label: `collector`. |
| `lustre_exporter_scrape_duration_seconds` | Summary | Duration by `source` and `result`. |

The summary is exposed as `_count` and `_sum`:

```text
lustre_exporter_scrape_duration_seconds_count
lustre_exporter_scrape_duration_seconds_sum
```

`result` is `success` or `error`. `source` is a collector constant,
except LNet `auto`, which rewrites `sys` to `lctl` when debugfs/proc
stats fail and the collector falls back to `lnetctl`:

| Collector | `source` value | What it does **not** mean |
|---|---|---|
| `health` | `sysfs` | Always this string, because the path is under sysfs. |
| `client` | `procfs` | Still `procfs` when stats were read from sysfs or debugfs. |
| `sptlrpc` | `sys` | Still `sys` when the procfs fallback was used. The same `source` value is used for file-path LNet, so one summary series can merge both collectors. |
| `lnet` | `sys` or `lctl` | `debugfs` and successful `auto` record `sys`. Explicit `lnetctl` or `auto` fallback records `lctl`. |
| `lpcc` | `lpcc` | Recorded even when the command failed and no series were emitted. |

Success `0` is a hard `Collect` error. In `-collector.strict` that also
becomes an invalid scrape metric. Success `1` with missing optional
series is often a skipped file, not a failed collector.

| Outcome | When |
|---|---|
| Always a collector error (success `0`; strict fails the scrape) | Health, sptlrpc, or LNet primary-source/parse error. LPCC JSON or mapper error. Mapper error. |
| Strict: collector error. Non-strict: warn, success `1` | Client discovered `stats` read/parse error. `rpc_stats` read error other than not-exist. `rpc_stats` parse error. Optional-file parse error. LDLM parse error. |
| Skip even in strict, success `1` | Missing or unreadable optional llite/OSC/MDC files. `rpc_stats` not found. Missing `ldlm_cbd`. Missing LNet params or LNet param parse failure. Failed `lnetctl net show` after a successful `stats show` (explicit `lnetctl` or `auto` supplemental). `lpcc` command failure. Zero discovered client mounts. |

In non-strict mode a failed llite `stats` read skips that mount. A
failed MDC or OSC `stats` read still continues into `rpc_stats` and
param files for that import. Omitted series go stale or never appear,
which looks like a quiet filesystem or a vanished import. Those gaps
are a read failure only when the file was discovered and then failed.

## What to look at first

Keep `instance` and `target` in every comparison. Do not treat an
unfiltered family as a single quantity.

| Question | Start here |
|---|---|
| Has this client lost a Lustre target? | `lustre_target_state` whose `state` is not `FULL` or `IDLE`. `lustre_target_active == 0` is the import marked inactive (the MDC fixture is `DISCONN` with `active` 1). `lustre_health_check` does not answer this. |
| Has application throughput dropped? | `rate(lustre_read_bytes_total[5m])` and `rate(lustre_write_bytes_total[5m])` on llite mount `target`s only (exclude usual `-mdc-` / `-osc-` import names). |
| Has OSC wire throughput dropped? | The same rates on OSC import `target`s only. |
| Is RPC wait rising on this import? | `rate(lustre_stats_seconds_sum{operation="req_waittime"}[5m]) / ignoring(type) rate(lustre_stats_total{operation="req_waittime"}[5m])`. |
| Is this import near its published RPC ceiling? | Sum of `lustre_rpcs_current` for read, write, and DIO versus `lustre_max_rpcs_in_flight` (keep `instance`, `job`, `component`, `target`, `type`). Modify versus `lustre_max_mod_rpcs_in_flight` needs `ignoring(operation)`. |
| Is writeback backing up? | `lustre_osc_dirty_bytes / lustre_osc_max_dirty_bytes` and `lustre_pending_pages{type="osc",operation="write"}`. |
| Is a local LNet interface down? | `lustre_lnet_ni_up == 0` (skip `@lo` unless you care about loopback). Absent series mean `lnetctl net show` was not available, not that every NI is up. |
| Is the LNet path shedding work? | Rising NI timeout, drop, abort, or error counters, plus `lustre_drop_count_total`. |
| Is traffic pinned to one NID? | Per-NID `lustre_send_count_total` / `lustre_receive_count_total` only when the `nid` label exists (full `lnetctl` counter path). |
| Is this mount metadata-heavy? | `lustre_stats_total` on that mount `target`, by `operation`. |
| Is the callback service busy? | `lustre_ldlm_cbd_stats` by `operation`. Hypothesis, not proof of lock contention. |
| Did a collector function fail? | `lustre_scrape_collector_success == 0`. Success `1` does not mean every target was read. |

On an AI or HPC cluster the common complaint is that one node's
training step is slower than the rest. Align these series for that
`instance`, keeping `target`:

1. Per-import `req_waittime` means (`rate` / `rate`, `ignoring(type)`)
2. Non-modify `lustre_rpcs_current` sum and
   `lustre_pending_pages{type="osc",operation="write"}`
3. OSC dirty-cache ratio
4. `lustre_lnet_ni_up` and NI timeout counters (absent without `lnetctl`)
5. Per-NID send and receive rates, when the `nid` label exists
6. `lustre_target_state` first; `lustre_target_active` only as "marked inactive"

That sequence stays on the client. It can tell you whether the node has
an inactive import, a full writeback cache, an RPC ceiling, or a down
local NI. It cannot tell you server queue depth, quota exhaustion, or
MDS recovery. If application throughput is down and client wait, RPC
pressure, LNet NI status, and import state are all normal, the stall is
probably above Lustre: the application, local CPU, or I/O that never
left the node.
