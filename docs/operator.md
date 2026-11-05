# Percona Operator Mapping

[Back to the README](../README.md) · [Complete examples](examples.md) · [Sizing assumptions](sizing.md)

Mapping verified against Percona Operator for MongoDB 1.23.0. Calculator output is not an apply-ready custom resource: map the values to the fields below and configure the rest of your deployment separately.

## Map the output

| Calculator group | Custom resource field |
|---|---|
| `mongodb` → `resources` | `replsets[].resources` |
| `mongodb` → `livenessProbe`, `readinessProbe` | `replsets[].livenessProbe`, `replsets[].readinessProbe` |
| `mongodb` → `configuration` | `replsets[].configuration` (a YAML string). Copy `net.maxIncomingConnections`; leave `cacheSizeGB` out. |
| `mongodb` → `storage` | `replsets[].storage` |
| `mongodb` → `pbm-agent.resources` | `backup.resources` |
| `configserver` → same groups | `sharding.configsvrReplSet.*` |
| `mongos` → `resources`, probes, `configuration` | `sharding.mongos.*`; set `sharding.mongos.size` to `mongos.count`. |
| `monitor` → `resources`, `livenessProbe` | `pmm.resources`, `pmm.livenessProbe` (optional, see [Probes](sizing.md#probes)) |

The operator has a single `backup.resources` and a single `pmm.resources` setting, so the values are the same for every component. The probe values equal the operator's runtime defaults, so you only need to set them if you want them pinned in the custom resource. A probe set in the custom resource replaces only the fields you give for `mongod` and `mongos`, but replaces the whole probe for PMM.

## Replica-set fragment

The [replica-set example](examples.md#complete-replica-set-example) maps to this fragment. Kubernetes accepts the plain byte values the calculator emits. The member count of `3` is an example deployment choice, not a calculator result.

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

With these values the operator starts `mongod` with `--wiredTigerCacheSizeGB=3.38`, the `cacheSizeGB` the calculator reports. Change the cache using `storage.wiredTiger.engineConfig.cacheSizeRatio`; setting `cacheSizeGB` in `configuration` has no effect because the operator overrides it.

`logcollector.enabled: false` keeps the pod inside the dimension. If you enable it, budget for both log-collector containers separately. Also budget for any custom sidecars. See [Pod budget and sidecars](sizing.md#pod-budget-and-sidecars) and the [Guaranteed QoS requirements](sizing.md#requests-and-limits).

Check `net.maxIncomingConnections` against the peak number of open connections to each component, including idle driver-pool sockets. The calculator uses the busy connection count and does not explicitly account for idle sockets; raise the limit as needed.

The calculator does not generate authentication, TLS, read concern, write concern, replication, storage size, or backup scheduling policy. Configure those separately in the Percona Operator custom resource and application. Validate the resulting deployment with load testing and monitoring before production use.
