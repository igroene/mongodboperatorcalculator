# Using as a Go Module

[Back to the README](../README.md) · [Input and output reference](api.md) · [Sizing assumptions](sizing.md)

## Calculate a configuration

This complete example calculates a replica-set configuration and prints the JSON result:

```go
package main

import (
    "fmt"
    "log"

    MO "github.com/igroene/mongodboperatorcalculator/src/mongodboperatorcalculator"
)

func main() {
    var conf MO.Configuration
    conf.Init()

    var calculator MO.MongoDBOperatorCalculator
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
        log.Fatalf("calculation failed: %v", calculationErr)
    }

    output, err := calculator.GetJSONOutput(message, request, families)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(output.String())
}
```

The public `Init()` signature is intentionally compatible with the MySQL calculator. Validation and calculation errors are returned by `GetCalculate()`. Unlike the HTTP API, the Go API cannot reject unknown fields; everything else is validated the same way.

## Fetch supported metadata

With the calculator above, retrieve supported layouts instead of calling the HTTP `/supported` endpoint. Add `encoding/json` to the imports:

```go
supported := calculator.GetSupportedLayouts()
metadata, err := json.MarshalIndent(supported, "", "  ")
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(metadata))
```

## Use a custom dimension

`Memory` is parsed by `Init`; you can also set `MemoryBytes` directly and leave `Memory` empty. After changing the dimension, initialize and calculate again:

```go
request.Dimension = MO.Dimension{
    Id:     MO.DimensionOpen,
    Cpu:    4000,
    Memory: "8Gi",
}
request = calculator.Init(request, conf)
calculationErr, message, families = calculator.GetCalculate()
if calculationErr != nil {
    log.Fatalf("calculation failed: %v", calculationErr)
}
```

## Access a family and parameter

After a successful calculation:

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
