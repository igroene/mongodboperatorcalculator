# MongoDB Operator Calculator

The MongoDB Operator Calculator is a Go library and HTTP service that produces conservative resource and configuration recommendations for Percona Server for MongoDB deployments managed by the Percona Operator for MongoDB.

It mirrors the public API shape of the MySQL Operator Calculator while using MongoDB-specific sizing rules.

## Scope

- Percona Server for MongoDB 7 and 8.
- Replica sets and sharded clusters.
- One `mongod` member per result; no replica-set member count is assumed.
- Per-component sharded output for one shard member, one config-server member, one `mongos`, and PMM.
- Kubernetes CPU and memory requests, limits, and probes.
- JSON and operator-oriented human-readable output.
- No storage sizing, replication tuning, security policy, backup policy, or autoscaling.

The calculator estimates maximum safe connection-driven throughput. It is not a benchmark and does not guarantee a particular operations-per-second rate. Validate generated values against the target workload before production use.

## Build and Run

```bash
git clone https://github.com/igroene/mongodboperatorcalculator.git
cd mongodboperatorcalculator
./mongodboperatorcalculator
```

The default server address is `0.0.0.0:8080`.

```bash
./mongodboperatorcalculator -address=127.0.0.1 -port=8080 -loglevel=INFO
```

## Command-Line Flags

| Flag | Default | Description |
|---|---:|---|
| `-address` | `0.0.0.0` | IP address or hostname to bind to. |
| `-port` | `8080` | TCP port to listen on. |
| `-loglevel` | `INFO` | Log level: `ERROR`, `INFO`, or `DEBUG`. Unknown values fall back to `INFO`. |
| `-version` | `false` | Print the calculator version and exit. |
| `-help` | `false` | Print command-line usage and exit. The standard Go `flag` package also accepts `-h`. |

Examples:

```bash
./mongodboperatorcalculator -help
./mongodboperatorcalculator -version
./mongodboperatorcalculator -address=127.0.0.1 -port=9090 -loglevel=DEBUG
```

## Input Parameters

All calculator inputs are passed as a JSON request to `POST /calculator` or through the Go module API.

| Parameter | Type | Required | Description |
|---|---|---:|---|
| `output` | string | Yes | `json` for structured output or `human` for operator-oriented text. Any value other than `human` uses JSON output. |
| `dbtype` | string | Yes | `replica_set` or `sharded_cluster`. |
| `dimension.id` | integer | Yes | Predefined dimension ID `1` through `4`, or `999` for explicit resources. Dimension `998` is not supported. |
| `dimension.cpu` | integer | Conditional | Total CPU in millicores for dimension `999`; for example, `4000` is four CPU cores. |
| `dimension.memory` | string | Conditional | Total memory for dimension `999`; examples include `8Gi`, `16GB`, and `4096Mi`. |
| `loadtype.id` | integer | Yes | Workload profile `1` through `4`. |
| `connections` | integer | Yes | Target concurrent connection count. Must be greater than zero. There is no automatic backoff or autoscaling. |
| `mongodbversion.major` | integer | Yes | Supported major version: `7` or `8`. |
| `mongodbversion.minor` | integer | Yes | MongoDB minor version. |
| `mongodbversion.patch` | integer | Yes | MongoDB patch version. |
| `providercostpct` | float | No | Fraction of resources reserved for platform overhead. For example, `0.12` reserves 12%. Default is `0`. Must be at least `0` and less than `1`. |
| `mongodbdedicated` | boolean | No | When `true`, allocate the entire replica-set dimension to `mongod` and omit PMM. Default is `false`. |

Custom memory quantities accept binary and decimal-looking suffixes supported by the calculator, including `B`, `KB`, `KiB`, `MB`, `MiB`, `GB`, `GiB`, `TB`, and `TiB`. Values are converted to bytes internally.

Invalid requests return message type `5001` and HTTP status `400`. `connections: 0` is invalid. The calculator does not implement the MySQL calculator's connection-driven dimension `998`, automatic connection discovery, connection backoff, or dimension scaling.

## Supported Dimensions

Predefined dimensions are total resource envelopes. Replica-set results describe one `mongod` member plus PMM. Sharded results split the envelope between one shard `mongod`, one config-server member, one `mongos`, and PMM; values are not multiplied by shard count.

| ID | Name | Total CPU | Total Memory | PMM CPU | PMM Memory |
|---:|---|---:|---:|---:|---:|
| `1` | XSmall | `600m` | `1Gi` | `100m` | `128Mi` |
| `2` | Small | `800m` | `2Gi` | `100m` | `128Mi` |
| `3` | Medium | `1000m` | `4Gi` | `200m` | `256Mi` |
| `4` | Large | `2000m` | `8Gi` | `300m` | `512Mi` |
| `5` | XLarge | `4000m` | `16Gi` | `400m` | `512Mi` |
| `6` | 2XLarge | `8000m` | `32Gi` | `500m` | `1Gi` |
| `7` | 4XLarge | `16000m` | `64Gi` | `1000m` | `1Gi` |
| `8` | 8XLarge | `32000m` | `128Gi` | `1000m` | `1Gi` |
| `9` | 12XLarge | `64000m` | `256Gi` | `1500m` | `2Gi` |
| `10` | 16XLarge | `128000m` | `512Gi` | `2000m` | `2Gi` |
| `999` | Open request by resources | User supplied | User supplied |

The dimension catalog intentionally includes small development profiles below MongoDB's general production recommendation of two real CPU cores. Use larger profiles for production workloads. Every predefined dimension reserves an explicit PMM CPU and memory allocation; the remaining resources are assigned to `mongod` for replica-set results.

## Workload Types

| ID | Name | Description |
|---:|---|---|
| `1` | Mainly Reads | Read-heavy workload. |
| `2` | Light OLTP | Mixed workload with moderate writes. |
| `3` | Heavy OLTP | Highly concurrent mixed workload. |
| `4` | Mainly Writes | Write-heavy workload. |

Workload profiles affect conservative CPU and connection-memory estimates. They do not change replication, read concern, write concern, authentication, TLS, or storage settings.

## Dedicated Mode

Set `mongodbdedicated` to `true` when the pod contains only `mongod`:

```json
{
  "dbtype": "replica_set",
  "dimension": {"id": 2},
  "loadtype": {"id": 1},
  "connections": 100,
  "mongodbversion": {"major": 7, "minor": 0, "patch": 0},
  "mongodbdedicated": true
}
```

Dedicated mode:

- Allocates the complete dimension to `mongod`.
- Omits the `monitor` family.
- Is valid only for `replica_set`.
- Keeps WiredTiger cache at 50% of the `mongod` memory limit.
- Rejects dedicated `sharded_cluster` requests.

## Sizing Rules

- WiredTiger cache is always `50%` of the `mongod` pod memory limit.
- The remainder is reserved for filesystem cache, connections, active operations, process overhead, PMM, and safety margin.
- CPU pressure is estimated from connections and workload profile.
- `mongos` does not receive a WiredTiger cache allocation.
- Kubernetes requests are calculated as 95% of the corresponding limits.
- A valid request exceeding safe capacity returns message type `3001`, HTTP `422`, and an empty `answer` object.
- A request within the safety envelope returns message type `1001` or `2001`.

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
| `1001` | Successful calculation within the safe resource envelope. |
| `2001` | Successful calculation close to the safety threshold. |
| `3001` | Requested workload exceeds safe resources; `answer` is empty. |
| `5001` | Invalid or unsupported request. |
| `6001` | Compatibility code from the MySQL calculator; not emitted. |
| `7001` | Compatibility code from the MySQL calculator; not emitted. |

### `incoming`

The resolved request, including the selected dimension, component resource allocation, normalized workload type, and MongoDB version.

### `answer`

Calculated configuration families. Each family contains groups, and each group contains parameters with `name` and `value` fields.

| Family | Meaning |
|---|---|
| `mongodb` | One shard or replica-set `mongod` member. |
| `configserver` | One config-server `mongod` member in a sharded cluster. |
| `mongos` | One `mongos` query router in a sharded cluster. |
| `monitor` | PMM Client sidecar resources. |

| Group | Meaning |
|---|---|
| `configuration` | MongoDB configuration values. |
| `resources` | Kubernetes CPU and memory requests and limits. |
| `readinessProbe` | Readiness probe settings. |
| `livenessProbe` | Liveness probe settings. |

Output units are parameter-specific: resource memory values are bytes, CPU values are Kubernetes millicores, WiredTiger cache is GB, and connection limits are integers.

## HTTP API

### `GET /supported`

Returns supported database types, dimensions, workload types, connection presets, output formats, and MongoDB version range.

```bash
curl -i http://127.0.0.1:8080/supported
```

Example response:

```json
{
  "dbtype": ["replica_set", "sharded_cluster"],
  "dimension": [
    {"id": 1, "name": "XSmall", "cpu": 600, "memory": "1Gi"},
    {"id": 2, "name": "Small", "cpu": 800, "memory": "2Gi"},
    {"id": 3, "name": "Medium", "cpu": 1000, "memory": "4Gi"},
    {"id": 4, "name": "Large", "cpu": 2000, "memory": "8Gi"},
    {"id": 5, "name": "XLarge", "cpu": 4000, "memory": "16Gi"},
    {"id": 6, "name": "2XLarge", "cpu": 8000, "memory": "32Gi"},
    {"id": 7, "name": "4XLarge", "cpu": 16000, "memory": "64Gi"},
    {"id": 8, "name": "8XLarge", "cpu": 32000, "memory": "128Gi"},
    {"id": 9, "name": "12XLarge", "cpu": 64000, "memory": "256Gi"},
    {"id": 10, "name": "16XLarge", "cpu": 128000, "memory": "512Gi"},
    {"id": 999, "name": "Open request by resources", "cpu": 0, "memory": "0"}
  ],
  "loadtype": [
    {"id": 1, "name": "Mainly Reads", "example": "Read-heavy workload"},
    {"id": 2, "name": "Light OLTP", "example": "Mixed workload with moderate writes"},
    {"id": 3, "name": "Heavy OLTP", "example": "Highly concurrent mixed workload"},
    {"id": 4, "name": "Mainly Writes", "example": "Write-heavy workload"}
  ],
  "connections": [50, 100, 200, 500, 1000, 2000],
  "output": ["human", "json"],
  "mongodbversions": {"min": {"major": 7, "minor": 0, "patch": 0}, "max": {"major": 8, "minor": 99, "patch": 99}}
}
```

Only `GET` is supported. Other methods return HTTP `405` and `Allow: GET`.

### `POST /calculator`

Accepts a JSON request body and returns JSON by default, or operator-oriented text when `output` is `human`.

Only `POST` is supported. Other methods return HTTP `405` and `Allow: POST`.

HTTP statuses:

| Status | Meaning |
|---:|---|
| `200` | Successful or close-to-limit calculation. |
| `400` | Malformed JSON or invalid request. |
| `422` | Valid request but insufficient resources for the requested workload. |
| `405` | Unsupported HTTP method. |

## Complete Replica-Set Example

Request:

```json
{
  "output": "json",
  "dbtype": "replica_set",
  "dimension": {"id": 2},
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

```json
{
  "message": {
    "type": 1001,
    "name": "Execution was successful and resources match the requested workload",
    "text": "estimated capacity utilization is 75%"
  },
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {
      "id": 2,
      "name": "Small",
      "cpu": 800,
      "memory": "2Gi",
      "mongodbCpu": 700,
      "monitorCpu": 100,
      "mongosCpu": 0,
      "configCpu": 0,
      "mongodbMemory": 2013265920,
      "monitorMemory": 134217728,
      "mongosMemory": 0,
      "configMemory": 0
    },
    "loadtype": {"id": 2, "name": "Light OLTP", "example": "Mixed workload with moderate writes"},
    "connections": 500,
    "output": "json",
    "mongodbversion": {"major": 7, "minor": 0, "patch": 0},
    "providercostpct": 0,
    "mongodbdedicated": false
  },
  "answer": {
    "mongodb": {
      "name": "mongod",
      "groups": {
        "configuration": {
          "name": "configuration",
          "parameters": {
            "net.maxIncomingConnections": {"name": "net.maxIncomingConnections", "value": "575"},
            "storage.engine": {"name": "storage.engine", "value": "wiredTiger"},
            "storage.wiredTiger.engineConfig.cacheSizeGB": {"name": "storage.wiredTiger.engineConfig.cacheSizeGB", "value": "0.94"}
          }
        },
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "45"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "23"}}},
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {"name": "limit_cpu", "value": "700m"},
            "limit_memory": {"name": "limit_memory", "value": "2013265920"},
            "request_cpu": {"name": "request_cpu", "value": "665m"},
            "request_memory": {"name": "request_memory", "value": "1912602624"}
          }
        }
      }
    },
    "monitor": {
      "name": "pmm-client",
      "groups": {
        "configuration": {"name": "configuration", "parameters": {}},
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "45"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "23"}}},
        "resources": {
          "name": "resources",
          "parameters": {
            "limit_cpu": {"name": "limit_cpu", "value": "100m"},
            "limit_memory": {"name": "limit_memory", "value": "134217728"},
            "request_cpu": {"name": "request_cpu", "value": "95m"},
            "request_memory": {"name": "request_memory", "value": "127506841"}
          }
        }
      }
    }
  }
}
```

## Complete Sharded-Cluster Example

Request:

```json
{
  "output": "json",
  "dbtype": "sharded_cluster",
  "dimension": {"id": 3},
  "loadtype": {"id": 2},
  "connections": 500,
  "mongodbversion": {"major": 8, "minor": 0, "patch": 0}
}
```

Expected output families and values:

```json
{
  "message": {"type": 1001, "name": "Execution was successful and resources match the requested workload", "text": "estimated capacity utilization is 72%"},
  "incoming": {
    "dbtype": "sharded_cluster",
    "dimension": {
      "id": 3, "name": "Medium", "cpu": 1000, "memory": "4Gi",
      "mongodbCpu": 700, "monitorCpu": 50, "mongosCpu": 100, "configCpu": 150,
      "mongodbMemory": 3006477107.2, "monitorMemory": 214748364.8000002,
      "mongosMemory": 429496729.6, "configMemory": 644245094.4
    },
    "loadtype": {"id": 2, "name": "Light OLTP", "example": "Mixed workload with moderate writes"},
    "connections": 500,
    "output": "json",
    "mongodbversion": {"major": 8, "minor": 0, "patch": 0},
    "providercostpct": 0,
    "mongodbdedicated": false
  },
  "answer": {
    "configserver": {
      "name": "config-server",
      "groups": {
        "configuration": {"name": "configuration", "parameters": {}},
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "45"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "23"}}},
        "resources": {"name": "resources", "parameters": {"limit_cpu": {"name": "limit_cpu", "value": "150m"}, "limit_memory": {"name": "limit_memory", "value": "644245094"}, "request_cpu": {"name": "request_cpu", "value": "142m"}, "request_memory": {"name": "request_memory", "value": "611032839"}}}
      }
    },
    "mongodb": {
      "name": "mongod",
      "groups": {
        "configuration": {"name": "configuration", "parameters": {"net.maxIncomingConnections": {"name": "net.maxIncomingConnections", "value": "575"}, "storage.engine": {"name": "storage.engine", "value": "wiredTiger"}, "storage.wiredTiger.engineConfig.cacheSizeGB": {"name": "storage.wiredTiger.engineConfig.cacheSizeGB", "value": "1.4"}}},
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "45"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "23"}}},
        "resources": {"name": "resources", "parameters": {"limit_cpu": {"name": "limit_cpu", "value": "700m"}, "limit_memory": {"name": "limit_memory", "value": "3006477107"}, "request_cpu": {"name": "request_cpu", "value": "665m"}, "request_memory": {"name": "request_memory", "value": "2856153252"}}}
      }
    },
    "mongos": {
      "name": "mongos",
      "groups": {
        "configuration": {"name": "configuration", "parameters": {"net.maxIncomingConnections": {"name": "net.maxIncomingConnections", "value": "575"}}},
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "45"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "23"}}},
        "resources": {"name": "resources", "parameters": {"limit_cpu": {"name": "limit_cpu", "value": "100m"}, "limit_memory": {"name": "limit_memory", "value": "429496729"}, "request_cpu": {"name": "request_cpu", "value": "95m"}, "request_memory": {"name": "request_memory", "value": "408021893"}}}
      }
    },
    "monitor": {
      "name": "pmm-client",
      "groups": {
        "configuration": {"name": "configuration", "parameters": {}},
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "45"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "23"}}},
        "resources": {"name": "resources", "parameters": {"limit_cpu": {"name": "limit_cpu", "value": "50m"}, "limit_memory": {"name": "limit_memory", "value": "214748364"}, "request_cpu": {"name": "request_cpu", "value": "47m"}, "request_memory": {"name": "request_memory", "value": "204010946"}}}
      }
    }
  }
}
```

Every value above is per component. The calculator does not multiply the shard `mongod` values by a shard count.

## Complete Dedicated Example

Request:

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

```json
{
  "message": {"type": 1001, "name": "Execution was successful and resources match the requested workload", "text": "estimated capacity utilization is 66%"},
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {"id": 2, "name": "Small", "cpu": 800, "memory": "2Gi", "mongodbCpu": 800, "monitorCpu": 100, "mongosCpu": 0, "configCpu": 0, "mongodbMemory": 2147483648, "monitorMemory": 134217728, "mongosMemory": 0, "configMemory": 0},
    "loadtype": {"id": 1, "name": "Mainly Reads", "example": "Read-heavy workload"},
    "connections": 100,
    "output": "json",
    "mongodbversion": {"major": 7, "minor": 0, "patch": 0},
    "providercostpct": 0,
    "mongodbdedicated": true
  },
  "answer": {
    "mongodb": {
      "name": "mongod",
      "groups": {
        "configuration": {"name": "configuration", "parameters": {"net.maxIncomingConnections": {"name": "net.maxIncomingConnections", "value": "115"}, "storage.engine": {"name": "storage.engine", "value": "wiredTiger"}, "storage.wiredTiger.engineConfig.cacheSizeGB": {"name": "storage.wiredTiger.engineConfig.cacheSizeGB", "value": "1"}}},
        "livenessProbe": {"name": "livenessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "33"}}},
        "readinessProbe": {"name": "readinessProbe", "parameters": {"timeoutSeconds": {"name": "timeoutSeconds", "value": "17"}}},
        "resources": {"name": "resources", "parameters": {"limit_cpu": {"name": "limit_cpu", "value": "800m"}, "limit_memory": {"name": "limit_memory", "value": "2147483648"}, "request_cpu": {"name": "request_cpu", "value": "760m"}, "request_memory": {"name": "request_memory", "value": "2040109465"}}}
      }
    }
  }
}
```

The `monitor`, `mongos`, and `configserver` families are intentionally absent.

## Overload Example

```bash
curl -i -X POST -H 'Content-Type: application/json' \
  -d '{"output":"json","dbtype":"replica_set","dimension":{"id":2},"loadtype":{"id":4},"connections":10000,"mongodbversion":{"major":7,"minor":0,"patch":0}}' \
  http://127.0.0.1:8080/calculator
```

The response has HTTP status `422` and the following shape:

```json
{
  "message": {"type": 3001, "name": "Resources overloaded", "text": "safe capacity exceeded"},
  "incoming": {
    "dbtype": "replica_set",
    "dimension": {"id": 2, "name": "Small", "cpu": 800, "memory": "2Gi", "mongodbCpu": 700, "monitorCpu": 100, "mongosCpu": 0, "configCpu": 0, "mongodbMemory": 2013265920, "monitorMemory": 134217728, "mongosMemory": 0, "configMemory": 0},
    "loadtype": {"id": 4, "name": "Mainly Writes", "example": "Write-heavy workload"},
    "connections": 10000,
    "output": "json",
    "mongodbversion": {"major": 7, "minor": 0, "patch": 0},
    "providercostpct": 0,
    "mongodbdedicated": false
  },
  "answer": {}
}
```

## Operator-Oriented Human Output

Set `output` to `human` to receive sections suitable for mapping into a Percona Operator custom resource:

```ini
[message]
name = Execution was successful and resources match the requested workload
type = 1001
text = estimated capacity utilization is 75%

[mongod.configuration]
net.maxIncomingConnections = 575
storage.engine = wiredTiger
storage.wiredTiger.engineConfig.cacheSizeGB = 0.94

[mongod.resources]
limit_cpu = 700m
limit_memory = 2013265920
request_cpu = 665m
request_memory = 1912602624

[mongod.readinessProbe]
timeoutSeconds = 23

[mongod.livenessProbe]
timeoutSeconds = 45

[pmm-client.resources]
limit_cpu = 100m
limit_memory = 134217728
request_cpu = 95m
request_memory = 127506841
```

Sharded output uses `mongod`, `configserver.mongod`, `mongos`, and `pmm-client` sections. Values are sorted deterministically.

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
    Dimension:   MO.Dimension{Id: 2},
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

Calculate a custom dimension:

```go
request.Dimension = MO.Dimension{
    Id:     MO.DimensionOpen,
    Cpu:    4000,
    Memory: "8Gi",
}
request.Dimension.MemoryBytes, err =
    request.Dimension.ConvertMemoryToBytes(request.Dimension.Memory)
if err != nil {
    log.Fatal(err)
}
request = calculator.Init(request, conf)
```

Access a calculated family and parameter:

```go
mongodbFamily, err := calculator.GetFamily(MO.FamilyTypeMongoDB)
if err != nil {
    log.Fatal(err)
}

cache := mongodbFamily.Groups[MO.GroupNameConfiguration].Parameters[
    "storage.wiredTiger.engineConfig.cacheSizeGB",
]
fmt.Println(cache.Value)
```

The public `Init()` signature is intentionally compatible with the MySQL calculator. Validation and calculation errors are returned by `GetCalculate()`.

## Percona Operator Mapping

| Calculator family | Percona Operator for MongoDB target |
|---|---|
| `mongodb` | One replica-set or shard `mongod` member configuration and resources. |
| `configserver` | One config-server `mongod` member in a sharded deployment. |
| `mongos` | One sharded-cluster query router. |
| `monitor` | PMM Client sidecar resources. |

The calculator does not generate authentication, TLS, read concern, write concern, replication, storage, or backup policy. Configure those separately in the Percona Operator custom resource and application.

## Verification

```bash
gofmt -w src
go test ./...
go vet ./...
go build -o /tmp/mongodboperatorcalculator ./src
git diff --check
```
