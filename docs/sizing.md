# Sizing and Capacity Model

[Back to the README](../README.md) · [Input reference](api.md#input-parameters) · [Apply results to the operator](operator.md)

## What the numbers mean

The calculator turns a connection count and a workload profile into resource sizes using a simple, uncalibrated heuristic. The utilization it reports is an estimate of resource pressure under that heuristic, not a measured or guaranteed capacity:

- `connections` is the number of client connections expected to be busy at the same time. Idle connections in a driver pool cost far less than the model assumes, and a single expensive query can cost far more. The generated `net.maxIncomingConnections` also uses this busy count; it does not explicitly account for idle driver-pool sockets. Before applying the output, raise that limit as needed to cover the peak number of open connections to each component, including idle sockets.
- The per-connection CPU and memory figures are fixed assumptions. They have not been calibrated against benchmarks.
- The model does not see working-set size, query shapes, index coverage, storage latency, replication lag, backup or balancer activity, or network limits. Any of these can saturate a deployment that the calculator reports as comfortable.

Use the output as a starting point, then validate it with a load test and monitoring (for example PMM) before production use.

## Pod budget and sidecars

A [dimension](api.md#supported-dimensions) is the total CPU and memory of one shard or replica-set `mongod` pod. The `mongod` container and its sidecars share that budget. `providercostpct` is taken off first, then the sidecar limits:

| Sidecar | When | CPU request / limit | Memory request / limit |
|---|---|---:|---:|
| PMM Client | Unless `mongodbdedicated` is `true` | `300m` / `400m` | `150M` / `256M` |
| PBM agent | When `backup.enabled` is `true` | `300m` / `300m` | `1Gi` / `1.2Gi` |

The operator has no built-in resource defaults for these sidecars. The values come from the commented recommendations in the operator's sample `deploy/cr.yaml`, and you must set them in your custom resource (see [Percona Operator Mapping](operator.md)). Whatever is left goes to `mongod`, rounded down to a whole MiB. If `mongod` would get less than `250m` CPU or `512Mi` memory, the request returns message type `3001` with an explanation.

The budget only covers `mongod`, PMM, and PBM. Other containers in the same pod are not modelled:

- **Log collector.** The operator's sample custom resource enables `logcollector`, which adds two containers, `logs` and `logrotate`, each with `logcollector.resources`. Either set `logcollector.enabled: false` or budget for both containers outside the dimension.
- **Custom sidecars** under `replsets[].sidecars` or `sharding.configsvrReplSet.sidecars`.

Init containers reuse the `mongod` resources and run before the other containers start, so they do not change the pod total.

Config-server and `mongos` pods are not part of the dimension. They are sized separately, and their sidecars are added on top of the sizes shown. The calculator does not multiply resources by a shard count or replica-set member count.

## Dedicated Mode

Set `mongodbdedicated` to `true` when PMM should be omitted from a replica-set pod. If backup is enabled, PBM is still emitted and still taken out of the pod budget, because it runs alongside `mongod`.

Dedicated mode:

- Gives `mongod` the whole dimension, minus PBM when backup is enabled.
- Omits the `monitor` family.
- Is valid only for `replica_set`. Dedicated `sharded_cluster` requests are rejected.
- Uses the same WiredTiger cache rule as every other mode.

See the [dedicated example](examples.md#complete-dedicated-example).

## Sizing Rules

### Shard and replica-set `mongod`

- Resources: the dimension minus `providercostpct` and the sidecar limits (see [Pod budget and sidecars](#pod-budget-and-sidecars)).
- WiredTiger cache: the operator always starts `mongod` with `--wiredTigerCacheSizeGB`, which overrides any `cacheSizeGB` in the configuration. The value is `max(0.25, floor(cacheSizeRatio × (memory limit − 1Gi)) / 1Gi)`, printed with two decimals, where `cacheSizeRatio` defaults to `0.5`. The calculator uses the same formula, emits `cacheSizeRatio` in the `storage` group, and reports the resulting `cacheSizeGB` so you can see what `mongod` will run with. Change the cache with `cacheSizeRatio`, not `cacheSizeGB`.
- `net.maxIncomingConnections`: `ceil(connections × 1.15) + 100`. The fixed `100` covers replica-set members, operator health checks, PMM, PBM, and admin sessions. In sharded clusters the shard also adds an assumed `10` pool connections per `mongos` (see [Connection pool assumptions](#connection-pool-assumptions)). Client connections still count in full, because every client operation can reach the same shard through `mongos`.

### Config server

A config server's load depends on cluster metadata (collections, chunks, migrations) and on how many `mongos` and shard members talk to it. The calculator only models the `mongos` side. It uses the smallest of these tiers that keeps estimated utilization at or below 85%:

| Tier | CPU | Memory |
|---:|---:|---:|
| 1 | `1000m` | `2Gi` |
| 2 | `2000m` | `4Gi` |
| 3 | `4000m` | `8Gi` |
| 4 | `8000m` | `16Gi` |

The config-server load is an assumed `20` connections per `mongos` with the Mainly Reads profile. `net.maxIncomingConnections` is `ceil(mongos.count × 20 × 1.15) + 100`.

The tier is a starting point, not a validated metadata capacity. Clusters with many sharded collections, many chunks, frequent migrations, or many shards may need a larger tier than the calculator picks.

### `mongos`

Each `mongos` handles `ceil(connections / mongos.count)` client connections, written as *n* below. It is sized so its estimated load sits at 70% of its limits:

- CPU: `(100m + n × CPU per connection × 0.5) / 0.70`, rounded up to `100m`, from `500m` to `16000m`.
- Memory: `(256Mi + n × (1Mi + working memory per connection × 0.5)) / 0.70`, rounded up to `64Mi`, from `512Mi` to `32Gi`.
- `net.maxIncomingConnections`: `ceil(n × 1.15) + 100`.
- No WiredTiger cache.

The split assumes clients spread evenly over every `mongos` and every `mongos` is up. It leaves no extra room for uneven client pools, a rolling restart, or a failed `mongos`. If you need to survive losing one `mongos`, calculate with `mongos.count` set to one less than you deploy.

If even the largest `mongos` cannot handle its share, the request is rejected with a hint to increase `mongos.count`.

### Connection pool assumptions

Each `mongos` keeps connection pools to every shard and config-server member. Their real size depends on `taskExecutorPoolSize` (by default one pool per CPU core), `ShardingTaskExecutorPoolMinSize`, `ShardingTaskExecutorPoolMaxSize`, and the workload. Shard members also open connections to the config servers. The calculator does not model any of this. It assumes `10` connections from each `mongos` to each shard member and `20` to each config-server member. These are calculator assumptions, not MongoDB defaults. If your pools are larger, raise `net.maxIncomingConnections` to match.

### Requests and limits

- By default, `mongod`, config-server, and `mongos` requests are 95% of their limits, rounded down to whole millicores and MiB. PMM and PBM use the fixed requests listed above.
- With `guaranteedqos: true`, every container the calculator emits, sidecars included, gets requests equal to limits. Kubernetes only gives a pod Guaranteed QoS if every container and init container in it has equal requests and limits. Disable or size the log collector and any custom sidecars the same way, or the pod falls back to Burstable.
- PMM memory is emitted in decimal bytes (`256000000`, the sample value of `256M`), and PBM memory limit is `1.2Gi` (`1288490188` bytes), which is not a whole MiB. Every other memory value is a whole number of MiB.

### Probes

Probe values are the runtime defaults that Percona Operator for MongoDB 1.23.0 applies when a probe is not set in the custom resource. They do not depend on connection count. Some of them differ from the commented examples in the operator's sample `deploy/cr.yaml`; the runtime defaults are what a pod gets.

| Container | Probe | initialDelaySeconds | timeoutSeconds | periodSeconds | failureThreshold | Other |
|---|---|---:|---:|---:|---:|---|
| Replica-set, shard, and config-server `mongod` | liveness | `60` | `10` | `30` | `4` | `startupDelaySeconds: 7200` |
| `mongos` | liveness | `60` | `10` | `30` | `4` | `startupDelaySeconds: 10` |
| Replica-set and shard `mongod` | readiness | `10` | `2` | `3` | `8` | `successThreshold: 1` |
| Config server | readiness | `10` | `2` | `3` | `3` | `successThreshold: 1` |
| `mongos` | readiness | `10` | `1` | `1` | `3` | `successThreshold: 1` |
| PMM Client | liveness | `60` | `5` | `10` | `3` | |

`startupDelaySeconds` is an operator extension, not a Kubernetes probe field. The operator passes it to its health-check command as `--startupDelaySeconds`.

PMM Client has no readiness probe by default. The PMM liveness probe sets no `failureThreshold`, so Kubernetes uses its default of `3`. Since 1.23.0 you can set `pmm.livenessProbe` and `pmm.readinessProbe` in the custom resource. They replace the whole probe, so include a handler such as `httpGet` on port `7777` path `/local/Status` for liveness, or `tcpSocket` on port `7777` for readiness.

## Capacity Model

Every component gets a CPU and a memory utilization estimate. The reported figure is the larger of the two. All coefficients are assumptions (see [What the numbers mean](#what-the-numbers-mean)), so treat the percentages as relative pressure, not as headroom you can rely on.

| Workload ID | Profile | CPU per connection | Working memory per connection |
|---:|---|---:|---:|
| `1` | Mainly Reads | `0.5m` | `0.25Mi` |
| `2` | Light OLTP | `1m` | `0.5Mi` |
| `3` | Heavy OLTP | `2m` | `1Mi` |
| `4` | Mainly Writes | `3m` | `1.5Mi` |

Every connection also counts `1Mi` for its thread stack.

- **CPU:** `100m + connections × CPU per connection`, divided by the CPU limit.
- **Memory:** the WiredTiger cache is an allocation, not load, so memory is measured against what is left outside the cache. The estimate is `128Mi + 5% of the memory limit + connections × (1Mi + working memory per connection)`, divided by `memory limit − cache`, where the cache is the operator-derived value described in [Sizing Rules](#shard-and-replica-set-mongod).
- **Idle pool connections:** in sharded clusters, the shard counts the assumed `10` pool connections per `mongos`. Each costs `1Mi` of stack memory but no CPU, so a high `mongos.count` can push a small shard towards its limit even with little client traffic.

The headline utilization is the shard or replica-set `mongod` estimate. `mongos` and the config server are sized by the calculator, so they only count toward the headline once even their largest size is above 85%. Sharded results list all three components in the message text.

| Utilization | Message type | HTTP status |
|---|---:|---:|
| Up to 85% | `1001` | `200` |
| Above 85%, up to 100% | `2001` | `200` |
| Above 100%, or `mongod` below its minimum | `3001` | `422` |

A `3001` response has an empty `answer` object. The message text names each overloaded component. The thresholds apply to the heuristic estimate; a `1001` result is not a guarantee that the workload fits.
