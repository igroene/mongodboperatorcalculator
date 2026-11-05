# MongoDB Operator Calculator

Calculate starting-point CPU, memory, and configuration values for Percona Server for MongoDB deployments managed by the Percona Operator for MongoDB. Use it as an HTTP service or embed it in a Go application.

Provide a pod resource budget, workload profile, and peak total open client connection count. The calculator checks whether the budget fits that count under the selected workload assumptions, allocates resources to `mongod` and its PMM/PBM sidecars, and returns resource requests, limits, probes, and MongoDB settings. For sharded clusters, it also sizes config-server members and `mongos` routers separately.

## Compatibility and limitations

- **MongoDB:** Percona Server for MongoDB 7 and 8.
- **Operator:** probe defaults, WiredTiger cache behavior, and custom resource mapping checked against Percona Operator for MongoDB **1.23.0**. Other releases may behave differently.
- **Topologies:** replica sets and sharded clusters; JSON or human-readable output, with optional Guaranteed QoS.
- **Not covered:** storage sizing, replication tuning, backup scheduling, security policy, or autoscaling.

> **These are uncalibrated heuristic estimates, not measured capacity.** The model does not account for working-set size, query shapes, index coverage, or storage latency. Validate results with load testing and monitoring before production use. See [sizing assumptions and limitations](docs/sizing.md).

Two inputs are especially important:

- **`connections` means peak simultaneously open client connections, including idle driver-pool sockets.** Every connection receives the selected profile's CPU and memory allowance, assuming it may become busy. For replica sets, use the count expected on the busiest member, including about two driver monitoring sockets per application instance; for sharded clusters, use the total across all `mongos`. See [Choosing the connection count](docs/sizing.md#choosing-the-connection-count).
- **A dimension budgets one replica-set or shard `mongod` pod**, including PMM and optional PBM. Results are per component, not cluster totals; no shard count or replica-set member count is assumed. Config-server and `mongos` pods are sized separately.

## Quick start

Requires **Go 1.21 or newer** and `curl`.

### 1. Build and start the service

```bash
git clone https://github.com/igroene/mongodboperatorcalculator.git
cd mongodboperatorcalculator
go build -o mongodboperatorcalculator ./src
./mongodboperatorcalculator -address=127.0.0.1 -port=8080
```

Without flags, the service listens on `0.0.0.0:8080`. It stops gracefully on `SIGINT` and `SIGTERM`. See [command-line flags](docs/api.md#command-line-flags) for other options.

### 2. Submit a calculation

In another terminal, request a replica-set member with a `2000m` CPU / `8Gi` pod budget (dimension `4`), Light OLTP workload (profile `2`), and 500 total open client connections:

```bash
curl -sS -X POST -H 'Content-Type: application/json' \
  -d '{
    "dbtype": "replica_set",
    "dimension": {"id": 4},
    "loadtype": {"id": 2},
    "connections": 500,
    "mongodbversion": {"major": 7, "minor": 0, "patch": 0}
  }' \
  http://127.0.0.1:8080/calculator
```

JSON is the default output. Add `"output": "human"` for operator-oriented text. See the [dimension catalog](docs/api.md#supported-dimensions), [workload profiles](docs/api.md#workload-types), and [complete examples](docs/examples.md) for other requests.

### 3. Read the result

The response contains three sections:

| Section | What to look for |
|---|---|
| `message` | Status code and estimated resource pressure. |
| `incoming` | Resolved request, including the CPU and memory allocated to each component. |
| `answer` | Configuration, storage settings, resources, and probes grouped by component. |

For this request, the `message` section is:

```json
{
  "type": 1001,
  "name": "Configuration fits the requested connection count under the selected workload assumptions",
  "text": "estimated capacity utilization is 38% (CPU 38%, memory 28%); assumes all 500 open client connections use the selected workload profile"
}
```

This is an excerpt, not the full response. The result gives `mongod` a `1600m` CPU limit, leaving `400m` for PMM within the pod budget. Resource memory values are bytes. See the [full response](docs/examples.md#complete-replica-set-example) and [output reference](docs/api.md#output-structure).

| Message code | Meaning | HTTP status |
|---:|---|---:|
| `1001` | Estimated utilization at or below 85%. Not a capacity guarantee. | `200` |
| `2001` | Above 85%, up to 100%; close to the model's limit. | `200` |
| `3001` | Resources insufficient; `answer` is empty. | `422` |
| `5001` | Request decoded but failed validation. | `400` |

Malformed requests use a separate `error` object. See [error formats](docs/api.md#errors).

## Apply results to the operator

**Neither output format is an apply-ready Kubernetes manifest.** Follow the [Percona Operator mapping guide](docs/operator.md) to copy the values into your custom resource; it includes a replica-set YAML fragment.

Before applying results:

- Change the WiredTiger cache through `storage.wiredTiger.engineConfig.cacheSizeRatio`. The reported `cacheSizeGB` is informational; the operator overrides a value placed in `configuration`.
- Check `net.maxIncomingConnections` against the peak open connections to each component. It includes headroom above the supplied client count and an internal allowance; larger internal pools may require adjustment. Headroom permits extra connections but does not reserve workload capacity for them.
- Disable the log collector or budget for both of its containers separately. Custom sidecars are also outside the calculator's budget.
- Configure authentication, TLS, storage, replication, and backup policy separately, then test the deployment under load.

## Documentation

| I want to… | Read |
|---|---|
| Choose inputs, dimensions, or workload profiles | [API reference](docs/api.md) |
| Understand sizing, sidecar budgets, QoS, or probes | [Sizing and capacity model](docs/sizing.md) |
| Apply results to a custom resource | [Percona Operator mapping](docs/operator.md) |
| Try sharded clusters, dedicated mode, backup, or error cases | [Complete examples](docs/examples.md) |
| Embed the calculator in Go | [Go library guide](docs/go-library.md) |
| Run checks or update output examples | [Contributing](CONTRIBUTING.md) |

The HTTP service also provides [`GET /supported`](docs/api.md#get-supported) for supported inputs and [`GET /healthz`](docs/api.md#get-healthz) for liveness and readiness checks.

The public API follows the shape of the [MySQL Operator Calculator](https://github.com/Tusamarco/mysqloperatorcalculator), but uses MongoDB-specific sizing rules. MySQL features such as dimension `998` and automatic connection sizing are not supported.
