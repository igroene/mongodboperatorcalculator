# API Reference

[Back to the README](../README.md) · [Complete examples](examples.md)

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

| Status | Meaning |
|---:|---|
| `200` | Successful or close-to-limit calculation. |
| `400` | Malformed JSON, unknown fields, or invalid request. |
| `405` | Unsupported HTTP method. |
| `413` | Request body larger than 1 MiB. |
| `422` | Valid request but insufficient resources for the requested workload. |

The server sets read, write, and idle timeouts, so slow clients cannot hold connections open.

## Input Parameters

All calculator inputs are passed as a JSON request to `POST /calculator` or through the [Go module API](go-library.md). The HTTP API rejects unknown fields and trailing data after the JSON object.

| Parameter | Type | Required | Description |
|---|---|---:|---|
| `output` | string | No | `json` (default) or `human`. Any other value is rejected. |
| `dbtype` | string | Yes | `replica_set` or `sharded_cluster`. |
| `dimension.id` | integer | Yes | Predefined dimension ID `1` through `10`, or `999` for explicit resources. Dimension `998` is not supported. |
| `dimension.cpu` | integer | Conditional | Total pod CPU in millicores for dimension `999`, from `1` to `512000`. For predefined dimensions it may be omitted or must match the catalog value. |
| `dimension.memory` | string | Conditional | Total pod memory for dimension `999`, for example `8Gi`, `8G`, `4096Mi`, or `8589934592`. For predefined dimensions it may be omitted or must match the catalog value. |
| `loadtype.id` | integer | Yes | Workload profile `1` through `4`. |
| `connections` | integer | Yes | Peak simultaneously open client connections, including idle driver-pool sockets, from `1` to `1000000`. Every connection receives the selected workload's CPU and memory allowance. For replica sets, use the count on the busiest member, including driver monitoring sockets from every application instance; for sharded clusters, use the total across all `mongos`. See [Choosing the connection count](sizing.md#choosing-the-connection-count). |
| `mongodbversion.major` | integer | Yes | Supported major version: `7` or `8`. |
| `mongodbversion.minor` | integer | Yes | MongoDB minor version, `0` to `99`. |
| `mongodbversion.patch` | integer | Yes | MongoDB patch version, `0` to `99`. |
| `providercostpct` | float | No | Fraction of the dimension reserved for platform overhead. For example, `0.12` reserves 12%. Default is `0`. Must be at least `0` and less than `1`. |
| `mongodbdedicated` | boolean | No | When `true`, omit PMM so `mongod` gets the whole pod budget except PBM. Valid only for `replica_set`. Default is `false`. See [Dedicated Mode](sizing.md#dedicated-mode). |
| `backup.enabled` | boolean | No | When `true`, add a PBM agent sidecar to shard and config-server `mongod` pods. Default is `false`. |
| `guaranteedqos` | boolean | No | When `true`, every container the calculator emits gets requests equal to limits. See [Requests and limits](sizing.md#requests-and-limits) for what else Guaranteed QoS needs. Default is `false`. |
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

A dimension is the total CPU and memory of one shard or replica-set `mongod` pod. The `mongod` container and its sidecars share that budget. See [Pod budget and sidecars](sizing.md#pod-budget-and-sidecars) for how resources are allocated.

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

| ID | Name | Description |
|---:|---|---|
| `1` | Mainly Reads | Read-heavy workload. |
| `2` | Light OLTP | Mixed workload with moderate writes. |
| `3` | Heavy OLTP | Highly concurrent mixed workload. |
| `4` | Mainly Writes | Write-heavy workload. |

Workload profiles affect the CPU and memory estimates. They do not change replication, read concern, write concern, authentication, TLS, or storage settings. The assumed per-connection costs are documented in the [Capacity Model](sizing.md#capacity-model).

## Output Structure

JSON output has three top-level fields:

```json
{
  "message": {},
  "incoming": {},
  "answer": {}
}
```

For text output, set `output` to `human`. See the [human-output example](examples.md#operator-oriented-human-output). Neither format is an apply-ready operator manifest; see [Percona Operator Mapping](operator.md).

### `message`

Diagnostic status information:

| Code | Meaning |
|---:|---|
| `1001` | Estimated utilization is at or below 85%. |
| `2001` | Estimated utilization is above 85% and at or below 100%. |
| `3001` | Estimated utilization is above 100%, or `mongod` does not fit in the dimension; `answer` is empty. Only the first case states the connection assumption in `text`. |
| `5001` | The request parsed but is invalid or unsupported. |
| `6001` | Compatibility code from the MySQL calculator; not emitted. |
| `7001` | Compatibility code from the MySQL calculator; not emitted. |

Branch on `type` only. `name` and `text` are human-readable and may change between releases; for example, their wording changed when `connections` was redefined as total open connections.

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

## Errors

There are two error formats:

- **The body could not be decoded** (empty body, malformed JSON, unknown fields, trailing data, or a body over 1 MiB). The response is a plain JSON object with a single `error` field, HTTP `400` or `413`, and no `message`, `incoming`, or `answer`:

  ```json
  {"error": "malformed JSON: json: unknown field \"shards\""}
  ```

- **The body decoded but failed validation** (unsupported values, out-of-range numbers, an unparsable memory quantity). The response uses the normal structure with message type `5001`, HTTP `400`, and an empty `answer`. `incoming` holds the decoded request as it was before any defaults were applied: fields you left out appear with their zero values, and nothing is calculated. See the [Invalid Request Example](examples.md#invalid-request-example).

Other methods return HTTP `405` with the same `error` object.

## Command-Line Flags

| Flag | Default | Description |
|---|---:|---|
| `-address` | `0.0.0.0` | IP address or hostname to bind to. |
| `-port` | `8080` | TCP port to listen on. |
| `-loglevel` | `INFO` | Log level: `ERROR`, `INFO`, or `DEBUG`. Unknown values fall back to `INFO`. `DEBUG` logs every calculation result. |
| `-version` | `false` | Print the calculator version and exit. |
| `-help` | `false` | Print command-line usage and exit. The standard Go `flag` package also accepts `-h`. |

```bash
./mongodboperatorcalculator -help
./mongodboperatorcalculator -version
./mongodboperatorcalculator -address=127.0.0.1 -port=9090 -loglevel=DEBUG
```
