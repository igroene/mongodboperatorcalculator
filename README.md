# MongoDB Operator Calculator

Conservative resource and configuration calculator for Percona Server for MongoDB deployments managed by the Percona Operator for MongoDB.

The project mirrors the public shape of the MySQL Operator Calculator while using MongoDB-specific sizing rules.

## Scope

- Percona Server for MongoDB 7 and 8
- Replica sets and sharded clusters
- Per-member `mongod` sizing; no replica-set member count is assumed
- Per-component sharded output for one shard member, one config-server member, one `mongos`, and PMM
- Kubernetes resource requests, limits, and probes
- JSON and human-readable output
- No storage sizing, replication tuning, security policy, or autoscaling

The calculator estimates maximum safe connection-driven throughput. It is not a benchmark and does not guarantee a particular operations-per-second rate.

## Build and Run

```bash
go build -o mongodboperatorcalculator ./src
./mongodboperatorcalculator
```

The server listens on `0.0.0.0:8080` by default. Pass a listen address as the first argument if required.

## Example

```bash
curl -X POST -H 'Content-Type: application/json' \
  -d '{
    "output": "json",
    "dbtype": "replica_set",
    "dimension": {"id": 2},
    "loadtype": {"id": 2},
    "connections": 500,
    "mongodbversion": {"major": 7, "minor": 0, "patch": 0}
  }' \
  http://127.0.0.1:8080/calculator
```

For `sharded_cluster`, the answer contains per-component values for one shard `mongod` member, one config-server member, one `mongos`, and PMM. It does not assume or calculate a shard count.

Use dimension `999` for explicit CPU millicores and memory, such as `8Gi`, `16GB`, or `4096Mi`. `providercostpct` reduces resources before calculation. Dedicated mode (`mongodbdedicated: true`) emits only the `mongodb` family and allocates the full dimension to `mongod`.

## Sizing Rules

- WiredTiger cache is always 50% of the `mongod` pod memory limit.
- The remaining memory is reserved for filesystem cache, connections, active operations, process overhead, PMM, and safety margin.
- CPU pressure is estimated from connections and workload profile.
- `mongos` does not receive a WiredTiger cache allocation.
- `connections` must be greater than zero. There is no automatic connection backoff or dimension scaling.
- An overloaded request returns status `3001` and no answer families.

The calculator does not emit authentication, TLS, write concern, read concern, replication, storage, or backup policy. Those remain deployment decisions.

## Workload Types

| ID | Name | Description |
|---:|---|---|
| 1 | Mainly Reads | Read-heavy workload |
| 2 | Light OLTP | Mixed workload with moderate writes |
| 3 | Heavy OLTP | Highly concurrent mixed workload |
| 4 | Mainly Writes | Write-heavy workload |

## Verification

```bash
go test ./...
go vet ./...
go build ./src
```
