package mongodboperatorcalculator

const (
	VERSION = "v0.2.0"

	OkI                    = 1001
	ClosetolimitI          = 2001
	OverutilizingI         = 3001
	ErrorexecI             = 5001
	ConnectionRecalculated = 6001 // retained for response compatibility; never emitted by this calculator
	ResourcesRecalculated  = 7001 // retained for response compatibility; never emitted by this calculator

	OkT             = "Execution was successful and resources match the requested workload"
	ClosetolimitT   = "Execution was successful however resources are close to saturation"
	OverutilizingT  = "Resources are not enough to cover the requested workload"
	InvalidRequestT = "Invalid request"
	// ErrorexecT is retained for source compatibility with the MySQL calculator.
	ErrorexecT = "There is an error while processing. See details: %s"

	LoadTypeMostlyReads      = 1
	LoadTypeSomeWrites       = 2
	LoadTypeEqualReadsWrites = 3
	LoadTypeHeavyWrites      = 4

	DimensionOpen = 999
	// DimensionConnectionDriven is the MySQL calculator's connection-driven
	// dimension. It is explicitly not supported here.
	DimensionConnectionDriven = 998

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
	GroupNamePBMResources   = "pbm-agent.resources"
	// GroupNameProbes is retained for source compatibility. Probe values are
	// emitted in the readinessProbe and livenessProbe groups.
	GroupNameProbes = "probes"

	ResultOutputFormatJson  = "json"
	ResultOutputFormatHuman = "human"

	// MongoDB documents 50% as a useful upper bound for container-aware cache sizing.
	WiredTigerCachePct = 0.50
	// MinCacheSizeGB is the smallest value mongod accepts for cacheSizeGB.
	MinCacheSizeGB = 0.25
	RequestPct     = 0.95
	CloseLimitPct  = 0.85

	// Smallest mongod container the calculator will recommend. A 512Mi limit
	// is the smallest that still yields the 0.25GB WiredTiger cache minimum.
	MinMongodCPU    = 250
	MinMongodMemory = 512 * mib

	// Upper bounds for user-supplied values.
	MaxDimensionCPU    = 512000
	MaxDimensionMemory = 16 * tib
	MaxConnections     = 1000000
	MaxMongosCount     = 100
	DefaultMongosCount = 3

	// Connection limits add a headroom factor on top of the requested
	// connections, plus a fixed allowance for replica-set members, operator
	// health checks, PMM, PBM, and administrative sessions.
	ConnectionHeadroomPct      = 1.15
	InternalConnectionOverhead = 100
	// Idle pool connections that every mongos keeps to each shard and config server.
	MongosPoolConnectionsPerShard  = 10
	MongosPoolConnectionsPerConfig = 20

	// Non-cache memory reserved inside mongod: a fixed part plus a share of the limit.
	MongodBaseOverheadMemory = 128 * mib
	MongodOverheadPct        = 0.05
	MongodBaseCPU            = 100
	// Every connection uses about 1MiB of thread stack, plus load-dependent working memory.
	ConnectionStackMemory = 1 * mib

	// mongos is sized so that the estimated load stays at this share of its limit.
	MongosTargetUtilization = 0.70
	MongosBaseMemory        = 256 * mib
	MongosBaseCPU           = 100
	MongosCPUFactor         = 0.5
	MongosMinCPU            = 500
	MongosMinMemory         = 512 * mib
	MongosMaxCPU            = 16000
	MongosMaxMemory         = 32 * gib

	// Shard members at or above this memory limit get at least the second config-server tier.
	ConfigLargeShardMemory = 32 * gib

	// Percona Operator for MongoDB default sidecar resources.
	PMMRequestCPU         = 300
	PMMLimitCPU           = 400
	PMMRequestMemory      = 150 * 1000 * 1000
	PMMLimitMemory        = 256 * 1000 * 1000
	PBMAgentRequestCPU    = 300
	PBMAgentCPU           = 300
	PBMAgentRequestMemory = 1 * gib
	PBMAgentMemory        = 12 * gib / 10

	mib = 1024 * 1024
	gib = 1024 * mib
	gb  = gib // retained name; the calculator uses binary gigabytes for cacheSizeGB
	tib = 1024 * gib
)

// ConfigServerTier is one config-server mongod container size.
type ConfigServerTier struct {
	Cpu    int
	Memory float64
}

// ConfigServerTiers lists config-server sizes from smallest to largest. The
// smallest tier that keeps estimated utilization below CloseLimitPct is used.
var ConfigServerTiers = []ConfigServerTier{
	{1000, 2 * gib},
	{2000, 4 * gib},
	{4000, 8 * gib},
	{8000, 16 * gib},
}

// Probe settings match the Percona Operator for MongoDB defaults.
type ProbeSettings struct {
	InitialDelaySeconds int
	TimeoutSeconds      int
	PeriodSeconds       int
	FailureThreshold    int
	SuccessThreshold    int
	StartupDelaySeconds int
}

var (
	MongodLivenessProbe       = ProbeSettings{InitialDelaySeconds: 60, TimeoutSeconds: 10, PeriodSeconds: 30, FailureThreshold: 4, StartupDelaySeconds: 7200}
	ReplicaSetReadinessProbe  = ProbeSettings{InitialDelaySeconds: 10, TimeoutSeconds: 2, PeriodSeconds: 3, FailureThreshold: 8, SuccessThreshold: 1}
	ShardedComponentReadiness = ProbeSettings{InitialDelaySeconds: 10, TimeoutSeconds: 2, PeriodSeconds: 3, FailureThreshold: 3, SuccessThreshold: 1}
	PMMLivenessProbe          = ProbeSettings{InitialDelaySeconds: 60, TimeoutSeconds: 5, PeriodSeconds: 10, FailureThreshold: 5}
	PMMReadinessProbe         = ProbeSettings{InitialDelaySeconds: 5, TimeoutSeconds: 1, PeriodSeconds: 10, FailureThreshold: 3, SuccessThreshold: 1}
)
