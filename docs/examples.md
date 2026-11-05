# Complete Examples

[Back to the README](../README.md) · [API reference](api.md) · [Apply results to the operator](operator.md)

These full request/response pairs are checked against real output by the [documentation example test](../CONTRIBUTING.md#documentation-examples). Each resource result is per component, not a cluster total. The utilization figures are [heuristic estimates](sizing.md#what-the-numbers-mean), not measured capacity.

Start the service using the [quick start](../README.md#quick-start), then choose an example:

- [Replica set](#complete-replica-set-example)
- [Sharded cluster with backup](#complete-sharded-cluster-example)
- [Dedicated mode](#complete-dedicated-example)
- [Overloaded workload](#overload-example)
- [Dimension too small for sidecars](#dimension-too-small-example)
- [Invalid request](#invalid-request-example)
- [Human-readable output](#operator-oriented-human-output)

## Complete Replica-Set Example

Save this request as `replica-request.json`:

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
    "name": "Configuration fits the requested connection count under the selected workload assumptions",
    "text": "estimated capacity utilization is 38% (CPU 38%, memory 28%); assumes all 500 open client connections use the selected workload profile"
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
    "name": "Configuration fits the requested connection count under the selected workload assumptions",
    "text": "estimated capacity utilization is 48% (shard mongod 48%, config server 20%, mongos 69%); assumes all 1500 open client connections use the selected workload profile"
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
    "name": "Configuration fits the requested connection count under the selected workload assumptions",
    "text": "estimated capacity utilization is 23% (CPU 19%, memory 23%); assumes all 100 open client connections use the selected workload profile"
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
    "name": "Resources do not fit the requested connection count under the selected workload assumptions",
    "text": "estimated capacity exceeded: mongod at 7525% (CPU 7525%, memory 1784%); choose a larger dimension or reduce connections; assumes all 10000 open client connections use the selected workload profile"
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
    "name": "Resources do not fit the requested connection count under the selected workload assumptions",
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
name = Configuration fits the requested connection count under the selected workload assumptions
type = 1001
text = estimated capacity utilization is 46% (CPU 46%, memory 31%); assumes all 500 open client connections use the selected workload profile

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
