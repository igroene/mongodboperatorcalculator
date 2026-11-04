package mongodboperatorcalculator

import (
	"fmt"
	"math"
)

// validateRequest checks every request field before any value is derived
// from it, so invalid requests are echoed back unchanged.
func validateRequest(r ConfigurationRequest, conf Configuration) error {
	if r.Output != "" && r.Output != ResultOutputFormatJson && r.Output != ResultOutputFormatHuman {
		return fmt.Errorf("unsupported output %q; use %q or %q", r.Output, ResultOutputFormatJson, ResultOutputFormatHuman)
	}
	if r.DBType != DbTypeReplicaSet && r.DBType != DbTypeShardedCluster {
		return fmt.Errorf("unsupported dbtype %q; use %q or %q", r.DBType, DbTypeReplicaSet, DbTypeShardedCluster)
	}
	if r.Dimension.Id == 0 {
		return fmt.Errorf("dimension is required")
	}
	if r.LoadType.Id == 0 {
		return fmt.Errorf("loadtype is required")
	}
	if conf.GetLoadByID(r.LoadType.Id).Id == 0 {
		return fmt.Errorf("unsupported loadtype %d; use 1 through %d", r.LoadType.Id, len(conf.LoadType))
	}
	if r.Connections <= 0 {
		return fmt.Errorf("connections must be greater than zero")
	}
	if r.Connections > MaxConnections {
		return fmt.Errorf("connections must be at most %d", MaxConnections)
	}
	if err := validateVersion(r.MongoDBVersion, conf.MongoDBVersions); err != nil {
		return err
	}
	if math.IsNaN(r.ProviderCostPct) || r.ProviderCostPct < 0 || r.ProviderCostPct >= 1 {
		return fmt.Errorf("providercostpct must be greater than or equal to 0 and less than 1")
	}
	if r.MongoDBDedicated && r.DBType == DbTypeShardedCluster {
		return fmt.Errorf("dedicated mode is only valid for replica_set")
	}
	if r.Mongos.Count != 0 {
		if r.DBType != DbTypeShardedCluster {
			return fmt.Errorf("mongos.count is only valid for sharded_cluster")
		}
		if r.Mongos.Count < 0 || r.Mongos.Count > MaxMongosCount {
			return fmt.Errorf("mongos.count must be between 1 and %d", MaxMongosCount)
		}
	}
	return nil
}

func validateVersion(v Version, supported Versions) error {
	if v.Minor < 0 || v.Minor > 99 || v.Patch < 0 || v.Patch > 99 ||
		compareVersions(v, supported.Min) < 0 || compareVersions(v, supported.Max) > 0 {
		return fmt.Errorf("unsupported MongoDB version %d.%d.%d; supported range is %d.%d.%d to %d.%d.%d",
			v.Major, v.Minor, v.Patch,
			supported.Min.Major, supported.Min.Minor, supported.Min.Patch,
			supported.Max.Major, supported.Max.Minor, supported.Max.Patch)
	}
	return nil
}

func compareVersions(a, b Version) int {
	for _, d := range []int{a.Major - b.Major, a.Minor - b.Minor, a.Patch - b.Patch} {
		if d != 0 {
			return d
		}
	}
	return 0
}
