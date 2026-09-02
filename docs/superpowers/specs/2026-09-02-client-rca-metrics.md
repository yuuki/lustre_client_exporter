# Client-side RCA metrics

この文書は、Lustre 文献と ChatGPT 相談結果を現行 `lustre_client_exporter` へ落とすための仕様である。
相談結果を採択するのではなく、現行実装・現行契約・client-only の境界に照らして採否を決める。

実装計画は `docs/superpowers/plans/2026-09-02-client-rca-metrics.md` を正とする。
この仕様は敵対的レビューのあと、計画と矛盾する分岐を潰した版である。

## 現行実装がすでに持っているもの

観測パイプラインは次の通りである。

```text
application
  → llite.stats / tunables          収集済み（operation は count のみ）
  → mdc/osc stats                   発見済み、未読
  → mdc/osc rpc_stats histogram     収集済み
  → mdc/osc rpc_stats 瞬間値        ファイル内にあるが未parse
  → OSC dirty / RPC limit           未収集
  → OSC/MDC active / state          未収集
  → LNet send/recv/drop             収集済み（nid は lnetctl net show 時のみ）
  → LNet NI health / status         未収集
  → ldlm_cbd.stats                  収集済み（count のみ）
```

`DiscoverClients()` は llite / mdc / osc の `stats` を glob し、`ClientTarget.StatsPath` に保持する。
`ClientCollector` は llite では `stats` を読むが、mdc / osc では `collectRPC()` だけを呼び、`rpc_stats` しか読まない。
fixture `testdata/mdc/stats.txt` と `testdata/osc/stats.txt` はすでに `req_waittime` と operation latency を含んでいる。

この点について相談結果は正しい。
最初の仕事は新機能の発明ではなく、発見済み入力を collector が捨てている状態を直すことである。

ただし「配線するだけ」ではない。
現行 `ParseLLiteStats` は `read_bytes` / `write_bytes` 以外の sum を捨てる。
mdc / osc の価値は usec の sum にあるので、別 parser が要る。

## 相談結果で採ってはいけない判断

### サーバ側メトリクスを client exporter に混ぜること

相談の前半は次を「最優先」としている。

- `recovery_status`
- `mdt.*.num_exports` / `obdfilter.*.num_exports`
- `osd-*.*.kbytes*`
- `mdt.*.md_stats`
- `obdfilter.*.stats`
- `mdt.*.job_stats` / `obdfilter.*.job_stats`
- server PTLRPC の `req_qdepth` / `req_active`
- `osd-*.*.brw_stats`
- `mds_reint`

`docs/design.md` はこれらを MVP から明示的に除外している。
このリポジトリは client node の exporter であり、GSI-HPC 互換のサーバ collector を再実装する場所ではない。

Qian の congestion-control も Xie の production variability も、サーバ側 queue と client 側観測を区別したうえで読まないと誤る。
client から見えるのは「この client がこの target に対してどれだけ待ったか」までである。
「OSS service が incoming をさばき切れていない」は server `ost_io` の `req_qdepth` と `req_waittime` が揃ったときの仮説であり、client `osc.*.stats` だけでは成立しない。

### client の `req_waittime` を server queue wait と同一視すること

Operations Manual の定義は server service statistics 向けである。

```text
req_waittime = request が server thread に拾われるまで queue で待った時間
```

client の `osc.*.stats` / `mdc.*.stats` の `req_waittime` は同じ名前でも同じ量ではない。
Lustre の client `after_reply` は `rq_sent_tv` から reply 受信までを usec で加算する。
つまり client-observed RPC 完了待ち（おおむね送信から reply まで）であり、server queue 滞留そのものではない。
network RTT、server 処理、server queue、LNet router queue が混ざる。

したがって次の PromQL は「この client から見た target 遅延」であり、「OSS 飽和」ではない。

```promql
rate(lustre_stats_seconds_sum{operation="req_waittime"}[5m])
/
rate(lustre_stats_total{operation="req_waittime"}[5m])
```

切り分けには LNet health、target state、dirty saturation、rpc 瞬間値を並べる。
server `req_qdepth` の代替にはしない。

### `req_active` / `req_qdepth` の stats 行を瞬間値と読むこと

stats 行は次の形である。

```text
req_active  200 samples [reqs] 1 16 800
```

これは sample 数、min、max、sum である。
「今処理中の RPC 数」ではない。
瞬間の in-flight は `rpc_stats` 先頭のスカラー行にある。

```text
read RPCs in flight:  3
write RPCs in flight: 5
pending write pages:  120
modify_RPCs_in_flight: 0
```

現行 `ParseRPCStats` はこれらの行を histogram の section header として消費し、値を metric にしない。
`TestParseRPCStats_SkipsScalarRpcsInFlightLines` がその意図を固定している。
histogram `lustre_rpcs_in_flight` は過去の in-flight 分布であり、今の concurrency ではない。

相談が挙げた RCA（pending pages が高いのに in-flight が低い）には、histogram ではなくこの瞬間値が要る。
P0 として stats より優先度は落ちない。

### `component="osc"` を新しい公開ラベルにすること

現行契約では client 系の `component` は `"client"` で固定する。
`lustre_rpcs_in_flight` は mdc / osc の区別を `type` ラベルで行う。
collector はすでに `ParseRPCStats(..., "client", t.Name, t.Component)` と渡している。

新しい系列も同じにする。
`lustre_stats_total` の label set に `type` を足してはならない。
同一 metric family の label key が llite と osc でずれると Prometheus gather が壊れる。

### `lustre_client_*` という新しい接頭辞

既存の公開名は `lustre_*` である。
`lustre_client_rpc_wait_seconds_*` は既存命名と揃わない。
req_waittime 専用 family も作らない。
timed operation はすべて `lustre_stats_seconds_sum{operation=...}` に載せる。

### min / max を window 統計のように出すこと

Lustre stats の min / max は mount または reset 以降の生涯極値である。
scrape 窓の min / max ではない。
llite の read/write size は GSI 互換のため極値をすでに出している。
latency に同じものを足すと「最近遅い」と誤読されやすい。
v1 では count と sum だけを出す。
mean は PromQL の `rate(sum)/rate(count)` で作る。

sum-of-squares は parser が読み飛ばす。
stddev は後で足せる。
Prometheus native histogram へ変換しない。
stats 行は histogram ではない。

### LNet health を「parser を少し拡張するだけ」と書くこと

現行 `lnetctl net show` は verbose ではない。
fixture `testdata/lnet/lnetctl_net_show.yaml` にも health は無い。
health stats は `lnetctl net show -v 3` で出る。

`auto` は debugfs を優先する。
debugfs の LNet stats には NI health が無い。
決めた動きは後述の source 表である。
peer show は peer 数で cardinality が増えるので default OFF は妥当である。

### `osc.*.state` を単語一つの gauge と書くこと

現行 Lustre の `osc.*.state` は次の複数行である。

```text
current_state: FULL
state_history:
 - [ 1781484433, CONNECTING ]
 - [ 1781484436, FULL ]
```

`current_state` だけを読む。
history は出さない。
値は FULL、IDLE、CONNECTING、DISCONN、REPLAY、REPLAY_LOCKS、REPLAY_WAIT、RECOVER、CLOSED、EVICTED などである。
FULL / IDLE を 1、それ以外を 0 に潰さない。
接続健全性は `lustre_target_active` と state-set を分けて持つ。

### 収集経路を `lctl get_param` に切り替えること

Wiki が `lctl get_param` を勧める理由は、procfs と sysfs の配置が Lustre 版で動くからである。
この exporter は procfs / sysfs / debugfs を直接読む設計で、GSI-HPC 互換の経路でもある。
scrape ごとに `lctl` を大量 spawn する設計にはしない。
不足があるなら discovery の glob を `/sys/fs/lustre` と debugfs へ広げる。

### JobStats を client の default に載せること

`docs/research.md` は JobStats を server-side かつ高コストとして default MVP から外している。
cardinality の警告自体は正しい。
この exporter の default には入れない。
Slurm JobID の分解は server exporter か、短い retention の別パイプラインの仕事である。

## 相談結果で採る判断

次は採る。

- mdc / osc の `stats` を読むこと。入口はすでにあり、client RCA の欠落として最も大きい。
- sum を捨てず、時間単位は seconds に正規化すること。
- OSC dirty と RPC stream tunables を出すこと。checkpoint write の stall を直接見る。
- OSC / MDC の `active` と `current_state` を出すこと。aggregate throughput では切れた OST を見逃す。
- local NI health を P0 相当として足すこと。ただし取得コマンドと auto 時の挙動を仕様化する。
- peer health は default OFF。
- `extents_stats` は opt-in。exporter は enable しない。
- `extents_stats_per_process` と `offset_stats` は default に入れない。
- timeouts、statahead_stats、ldlm namespace は P1。
- サーバ BRW / jobstats / recovery / exports は対象外のまま。

## 公開メトリクス契約

既存 GSI 互換名は変えない。
新しい family だけを足す。
HELP はオリジナル文にする。

共通ラベルは次の通りである。

- 既存 client 系: `component="client"`, `target=<llite|mdc|osc 名>`
- mdc / osc を区別する新しい family: さらに `type="mdc"|"osc"`
- LNet: `component="lnet"`, `target="lnet"`, 必要なら `nid`

### PR1 で足すもの

| MetricID | 公開名 | 型 | ラベル | 入力 |
|---|---|---|---|---|
| `stats_total` | `lustre_stats_total` | Counter | component, target, operation | mdc/osc `stats` の samples。既存 family へ加算 |
| `stats_seconds_sum` | `lustre_stats_seconds_sum` | Counter | component, target, type, operation | 時間単位行の sum。usec/usecs→/1e6、msec→/1e3、nsec→/1e9、sec/secs→そのまま |
| `rpcs_current` | `lustre_rpcs_current` | Gauge | component, target, type, operation | `read/write/dio read/dio write RPCs in flight`, `modify_RPCs_in_flight` |
| `pending_pages` | `lustre_pending_pages` | Gauge | component, target, type, operation | `pending read pages`, `pending write pages` |

`lustre_stats_total` に `type` は足さない。
llite 既存系列の label set を壊さない。
osc / mdc の区別は `target`（名前に `-osc-` / `-mdc-` が含まれる）と、latency family の `type` で行う。

`read_bytes` / `write_bytes` が osc stats にある場合は、llite と同じ 4 指標（samples / min / max / bytes）も出す。
これらの operation では `lustre_stats_total` を出さない（llite と同じ）。

時間単位でない行（`[reqs]`, `[bytes]`, `[bufs]`）は count だけ出す。
`req_active` の sum から平均 concurrency を作ることはできるが、瞬間値 `lustre_rpcs_current` の方が RCA に直結するので v1 では出さない。

`operation` の値はファイルの名前をそのまま使う。
`req_waittime`, `ost_read`, `mds_getattr`, `ldlm_cancel` を exporter 側で改名しない。

### PR2 で足すもの

| MetricID | 公開名 | 型 | ラベル | 入力 |
|---|---|---|---|---|
| `osc_dirty_bytes` | `lustre_osc_dirty_bytes` | Gauge | component, target | `osc/*/cur_dirty_bytes` |
| `osc_max_dirty_bytes` | `lustre_osc_max_dirty_bytes` | Gauge | component, target | `osc/*/max_dirty_mb` × 1024 × 1024 |
| `max_pages_per_rpc` | `lustre_max_pages_per_rpc` | Gauge | component, target, type | ファイルがあれば読む。OSC は通常ある。MDC に無ければ skip |
| `max_rpcs_in_flight` | `lustre_max_rpcs_in_flight` | Gauge | component, target, type | `*/max_rpcs_in_flight` |
| `max_mod_rpcs_in_flight` | `lustre_max_mod_rpcs_in_flight` | Gauge | component, target, type | `mdc/*/max_mod_rpcs_in_flight`。MDC modify の上限。`max_rpcs_in_flight` ではない |
| `target_active` | `lustre_target_active` | Gauge | component, target, type | `*/active`（0/1） |
| `target_state` | `lustre_target_state` | Gauge | component, target, type, state | `*/state` の `current_state`。現在値だけ 1 |

`max_dirty_mb` は整数とは限らない。`ParseFloat` する。
無い tunables ファイルは llite tunables と同じく skip する。strict でも欠落はエラーにしない。
parse に失敗したときだけ、strict なら scrape を落とす。
MDC に dirty が無いのは正常である。
MDC の modify 同時実行上限は `max_mod_rpcs_in_flight` である。
`lustre_rpcs_current{type="mdc",operation="modify"}` との比較に `lustre_max_rpcs_in_flight` を使ってはならない。

`lustre_target_state` は観測した state だけを 1 で出す。
既知 enum を全部 0 埋めしない。
series は state 変更後に stale になる。
alert は `lustre_target_state{state!~"FULL|IDLE"} == 1` を基本にする。

### PR3 で足すもの

| MetricID | 公開名 | 型 | ラベル | 入力 |
|---|---|---|---|---|
| `lnet_ni_up` | `lustre_lnet_ni_up` | Gauge | component, target, nid | `status: up` → 1 |
| `lnet_ni_health` | `lustre_lnet_ni_health` | Gauge | component, target, nid | `health value`（最大 1000） |
| `lnet_ni_health_interrupts_total` | `lustre_lnet_ni_health_interrupts_total` | Counter | component, target, nid | health stats |
| `lnet_ni_health_dropped_total` | `lustre_lnet_ni_health_dropped_total` | Counter | component, target, nid | health stats。既存 `lustre_drop_count_total` とは別量 |
| `lnet_ni_health_aborted_total` | `lustre_lnet_ni_health_aborted_total` | Counter | component, target, nid | health stats |
| `lnet_ni_health_no_route_total` | `lustre_lnet_ni_health_no_route_total` | Counter | component, target, nid | health stats |
| `lnet_ni_health_timeouts_total` | `lustre_lnet_ni_health_timeouts_total` | Counter | component, target, nid | health stats |
| `lnet_ni_health_errors_total` | `lustre_lnet_ni_health_errors_total` | Counter | component, target, nid | health stats |

取得コマンドは `lnetctl net show -v 3` である。
`docs/research.md` の whamcloud 調査は `-v 4` に触れている。
health stats は `-v 3` で出るので、この exporter は `-v 3` に固定する。
`-v 3` が失敗したら `lnetctl net show` へ落とす。

parser は 2 関数に分ける。混ぜない。

- `ParseLNetCtlNetStats` は nid 付き send/recv/drop だけを返す。現行の件数 6 テストを維持する。
- `ParseLNetCtlNetNI` は `status` があれば `lnet_ni_up` を返す。`health stats` があれば health 系を返す。
- `health value: 0` は正常な観測である。ブロックの有無を `!= 0` で判定してはならない。

`source` ごとの取得は次で固定する。

| source | 動き |
|---|---|
| `lnetctl` | `stats show` + `net show -v 3`（失敗時 `net show`）。counts と NI extras の両方 |
| `auto` | `collectFromDebugFS()` 成功なら、その count に加えて `net show -v 3` を非致命で足し、NI extras だけ append する。nid 付き count は足さない。`collectFromDebugFS()` 失敗なら現行どおり `collectFromLNetCtl`（counts と NI extras の両方）。`TestLNetCollector_Auto_FallbackToLNetCtl` と `failure_test.go` の同趣旨テストは変えない |
| `debugfs` | lnetctl を呼ばない。`RunCommand` を発行しない |

`collectFromDebugFS()` の成功は `reader.ReadFirstAvailable(..., discovery.LNetStatsPaths(cfg))` の成功である。
`LNetStatsPaths` は `[/sys/kernel/debug/lnet/stats, /proc/sys/lnet/stats]` である。
`/proc/sys/lnet/stats` が読めたら成功である。
debugfs のファイルだけを成功条件にしてはならない。
`newTestLNetFakeReader` と `newFullFakeReader` は proc の stats だけを載せる。
そのため Auto + それらの reader では `lnetctl stats show` は呼ばれない。

`source=lnetctl` で `net show` が count を返したら、global の `send_count_total` / `receive_count_total` / `drop_count_total` を落とす。
判定は MetricID である。
`len(Labels) == 0` では落とせない。
`ParseLNetCtlStats` は常に `{component,target}` を付ける。
nid 付き count の MetricID は `*_by_nid_total` なので、この 3 ID を落とせば label set は混ざらない。

`auto` の supplemental が count を足すと、global の `lustre_send_count_total`（nid 無し）と nid 付き family が同じ scrape に混ざる。
emitter は name+label keys で `Desc` をキャッシュする。
同一公開名で label set が違う系列は出さない。

command 失敗は debug log して捨てる。scrape 全体は落とさない。
`source-timeout` は collector の context が既に切る。

契約テスト `TestContract_*` が `lustre_lnet_ni_*` を期待するなら、FakeReader に `lnetctl net show -v 3` を載せ、LNet collector は `auto` にする。
debugfs だけの reader で名前を expected に足してはならない。

`--collector.lnet.peer-health` はこの仕様の後続である。
PR3 は `main.go` を触らない。
peer show は呼ばない。

### PR4（P1）で足すもの

実装前に実機 fixture を 1 本取る。
フォーマットが版で揺れるため、推測 parser を先に書かない。

対象は次である。

- `osc/*/timeouts` と `mdc/*/timeouts`（adaptive timeout の cur / worst / last_reply）
- `llite/*/statahead_stats`（設定値 `statahead_max` ではなく動作統計）
- `ldlm/namespaces/*` の `lock_count`, `pool/granted`, `pool/grant_rate`, `pool/cancel_rate`
- `--collector.client.extents` が true のときだけ `llite/*/extents_stats`

`ldlm_cbd` と namespace は別物である。
前者は callback service の PTLRPC stats、後者は lock の保有と churn である。
既存 `lustre_ldlm_cbd_stats` に足さない。

## 入力パス

stats と rpc_stats の探索順は次である。

1. `/proc/fs/lustre/{llite,mdc,osc}/*/`
2. `/sys/fs/lustre/{llite,mdc,osc}/*/`
3. `/sys/kernel/debug/lustre/{llite,mdc,osc}/*/`

同一 `component/name` は先に見た path を残す。

mdc/osc を `stats` で見つけたら、同じディレクトリの `rpc_stats` を sibling として `RpcStatsPath` に入れる。
第二 glob が無くても入れる。
default の FakeReader は `rpc_stats` glob を持たない。
sibling 代入を消すと契約テストが `lustre_rpcs_in_flight` を失う。

単一値ファイル（llite tunables、OSC dirty、active、state）の根は stats の親ディレクトリではない。
debugfs に stats だけがあり、tunables は sysfs または procfs にある、という配置を仕様として扱う。

`ClientTarget` に `ParamRoots` を持つ。

```text
ParamRoots = [
  /proc/fs/lustre/<component>/<name>,
  /sys/fs/lustre/<component>/<name>,
]
```

collector は各ファイルをこの順で読む。
debugfs は ParamRoots に入れない。
既存の `BasePath` は `ParamRoots[0]` と同一にして、古いテストを壊さない。

発見済みの `StatsPath` / `RpcStatsPath` の ReadFile 失敗は、strict ならエラー、そうでなければ warn して続行する。
ParamRoots 上の optional ファイル欠落は、strict でもエラーにしない。

`lctl get_param` へは切り替えない。

## client から見た切り分け

サーバ queue を持たない前提で、使える対応は次に限る。

| 観測 | 言えること |
|---|---|
| `health_check != 1` | 他より先に health を見る |
| `target_active==0` または `state` が FULL/IDLE 以外 | その target との接続が切れているか回復中 |
| llite write が多いが osc `ost_write` が少ない | cache または未 flush。RPC 未到達 |
| `lustre_osc_dirty_bytes / lustre_osc_max_dirty_bytes` が 1 に近い | writeback 上限。新規 write が stall しうる |
| `pending_pages` が高く OSC の `rpcs_current` が `max_rpcs_in_flight` に張り付く | client は RPC を出しているが相手または経路が追いついていない |
| MDC の `rpcs_current{operation="modify"}` が `max_mod_rpcs_in_flight` に張り付く | metadata RPC の同時実行上限 |
| `pending_pages` が高く `rpcs_current` が低い | import 状態、lock、または concurrency 設定を疑う |
| `req_waittime` 平均が上がり、LNet health が落ちる / timeout が増える | ネットワークまたは RDMA 側 |
| `req_waittime` 平均が上がり、LNet は正常で、特定 OSC だけ悪い | その OST またはその経路。server 側の確認が次 |
| `req_waittime` 平均が全 OSC で上がり、LNet も悪い | 共有 fabric |
| llite の I/O が小さく、`pages_per_rpc` も小さい | aggregation できていない |
| llite の I/O が小さく、`pages_per_rpc` が大きい | client 側でまとまっている |
| ldlm namespace の cancel/grant だけが高い | 次の P1。bandwidth ではなく lock churn の仮説 |

「throughput 低下かつ client wait 正常」なら、Lustre RPC より上（アプリ、llite cache、CPU）か、そもそも I/O が出ていない。

## 明示的にやらないこと

- server の recovery、exports、quota、jobstats、BRW、MDS/OSS service stats
- `lustre_stats_total` の label 変更
- stats 行から Prometheus Histogram を合成すること
- extents 収集の有効化
- PID 単位統計
- peer health の default ON
- GSI-HPC や whamcloud からのコード転記
- `--collector.ost` などサーバ collector flag の追加

## PR 分割

相談の「最初の 4 点をまとめて入れる」はレビュー単位として大きい。
collector も入力もテストも分かれるので 3 本に切る。

1. mdc/osc `stats` の count+sum と、`rpc_stats` 瞬間値。discovery の sysfs/debugfs 補完を含む。
2. dirty / RPC limit / active / state。
3. LNet local NI health。コマンド変更と auto 時の supplemental 取得。

P1（timeouts、statahead、ldlm namespace、extents opt-in）は実機 fixture のあとで別計画にする。

## 敵対的レビューで止揚した点

初稿の「debugfs の stats 親を BasePath にする」は捨てた。
stats の場所と tunables の場所を同じディレクトリとみなすと、discovery 拡張のあと llite / OSC 単一値が静かに消える。

初稿の「Task 9 で ParseLNetCtlNetStats を拡張し、Task 10 で count を落とすか関数を分けてもよい」は捨てた。
関数は最初から分ける。
`auto` の supplemental は NI extras だけを足す。
`debugfs` source は lnetctl を呼ばない。

初稿の「health が無い NI では count だけ」は、`status: up` がある現行 fixture と矛盾するので捨てた。
`lnet_ni_up` は status から出す。
health 系は `health stats` があるときだけ出す。

初稿は `rpc_stats` スカラー化のあと、既存 parser テストが通ると書いていた。
実際には inline fixture を持つ 4 本が `MetricID == rpcs_in_flight` と件数で落ちる。
計画はそれらのテスト更新を必須にする。

MDC の concurrency 上限を `max_rpcs_in_flight` と同一視する案は捨てた。
modify は `max_mod_rpcs_in_flight` を読む。

PR3 で `main.go` に peer-health flag を「任意で」足す分岐は捨てた。
PR3 は flag を足さない。

`auto` を「debugfs 成功後の supplemental だけ」と書く案は捨てた。
debugfs 失敗時の現行 `collectFromLNetCtl` を残さないと、既存の Auto fallback テスト 3 本が落ちる。

stats 発見の sibling `RpcStatsPath` を第二 glob に置き換える案は捨てた。
default FakeReader は `rpc_stats` glob を持たない。

`dropGlobalLNetCountStats` の空ラベル判定は捨てた。
落とす対象は MetricID の `send_count_total` / `receive_count_total` / `drop_count_total` である。
nid 付き count が無いときは落とさない。

`collectFromDebugFS` の成功を debugfs ファイル限定にする案は捨てた。
成功は `LNetStatsPaths` の `ReadFirstAvailable` である。
`/proc/sys/lnet/stats` が読めたら成功である。
