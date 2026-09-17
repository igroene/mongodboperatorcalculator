package mongodboperatorcalculator

const (
	VERSION = "v0.1.0"

	OkI                    = 1001
	ClosetolimitI          = 2001
	OverutilizingI         = 3001
	ErrorexecI             = 5001
	ConnectionRecalculated = 6001 // retained for response compatibility; never emitted by this calculator
	ResourcesRecalculated  = 7001 // retained for response compatibility; never emitted by this calculator

	OkT            = "Execution was successful and resources match the requested workload"
	ClosetolimitT  = "Execution was successful however resources are close to saturation"
	OverutilizingT = "Resources are not enough to cover the requested workload"
	ErrorexecT     = "There is an error while processing. See details: %s"

	LoadTypeMostlyReads      = 1
	LoadTypeSomeWrites       = 2
	LoadTypeEqualReadsWrites = 3
	LoadTypeHeavyWrites      = 4

	DimensionOpen = 999

	DbTypeReplicaSet     = "replica_set"
	DbTypeShardedCluster = "sharded_cluster"

	FamilyTypeMongoDB       = "mongodb"
	FamilyTypeMongos        = "mongos"
	FamilyTypeConfig        = "configserver"
	FamilyTypeMonitor       = "monitor"
	GroupNameConfiguration  = "configuration"
	GroupNameResources      = "resources"
	GroupNameReadinessProbe = "readinessProbe"
	GroupNameLivenessProbe  = "livenessProbe"
	// GroupNameProbes is retained for source compatibility. Probe values are
	// emitted in the readinessProbe and livenessProbe groups.
	GroupNameProbes = "probes"

	ResultOutputFormatJson  = "json"
	ResultOutputFormatHuman = "human"

	// MongoDB documents 50% as a useful upper bound for container-aware cache sizing.
	WiredTigerCachePct = 0.50
	RequestPct         = 0.95
	CloseLimitPct      = 0.85
)
