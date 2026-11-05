# MongoDB Operator Calculator

The MongoDB Operator Calculator is a Go library and HTTP service that produces starting-point resource and configuration values for Percona Server for MongoDB deployments managed by the Percona Operator for MongoDB.

It mirrors the public API shape of the MySQL Operator Calculator while using MongoDB-specific sizing rules.

## Scope

- Percona Server for MongoDB 7 and 8.
- Percona Operator for MongoDB 1.23.0. Probe values, the WiredTiger cache, and the custom resource mapping are checked against that release. Other operator releases may behave differently.
- Replica sets and sharded clusters.
- One `mongod` member per result; no replica-set member count is assumed.
- Per-component sharded output for one shard member, one config-server member, one `mongos`, and PMM.
- PMM Client and optional PBM agent sidecars, taken out of the pod budget.
- Kubernetes CPU and memory requests, limits, and probes, with optional Guaranteed QoS.
- JSON and operator-oriented human-readable output.
- No storage sizing, replication tuning, backup scheduling, security policy, or autoscaling.

### What the numbers mean

The calculator turns a connection count and a workload profile into resource sizes using a simple, uncalibrated heuristic. The utilization it reports is an estimate of resource pressure under that heuristic, not a measured or guaranteed capacity:

- `connections` is the number of client connections expected to be busy at the same time. Idle connections in a driver pool cost far less than the model assumes, and a single expensive query can cost far more.
- The per-connection CPU and memory figures are fixed assumptions. They have not been calibrated against benchmarks.
- The model does not see working-set size, query shapes, index coverage, storage latency, replication lag, backup or balancer activity, or network limits. Any of these can saturate a deployment that the calculator reports as comfortable.

Use the output as a starting point, then validate it with a load test and monitoring (for example PMM) before production use.

## Build and Run

```bash
git clone https://github.com/igroene/mongodboperatorcalculator.git
cd mongodboperatorcalculator
go build -o mongodboperatorcalculator ./src
./mongodboperatorcalculator
```

The default server address is `0.0.0.0:8080`.

```bash
./mongodboperatorcalculator -address=127.0.0.1 -port=8080 -loglevel=INFO
```

The server stops gracefully on `SIGINT` and `SIGTERM`, so it can run as a Kubernetes Deployment. Use `GET /healthz` for liveness and readiness probes.

## Command-Line Flags

| Flag | Default | Description |
|---|---:|---|
| `-address` | `0.0.0.0` | IP address or hostname to bind to. |
| `-port` | `8080` | TCP port to listen on. |
| `-loglevel` | `INFO` | Log level: `ERROR`, `INFO`, or `DEBUG`. Unknown values fall back to `INFO`. `DEBUG` logs every calculation result. |
| `-version` | `false` | Print the calculator version and exit. |
| `-help` | `false` | Print command-line usage and exit. The standard Go `flag` package also accepts `-h`. |

Examples:

```bash
./mongodboperatorcalculator -help
./mongodboperatorcalculator -version
./mongodboperatorcalculator -address=127.0.0.1 -port=9090 -loglevel=DEBUG
```

## Input Parameters

All calculator inputs are passed as a JSON request to `POST /calculator` or through the Go module API. The HTTP API rejects unknown fields and trailing data after the JSON object.

| Parameter | Type | Required | Description |
|---|---|---:|---|
| `output` | string | No | `json` (default) or `human`. Any other value is rejected. |
| `dbtype` | string | Yes | `replica_set` or `sharded_cluster`. |
| `dimension.id` | integer | Yes | Predefined dimension ID `1` through `10`, or `999` for explicit resources. Dimension `998` is not supported. |
| `dimension.cpu` | integer | Conditional | Total pod CPU in millicores for dimension `999`, from `1` to `512000`. For predefined dimensions it may be omitted or must match the catalog value. |
| `dimension.memory` | string | Conditional | Total pod memory for dimension `999`, for example `8Gi`, `8G`, `4096Mi`, or `8589934592`. For predefined dimensions it may be omitted or must match the catalog value. |
| `loadtype.id` | integer | Yes | Workload profile `1` through `4`. |
| `connections` | integer | Yes | Client connections expected to be busy at the same time, from `1` to `1000000`. For sharded clusters this is the total across all `mongos`. |
| `mongodbversion.major` | integer | Yes | Supported major version: `7` or `8`. |
| `mongodbversion.minor` | integer | Yes | MongoDB minor version, `0` to `99`. |
| `mongodbversion.patch` | integer | Yes | MongoDB patch version, `0` to `99`. |
| `providercostpct` | float | No | Fraction of the dimension reserved for platform overhead. For example, `0.12` reserves 12%. Default is `0`. Must be at least `0` and less than `1`. |
| `mongodbdedicated` | boolean | No | When `true`, omit PMM so `mongod` gets the whole pod budget except PBM. Valid only for `replica_set`. Default is `false`. |
| `backup.enabled` | boolean | No | When `true`, add a PBM agent sidecar to shard and config-server `mongod` pods. Default is `false`. |
| `guaranteedqos` | boolean | No | When `true`, every container the calculator emits gets requests equal to limits. See [Requests and limits](#requests-and-limits) for what else Guaranteed QoS needs. Default is `false`. |
| `mongos.count` | integer | No | Number of `mongos` pods sharing the client connections, `1` to `100`. Valid only for `sharded_cluster`. Default is `3`, the size in the operator's sample custom resource. |

The MongoDB version is checked against the accepted range (`7.0.0` to `8.99.99`). The check is numeric only: it does not mean every release in that range has been tested. The version does not change the generated values yet.

### Memory Quantities

Memory uses Kubernetes-style suffixes, with a few differences: suffixes are case-insensitive, `KiB`-style and `KB`-style aliases are accepted, and exponents (`1e9`) are rejected. A lowercase `m` on its own is rejected because Kubernetes reads it as milli.

| Suffix | Meaning |
|---|---|
| none, `B` | Bytes |
| `Ki`, `Mi`, `Gi`, `Ti`, `Pi` (also `KiB`, `MiB`, `GiB`, `TiB`, `PiB`) | Binary: powers of 1024 |
| `k`, `M`, `G`, `T`, `P` (also `KB`, `MB`, `GB`, `TB`, `PB`) | Decimal: powers of 1000 |

So `8Gi` is 8,589,934,592 bytes and `8G` or `8GB` is 8,000,000,000 bytes. Exponents, hexadecimal, negative values, `NaN`, and `Inf` are rejected. Memory must be greater than zero and at most `16Ti`.

Requests that parse but fail validation return message type `5001` and HTTP status `400`. See [Errors](#errors) for the full error format.

## Supported Dimensions

A dimension is the total CPU and memory of one shard or replica-set `mongod` pod. The `mongod` container and its sidecars share that budget. `providercostpct` is taken off first, then the sidecar limits:

| Sidecar | When | CPU request / limit | Memory request / limit |
|---|---|---:|---:|
| PMM Client | Unless `mongodbdedicated` is `true` | `300m` / `400m` | `150M` / `256M` |
| PBM agent | When `backup.enabled` is `true` | `300m` / `300m` | `1Gi` / `1.2Gi` |

The operator has no built-in resource defaults for these sidecars. The values come from the commented recommendations in the operator's sample `deploy/cr.yaml`, and you must set them in your custom resource (see [Percona Operator Mapping](#percona-operator-mapping)). Whatever is left goes to `mongod`, rounded down to a whole MiB. If `mongod` would get less than `250m` CPU or `512Mi` memory, the request returns message type `3001` with an explanation.

The budget only covers `mongod`, PMM, and PBM. Other containers in the same pod are not modelled:

- **Log collector.** The operator's sample custom resource enables `logcollector`, which adds two containers, `logs` and `logrotate`, each with `logcollector.resources`. Either set `logcollector.enabled: false` or budget for both containers outside the dimension.
- **Custom sidecars** under `replsets[].sidecars` or `sharding.configsvrReplSet.sidecars`.

Init containers reuse the `mongod` resources and run before the other containers start, so they do not change the pod total.

| ID | Name | Total CPU | Total Memory | Notes |
|---:|---|---:|---:|---|
| `1` | XSmall | `600m` | `1Gi` | Only fits with `mongodbdedicated` and no backup. |
| `2` | Small | `800m` | `2Gi` | Fits PMM or PBM, not both. |
| `3` | Medium | `1000m` | `4Gi` | |
| `4` | Large | `2000m` | `8Gi` | |
| `5` | XLarge | `4000m` | `16Gi` | |
| `6` | 2XLarge | `8000m` | `32Gi` | |
| `7` | 4XLarge | `16000m` | `64Gi` | |
| `8` | 8XLarge | `32000m` | `128Gi` | |
| `9` | 12XLarge | `64000m` | `256Gi` | |
| `10` | 16XLarge | `128000m` | `512Gi` | |
| `999` | Open request by resources | User supplied | User supplied | Same pod budget rules. |

The catalog includes small development profiles below MongoDB's general production recommendation of two real CPU cores. Use larger profiles for production workloads.

Config-server and `mongos` pods are not part of the dimension. They are sized separately, and their sidecars are added on top of the sizes shown.

## Workload Types

| ID | Name | Description | CPU per connection | Working memory per connection |
|---:|---|---|---:|---:|
| `1` | Mainly Reads | Read-heavy workload. | `0.5m` | `0.25Mi` |
| `2` | Light OLTP | Mixed workload with moderate writes. | `1m` | `0.5Mi` |
| `3` | Heavy OLTP | Highly concurrent mixed workload. | `2m` | `1Mi` |
| `4` | Mainly Writes | Write-heavy workload. | `3m` | `1.5Mi` |

Every connection also counts `1Mi` for its thread stack. Workload profiles affect the CPU and memory estimates. They do not change replication, read concern, write concern, authentication, TLS, or storage settings.

## Dedicated Mode

Set `mongodbdedicated` to `true` when PMM should be omitted from a replica-set pod. If backup is enabled, PBM is still emitted and still taken out of the pod budget, because it runs alongside `mongod`.

Dedicated mode:

- Gives `mongod` the whole dimension, minus PBM when backup is enabled.
- Omits the `monitor` family.
- Is valid only for `replica_set`. Dedicated `sharded_cluster` requests are rejected.
- Uses the same WiredTiger cache rule as every other mode.

## Sizing Rules

### Shard and replica-set `mongod`

- Resources: the dimension minus `providercostpct` and the sidecar limits (see [Supported Dimensions](#supported-dimensions)).
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

## Output Structure

JSON output has three top-level fields:

```json
{
  "message": {},
  "incoming": {},
  "answer": {}
}
```

### `message`

Diagnostic status information:

| Code | Meaning |
|---:|---|
| `1001` | Estimated utilization is at or below 85%. |
| `2001` | Estimated utilization is above 85% and at or below 100%. |
| `3001` | Estimated utilization is above 100%, or `mongod` does not fit in the dimension; `answer` is empty. |
| `5001` | The request parsed but is invalid or unsupported. |
| `6001` | Compatibility code from the MySQL calculator; not emitted. |
| `7001` | Compatibility code from the MySQL calculator; not emitted. |

### `incoming`

The resolved request. On success or overload, `dimension` contains the requested totals (`cpu`, `memory`) plus the calculated allocation:

| Field | Meaning |
|---|---|
| `mongodbCpu`, `mongodbMemory` | Shard or replica-set `mongod` container limits. |
| `monitorCpu`, `monitorMemory` | PMM Client limits taken out of the `mongod` pod. `0` in dedicated mode. |
| `backupCpu`, `backupMemory` | PBM agent limits taken out of the `mongod` pod. `0` without backup. |
| `configCpu`, `configMemory` | Config-server `mongod` container limits (separate pod). |
| `mongosCpu`, `mongosMemory` | `mongos` container limits (separate pod). |

Memory allocations are bytes and CPU allocations are millicores. For `5001` responses, `incoming` holds the decoded request without defaults or calculated values (see [Errors](#errors)).

### `answer`

Calculated configuration families. Each family contains groups, and each group contains parameters with `name` and `value` fields.

| Family | Meaning |
|---|---|
| `mongodb` | One shard or replica-set `mongod` member. |
| `configserver` | One config-server `mongod` member in a sharded cluster. |
| `mongos` | One `mongos` query router in a sharded cluster. |
| `monitor` | PMM Client sidecar resources and probes. In sharded clusters, groups are prefixed with `mongod.`, `configserver.`, or `mongos.`. |

| Group | Meaning |
|---|---|
| `configuration` | MongoDB configuration values. `storage.wiredTiger.engineConfig.cacheSizeGB` is informational: it is the value the operator passes to `mongod`, and setting it in the configuration has no effect. |
| `storage` | Custom resource storage settings (`replsets[].storage` or `sharding.configsvrReplSet.storage`): `engine` and `wiredTiger.engineConfig.cacheSizeRatio`. Emitted under `mongodb` and `configserver`. |
| `resources` | Kubernetes CPU and memory requests and limits. |
| `readinessProbe` | Readiness probe settings. Not emitted for PMM. |
| `livenessProbe` | Liveness probe settings. |
| `pbm-agent.resources` | PBM agent requests and limits when `backup.enabled` is `true`; emitted under `mongodb` and `configserver`. |

Output units are parameter-specific: resource memory values are bytes, CPU values are Kubernetes millicores, WiredTiger cache is binary GB, and connection limits are integers.

## HTTP API

### `GET /supported`

Returns supported database types, dimensions, workload types, connection presets, output formats, and MongoDB version range. Allocation fields in the dimension list are `0`, because they depend on the request.

```bash
curl -i http://127.0.0.1:8080/supported
```

Only `GET` is supported. Other methods return HTTP `405` and `Allow: GET`.

### `GET /healthz`

Returns `{"status":"ok","version":"<version>"}` with HTTP `200`. Only `GET` is supported.

### `POST /calculator`

Accepts a JSON request body of at most 1 MiB and returns JSON by default, or operator-oriented text when `output` is `human`.

Only `POST` is supported. Other methods return HTTP `405` and `Allow: POST`.

HTTP statuses:

| Status | Meaning |
|---:|---|
| `200` | Successful or close-to-limit calculation. |
| `400` | Malformed JSON, unknown fields, or invalid request. |
| `405` | Unsupported HTTP method. |
| `413` | Request body larger than 1 MiB. |
| `422` | Valid request but insufficient resources for the requested workload. |

The server sets read, write, and idle timeouts, so slow clients cannot hold connections open.

### Errors

There are two error formats:

- **The body could not be decoded** (empty body, malformed JSON, unknown fields, trailing data, or a body over 1 MiB). The response is a plain JSON object with a single `error` field, HTTP `400` or `413`, and no `message`, `incoming`, or `answer`:

  ```json
  {"error": "malformed JSON: json: unknown field \"shards\""}
  ```

- **The body decoded but failed validation** (unsupported values, out-of-range numbers, an unparsable memory quantity). The response uses the normal structure with message type `5001`, HTTP `400`, and an empty `answer`. `incoming` holds the decoded request as it was before any defaults were applied: fields you left out appear with their zero values, and nothing is calculated. See the [Invalid Request Example](#invalid-request-example).

Other methods return HTTP `405` with the same `error` object.

## Complete Replica-Set Example

Request:

<!-- request:replica-set -->
```json
{
  "output": "json",
  "dbtype": "replica_set",
  "dimension": {"id": 4},
  "loadtype": {"id": 2},
  "connections": 500,
  "mongodbversion": {"major": 7, "minor": 0, "patch": 0}
}
```

```bash
curl -X POST -H 'Content-Type: application/json' \
  -d @replica-request.json \
  http://127.0.0.1:8080/calculator
```

Expected output:

<!-- response:replica-set -->
```json
{
  "message": {
    "type": 1001,
    "name": "Execution was successful and resources match the requested workload",
    "text": "estimated capacity utilization is 38% (CPU 38%, memory 28%)"
  },
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {
      "id": 4,
      "name": "Large",
      "cpu": 2000,
      "memory": "8Gi",
      "mongodbCpu": 1600,
      "monitorCpu": 400,
      "backupCpu": 0,
      "mongosCpu": 0,
      "configCpu": 0,
      "mongodbMemory": 8333033472,
      "monitorMemory": 256000000,
      "backupMemory": 0,
      "mongosMemory": 0,
      "configMemory": 0
    },
    "loadtype": {
      "id": 2,
      "name": "Light OLTP",
      "example": "Mixed workload with moderate writes"
    },
    "connections": 500,
    "output": "json",
    "mongodbversion": {
      "major": 7,
      "minor": 0,
      "patch": 0
    },
    "providercostpct": 0,
    "mongodbdedicated": false,
    "guaranteedqos": false,
    "backup": {
      "enabled": false
    },
    "mongos": {
      "count": 0
    }
  },
  "answer": {
    "mongodb": {
      "name": "mongod",
      "groups": {
        "configuration": {
          "name": "configuration",
          "parameters": {
            "net.maxIncomingConnections": {
              "name": "net.maxIncomingConnections",
              "value": "675"
            },
            "storage.engine": {
              "name": "storage.engine",
              "value": "wiredTiger"
            },
            "storage.wiredTiger.engineConfig.cacheSizeGB": {
              "name": "storage.wiredTiger.engineConfig.cacheSizeGB",
              "value": "3.38"
            }
          }
        },
        "livenessProbe": {
          "name": "livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "4"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "30"
            },
            "startupDelaySeconds": {
              "name": "startupDelaySeconds",
              "value": "7200"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "10"
            }
          }
        },
        "readinessProbe": {
          "name": "readinessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "8"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "10"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "3"
            },
            "successThreshold": {
              "name": "successThreshold",
              "value": "1"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "2"
            }
          }
        },
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "1600m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "8333033472"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "1520m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "7915700224"
            }
          }
        },
        "storage": {
          "name": "storage",
          "parameters": {
            "engine": {
              "name": "engine",
              "value": "wiredTiger"
            },
            "wiredTiger.engineConfig.cacheSizeRatio": {
              "name": "wiredTiger.engineConfig.cacheSizeRatio",
              "value": "0.5"
            }
          }
        }
      }
    },
    "monitor": {
      "name": "pmm-client",
      "groups": {
        "livenessProbe": {
          "name": "livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "3"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "10"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "5"
            }
          }
        },
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "400m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "256000000"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "300m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "150000000"
            }
          }
        }
      }
    }
  }
}
```

## Complete Sharded-Cluster Example

Request:

<!-- request:sharded-cluster -->
```json
{
  "output": "json",
  "dbtype": "sharded_cluster",
  "dimension": {"id": 5},
  "loadtype": {"id": 2},
  "connections": 1500,
  "mongos": {"count": 3},
  "backup": {"enabled": true},
  "mongodbversion": {"major": 8, "minor": 0, "patch": 0}
}
```

Expected output. Every component is an independent pod; no shard count or replica-set member count is assumed:

<!-- response:sharded-cluster -->
```json
{
  "message": {
    "type": 1001,
    "name": "Execution was successful and resources match the requested workload",
    "text": "estimated capacity utilization is 48% (shard mongod 48%, config server 20%, mongos 69%)"
  },
  "incoming": {
    "dbtype": "sharded_cluster",
    "dimension": {
      "id": 5,
      "name": "XLarge",
      "cpu": 4000,
      "memory": "16Gi",
      "mongodbCpu": 3300,
      "monitorCpu": 400,
      "backupCpu": 300,
      "mongosCpu": 600,
      "configCpu": 1000,
      "mongodbMemory": 15635316736,
      "monitorMemory": 256000000,
      "backupMemory": 1288490188,
      "mongosMemory": 1342177280,
      "configMemory": 2147483648
    },
    "loadtype": {
      "id": 2,
      "name": "Light OLTP",
      "example": "Mixed workload with moderate writes"
    },
    "connections": 1500,
    "output": "json",
    "mongodbversion": {
      "major": 8,
      "minor": 0,
      "patch": 0
    },
    "providercostpct": 0,
    "mongodbdedicated": false,
    "guaranteedqos": false,
    "backup": {
      "enabled": true
    },
    "mongos": {
      "count": 3
    }
  },
  "answer": {
    "configserver": {
      "name": "config-server",
      "groups": {
        "configuration": {
          "name": "configuration",
          "parameters": {
            "net.maxIncomingConnections": {
              "name": "net.maxIncomingConnections",
              "value": "169"
            },
            "storage.engine": {
              "name": "storage.engine",
              "value": "wiredTiger"
            },
            "storage.wiredTiger.engineConfig.cacheSizeGB": {
              "name": "storage.wiredTiger.engineConfig.cacheSizeGB",
              "value": "0.50"
            }
          }
        },
        "livenessProbe": {
          "name": "livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "4"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "30"
            },
            "startupDelaySeconds": {
              "name": "startupDelaySeconds",
              "value": "7200"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "10"
            }
          }
        },
        "pbm-agent.resources": {
          "name": "pbm-agent.resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "300m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "1288490188"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "300m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "1073741824"
            }
          }
        },
        "readinessProbe": {
          "name": "readinessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "3"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "10"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "3"
            },
            "successThreshold": {
              "name": "successThreshold",
              "value": "1"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "2"
            }
          }
        },
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "1000m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "2147483648"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "950m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "2039480320"
            }
          }
        },
        "storage": {
          "name": "storage",
          "parameters": {
            "engine": {
              "name": "engine",
              "value": "wiredTiger"
            },
            "wiredTiger.engineConfig.cacheSizeRatio": {
              "name": "wiredTiger.engineConfig.cacheSizeRatio",
              "value": "0.5"
            }
          }
        }
      }
    },
    "mongodb": {
      "name": "mongod",
      "groups": {
        "configuration": {
          "name": "configuration",
          "parameters": {
            "net.maxIncomingConnections": {
              "name": "net.maxIncomingConnections",
              "value": "1855"
            },
            "storage.engine": {
              "name": "storage.engine",
              "value": "wiredTiger"
            },
            "storage.wiredTiger.engineConfig.cacheSizeGB": {
              "name": "storage.wiredTiger.engineConfig.cacheSizeGB",
              "value": "6.78"
            }
          }
        },
        "livenessProbe": {
          "name": "livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "4"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "30"
            },
            "startupDelaySeconds": {
              "name": "startupDelaySeconds",
              "value": "7200"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "10"
            }
          }
        },
        "pbm-agent.resources": {
          "name": "pbm-agent.resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "300m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "1288490188"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "300m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "1073741824"
            }
          }
        },
        "readinessProbe": {
          "name": "readinessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "8"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "10"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "3"
            },
            "successThreshold": {
              "name": "successThreshold",
              "value": "1"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "2"
            }
          }
        },
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "3300m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "15635316736"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "3135m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "14853079040"
            }
          }
        },
        "storage": {
          "name": "storage",
          "parameters": {
            "engine": {
              "name": "engine",
              "value": "wiredTiger"
            },
            "wiredTiger.engineConfig.cacheSizeRatio": {
              "name": "wiredTiger.engineConfig.cacheSizeRatio",
              "value": "0.5"
            }
          }
        }
      }
    },
    "mongos": {
      "name": "mongos",
      "groups": {
        "configuration": {
          "name": "configuration",
          "parameters": {
            "net.maxIncomingConnections": {
              "name": "net.maxIncomingConnections",
              "value": "675"
            }
          }
        },
        "livenessProbe": {
          "name": "livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "4"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "30"
            },
            "startupDelaySeconds": {
              "name": "startupDelaySeconds",
              "value": "10"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "10"
            }
          }
        },
        "readinessProbe": {
          "name": "readinessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "3"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "10"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "1"
            },
            "successThreshold": {
              "name": "successThreshold",
              "value": "1"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "1"
            }
          }
        },
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "600m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "1342177280"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "570m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "1275068416"
            }
          }
        }
      }
    },
    "monitor": {
      "name": "pmm-client",
      "groups": {
        "configserver.livenessProbe": {
          "name": "configserver.livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "3"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "10"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "5"
            }
          }
        },
        "configserver.resources": {
          "name": "configserver.resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "400m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "256000000"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "300m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "150000000"
            }
          }
        },
        "mongod.livenessProbe": {
          "name": "mongod.livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "3"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "10"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "5"
            }
          }
        },
        "mongod.resources": {
          "name": "mongod.resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "400m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "256000000"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "300m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "150000000"
            }
          }
        },
        "mongos.livenessProbe": {
          "name": "mongos.livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "3"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "10"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "5"
            }
          }
        },
        "mongos.resources": {
          "name": "mongos.resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "400m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "256000000"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "300m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "150000000"
            }
          }
        }
      }
    }
  }
}
```

Every value above is per component. The calculator does not multiply the shard `mongod` values by a shard count.

## Complete Dedicated Example

Request:

<!-- request:dedicated -->
```json
{
  "output": "json",
  "dbtype": "replica_set",
  "dimension": {"id": 2},
  "loadtype": {"id": 1},
  "connections": 100,
  "mongodbversion": {"major": 7, "minor": 0, "patch": 0},
  "mongodbdedicated": true
}
```

Expected output:

<!-- response:dedicated -->
```json
{
  "message": {
    "type": 1001,
    "name": "Execution was successful and resources match the requested workload",
    "text": "estimated capacity utilization is 23% (CPU 19%, memory 23%)"
  },
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {
      "id": 2,
      "name": "Small",
      "cpu": 800,
      "memory": "2Gi",
      "mongodbCpu": 800,
      "monitorCpu": 0,
      "backupCpu": 0,
      "mongosCpu": 0,
      "configCpu": 0,
      "mongodbMemory": 2147483648,
      "monitorMemory": 0,
      "backupMemory": 0,
      "mongosMemory": 0,
      "configMemory": 0
    },
    "loadtype": {
      "id": 1,
      "name": "Mainly Reads",
      "example": "Read-heavy workload"
    },
    "connections": 100,
    "output": "json",
    "mongodbversion": {
      "major": 7,
      "minor": 0,
      "patch": 0
    },
    "providercostpct": 0,
    "mongodbdedicated": true,
    "guaranteedqos": false,
    "backup": {
      "enabled": false
    },
    "mongos": {
      "count": 0
    }
  },
  "answer": {
    "mongodb": {
      "name": "mongod",
      "groups": {
        "configuration": {
          "name": "configuration",
          "parameters": {
            "net.maxIncomingConnections": {
              "name": "net.maxIncomingConnections",
              "value": "215"
            },
            "storage.engine": {
              "name": "storage.engine",
              "value": "wiredTiger"
            },
            "storage.wiredTiger.engineConfig.cacheSizeGB": {
              "name": "storage.wiredTiger.engineConfig.cacheSizeGB",
              "value": "0.50"
            }
          }
        },
        "livenessProbe": {
          "name": "livenessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "4"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "60"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "30"
            },
            "startupDelaySeconds": {
              "name": "startupDelaySeconds",
              "value": "7200"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "10"
            }
          }
        },
        "readinessProbe": {
          "name": "readinessProbe",
          "parameters": {
            "failureThreshold": {
              "name": "failureThreshold",
              "value": "8"
            },
            "initialDelaySeconds": {
              "name": "initialDelaySeconds",
              "value": "10"
            },
            "periodSeconds": {
              "name": "periodSeconds",
              "value": "3"
            },
            "successThreshold": {
              "name": "successThreshold",
              "value": "1"
            },
            "timeoutSeconds": {
              "name": "timeoutSeconds",
              "value": "2"
            }
          }
        },
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {
              "name": "limit_cpu",
              "value": "800m"
            },
            "limit_memory": {
              "name": "limit_memory",
              "value": "2147483648"
            },
            "request_cpu": {
              "name": "request_cpu",
              "value": "760m"
            },
            "request_memory": {
              "name": "request_memory",
              "value": "2039480320"
            }
          }
        },
        "storage": {
          "name": "storage",
          "parameters": {
            "engine": {
              "name": "engine",
              "value": "wiredTiger"
            },
            "wiredTiger.engineConfig.cacheSizeRatio": {
              "name": "wiredTiger.engineConfig.cacheSizeRatio",
              "value": "0.5"
            }
          }
        }
      }
    }
  }
}
```

The `monitor`, `mongos`, and `configserver` families are intentionally absent.

## Overload Example

<!-- request:overload -->
```json
{
  "output": "json",
  "dbtype": "replica_set",
  "dimension": {"id": 2},
  "loadtype": {"id": 4},
  "connections": 10000,
  "mongodbversion": {"major": 7, "minor": 0, "patch": 0}
}
```

The response has HTTP status `422`:

<!-- response:overload status=422 -->
```json
{
  "message": {
    "type": 3001,
    "name": "Resources are not enough to cover the requested workload",
    "text": "estimated capacity exceeded: mongod at 7525% (CPU 7525%, memory 1784%); choose a larger dimension or reduce connections"
  },
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {
      "id": 2,
      "name": "Small",
      "cpu": 800,
      "memory": "2Gi",
      "mongodbCpu": 400,
      "monitorCpu": 400,
      "backupCpu": 0,
      "mongosCpu": 0,
      "configCpu": 0,
      "mongodbMemory": 1890582528,
      "monitorMemory": 256000000,
      "backupMemory": 0,
      "mongosMemory": 0,
      "configMemory": 0
    },
    "loadtype": {
      "id": 4,
      "name": "Mainly Writes",
      "example": "Write-heavy workload"
    },
    "connections": 10000,
    "output": "json",
    "mongodbversion": {
      "major": 7,
      "minor": 0,
      "patch": 0
    },
    "providercostpct": 0,
    "mongodbdedicated": false,
    "guaranteedqos": false,
    "backup": {
      "enabled": false
    },
    "mongos": {
      "count": 0
    }
  },
  "answer": {}
}
```

## Dimension Too Small Example

XSmall cannot hold `mongod` plus the PMM sidecar:

<!-- request:too-small -->
```json
{
  "dbtype": "replica_set",
  "dimension": {"id": 1},
  "loadtype": {"id": 1},
  "connections": 10,
  "mongodbversion": {"major": 8, "minor": 0, "patch": 0}
}
```

The response has HTTP status `422`:

<!-- response:too-small status=422 -->
```json
{
  "message": {
    "type": 3001,
    "name": "Resources are not enough to cover the requested workload",
    "text": "dimension XSmall (600m/1Gi) leaves 200m CPU and 779Mi memory for mongod after PMM 400m/244Mi; the minimum is 250m and 512Mi; choose a larger dimension or set mongodbdedicated"
  },
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {
      "id": 1,
      "name": "XSmall",
      "cpu": 600,
      "memory": "1Gi",
      "mongodbCpu": 200,
      "monitorCpu": 400,
      "backupCpu": 0,
      "mongosCpu": 0,
      "configCpu": 0,
      "mongodbMemory": 816840704,
      "monitorMemory": 256000000,
      "backupMemory": 0,
      "mongosMemory": 0,
      "configMemory": 0
    },
    "loadtype": {
      "id": 1,
      "name": "Mainly Reads",
      "example": "Read-heavy workload"
    },
    "connections": 10,
    "output": "json",
    "mongodbversion": {
      "major": 8,
      "minor": 0,
      "patch": 0
    },
    "providercostpct": 0,
    "mongodbdedicated": false,
    "guaranteedqos": false,
    "backup": {
      "enabled": false
    },
    "mongos": {
      "count": 0
    }
  },
  "answer": {}
}
```

## Invalid Request Example

A request that decodes but fails validation. `mongodbversion` is missing, so it is echoed with zero values:

<!-- request:invalid -->
```json
{
  "dbtype": "replica_set",
  "dimension": {"id": 4},
  "loadtype": {"id": 2},
  "connections": 100
}
```

The response has HTTP status `400`:

<!-- response:invalid status=400 -->
```json
{
  "message": {
    "type": 5001,
    "name": "Invalid request",
    "text": "unsupported MongoDB version 0.0.0; supported range is 7.0.0 to 8.99.99"
  },
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {
      "id": 4,
      "name": "",
      "cpu": 0,
      "memory": "",
      "mongodbCpu": 0,
      "monitorCpu": 0,
      "backupCpu": 0,
      "mongosCpu": 0,
      "configCpu": 0,
      "mongodbMemory": 0,
      "monitorMemory": 0,
      "backupMemory": 0,
      "mongosMemory": 0,
      "configMemory": 0
    },
    "loadtype": {
      "id": 2,
      "name": "",
      "example": ""
    },
    "connections": 100,
    "output": "",
    "mongodbversion": {
      "major": 0,
      "minor": 0,
      "patch": 0
    },
    "providercostpct": 0,
    "mongodbdedicated": false,
    "guaranteedqos": false,
    "backup": {
      "enabled": false
    },
    "mongos": {
      "count": 0
    }
  },
  "answer": {}
}
```

## Operator-Oriented Human Output

Set `output` to `human` to receive sections suitable for mapping into a Percona Operator custom resource. Empty sections are omitted.

<!-- request:human -->
```json
{
  "output": "human",
  "dbtype": "replica_set",
  "dimension": {"id": 4},
  "loadtype": {"id": 2},
  "connections": 500,
  "backup": {"enabled": true},
  "mongodbversion": {"major": 8, "minor": 0, "patch": 0}
}
```

<!-- response:human -->
```ini
[message]
name = Execution was successful and resources match the requested workload
type = 1001
text = estimated capacity utilization is 46% (CPU 46%, memory 31%)

[mongod.configuration]
net.maxIncomingConnections = 675
storage.engine = wiredTiger
storage.wiredTiger.engineConfig.cacheSizeGB = 2.78

[mongod.livenessProbe]
failureThreshold = 4
initialDelaySeconds = 60
periodSeconds = 30
startupDelaySeconds = 7200
timeoutSeconds = 10

[mongod.pbm-agent.resources]
limit_cpu = 300m
limit_memory = 1288490188
request_cpu = 300m
request_memory = 1073741824

[mongod.readinessProbe]
failureThreshold = 8
initialDelaySeconds = 10
periodSeconds = 3
successThreshold = 1
timeoutSeconds = 2

[mongod.resources]
limit_cpu = 1300m
limit_memory = 7045382144
request_cpu = 1235m
request_memory = 6693060608

[mongod.storage]
engine = wiredTiger
wiredTiger.engineConfig.cacheSizeRatio = 0.5

[pmm-client.livenessProbe]
failureThreshold = 3
initialDelaySeconds = 60
periodSeconds = 10
timeoutSeconds = 5

[pmm-client.resources]
limit_cpu = 400m
limit_memory = 256000000
request_cpu = 300m
request_memory = 150000000

```

Sharded output uses `mongod`, `configserver.mongod`, `mongos`, and component-specific PMM sections such as `pmm-client.mongod.resources`, `pmm-client.configserver.resources`, and `pmm-client.mongos.resources`. Shard and config-server sections include `storage`. When backup is enabled, it also includes `mongod.pbm-agent.resources` and `configserver.mongod.pbm-agent.resources`. Values are sorted deterministically.

## Using as a Go Module

Import the package:

```go
import (
    "encoding/json"
    "fmt"
    "log"

    MO "github.com/igroene/mongodboperatorcalculator/src/mongodboperatorcalculator"
)
```

Fetch supported metadata:

```go
var conf MO.Configuration
conf.Init()

var calculator MO.MongoDBOperatorCalculator
supported := calculator.GetSupportedLayouts()
metadata, err := json.MarshalIndent(supported, "", "  ")
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(metadata))
```

Calculate a replica-set configuration:

```go
request := MO.ConfigurationRequest{
    Output:      MO.ResultOutputFormatJson,
    DBType:      MO.DbTypeReplicaSet,
    Dimension:   MO.Dimension{Id: 4},
    LoadType:    MO.LoadType{Id: MO.LoadTypeSomeWrites},
    Connections: 500,
    MongoDBVersion: MO.Version{Major: 7, Minor: 0, Patch: 0},
}

request = calculator.Init(request, conf)
calculationErr, message, families := calculator.GetCalculate()
if calculationErr != nil {
    log.Printf("calculation failed: %v", calculationErr)
}

output, err := calculator.GetJSONOutput(message, request, families)
if err != nil {
    log.Fatal(err)
}
fmt.Println(output.String())
```

Calculate a custom dimension. `Memory` is parsed by `Init`; you can also set `MemoryBytes` directly and leave `Memory` empty:

```go
request.Dimension = MO.Dimension{
    Id:     MO.DimensionOpen,
    Cpu:    4000,
    Memory: "8Gi",
}
request = calculator.Init(request, conf)
```

Access a calculated family and parameter:

```go
mongodbFamily, err := calculator.GetFamily(MO.FamilyTypeMongoDB)
if err != nil {
    log.Fatal(err)
}

storage := mongodbFamily.Groups[MO.GroupNameStorage].Parameters
fmt.Println(storage["wiredTiger.engineConfig.cacheSizeRatio"].Value)

// The cache size the operator will pass to mongod (informational).
config := mongodbFamily.Groups[MO.GroupNameConfiguration].Parameters
fmt.Println(config["storage.wiredTiger.engineConfig.cacheSizeGB"].Value)
```

The public `Init()` signature is intentionally compatible with the MySQL calculator. Validation and calculation errors are returned by `GetCalculate()`. Unlike the HTTP API, the Go API cannot reject unknown fields; everything else is validated the same way.

## Percona Operator Mapping

Mapping verified against Percona Operator for MongoDB 1.23.0.

| Calculator group | Custom resource field |
|---|---|
| `mongodb` → `resources` | `replsets[].resources` |
| `mongodb` → `livenessProbe`, `readinessProbe` | `replsets[].livenessProbe`, `replsets[].readinessProbe` |
| `mongodb` → `configuration` | `replsets[].configuration` (a YAML string). Copy `net.maxIncomingConnections`; leave `cacheSizeGB` out. |
| `mongodb` → `storage` | `replsets[].storage` |
| `mongodb` → `pbm-agent.resources` | `backup.resources` |
| `configserver` → same groups | `sharding.configsvrReplSet.*` |
| `mongos` → `resources`, probes, `configuration` | `sharding.mongos.*`; set `sharding.mongos.size` to `mongos.count`. |
| `monitor` → `resources`, `livenessProbe` | `pmm.resources`, `pmm.livenessProbe` (optional, see [Probes](#probes)) |

The operator has a single `backup.resources` and a single `pmm.resources` setting, so the values are the same for every component. The probe values equal the operator's runtime defaults, so you only need to set them if you want them pinned in the custom resource. A probe set in the custom resource replaces only the fields you give for `mongod` and `mongos`, but replaces the whole probe for PMM.

The [replica-set example](#complete-replica-set-example) maps to this fragment. Kubernetes accepts the plain byte values the calculator emits:

```yaml
spec:
  replsets:
  - name: rs0
    size: 3
    resources:
      limits:
        cpu: 1600m
        memory: "8333033472"
      requests:
        cpu: 1520m
        memory: "7915700224"
    configuration: |
      net:
        maxIncomingConnections: 675
    storage:
      engine: wiredTiger
      wiredTiger:
        engineConfig:
          cacheSizeRatio: 0.5
  pmm:
    enabled: true
    resources:
      limits:
        cpu: 400m
        memory: "256000000"
      requests:
        cpu: 300m
        memory: "150000000"
  logcollector:
    enabled: false
```

With these values the operator starts `mongod` with `--wiredTigerCacheSizeGB=3.38`, the `cacheSizeGB` the calculator reports. `logcollector.enabled: false` keeps the pod inside the dimension (see [Supported Dimensions](#supported-dimensions)).

The calculator does not generate authentication, TLS, read concern, write concern, replication, storage size, or backup scheduling policy. Configure those separately in the Percona Operator custom resource and application.

## Verification

```bash
gofmt -w src
go test ./...
go vet ./...
go build -o /tmp/mongodboperatorcalculator ./src
git diff --check
```

The README examples are checked against real output by `TestReadmeExamplesMatchOutput`. After an intentional output change, regenerate them with:

```bash
go test ./src -run TestReadmeExamples -update-readme
```
