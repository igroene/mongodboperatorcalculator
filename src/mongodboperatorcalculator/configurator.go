package mongodboperatorcalculator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// utilization is the estimated share of a container's CPU and memory limit
// used by the requested workload.
type utilization struct {
	component string
	cpu       float64
	memory    float64
	// autoSized components are sized by the calculator to fit the workload.
	// They only drive the headline utilization once even their largest
	// size is close to saturation.
	autoSized bool
}

func (u utilization) pct() float64 { return math.Max(u.cpu, u.memory) }

// resourcePlan is the result of fitting the requested workload into the
// selected dimension and the derived sharded-cluster components.
type resourcePlan struct {
	dimension         Dimension
	mongosCount       int
	mongosConnections int
	configConnections int
	components        []utilization
	// fitErr is set when the dimension cannot hold mongod plus its sidecars.
	fitErr error
}

// planResources computes every container size and utilization estimate. It
// expects a validated request with a resolved dimension.
func planResources(r ConfigurationRequest) resourcePlan {
	d := r.Dimension
	p := resourcePlan{}
	totalCPU := int(float64(d.Cpu) * (1 - r.ProviderCostPct))
	totalMemory := math.Floor(d.MemoryBytes * (1 - r.ProviderCostPct))

	sidecars := []string{}
	if !r.MongoDBDedicated {
		d.MonitorCpu, d.MonitorMemory = PMMLimitCPU, PMMLimitMemory
		sidecars = append(sidecars, fmt.Sprintf("PMM %dm/%s", PMMLimitCPU, formatBytes(PMMLimitMemory)))
	}
	if r.Backup.Enabled {
		d.BackupCpu, d.BackupMemory = PBMAgentCPU, PBMAgentMemory
		sidecars = append(sidecars, fmt.Sprintf("PBM %dm/%s", PBMAgentCPU, formatBytes(PBMAgentMemory)))
	}
	mongodCPU := totalCPU - d.MonitorCpu - d.BackupCpu
	mongodMemory := floorMiB(totalMemory - d.MonitorMemory - d.BackupMemory)
	if mongodCPU < MinMongodCPU || mongodMemory < MinMongodMemory {
		budget := fmt.Sprintf("%dm/%s", totalCPU, formatBytes(totalMemory))
		hint := "choose a larger dimension"
		if !r.MongoDBDedicated && r.DBType == DbTypeReplicaSet {
			hint += " or set mongodbdedicated"
		}
		after := ""
		if len(sidecars) > 0 {
			after = " after " + strings.Join(sidecars, " and ")
		}
		p.fitErr = fmt.Errorf("dimension %s (%s) leaves %dm CPU and %s memory for mongod%s; the minimum is %dm and %s; %s",
			d.Name, budget, max(0, mongodCPU), formatBytes(math.Max(0, mongodMemory)), after,
			MinMongodCPU, formatBytes(MinMongodMemory), hint)
	}
	d.MongoDBCpu = max(0, mongodCPU)
	d.MongoDBMemory = math.Max(0, mongodMemory)

	load := r.LoadType.Id
	shardName := "mongod"
	if r.DBType == DbTypeShardedCluster {
		shardName = "shard mongod"
	}
	idlePool := 0
	if r.DBType == DbTypeShardedCluster {
		p.mongosCount = r.Mongos.Count
		if p.mongosCount == 0 {
			p.mongosCount = DefaultMongosCount
		}
		idlePool = p.mongosCount * MongosPoolConnectionsPerShard
	}
	if p.fitErr == nil {
		p.components = append(p.components, mongodUtilization(shardName, d.MongoDBCpu, d.MongoDBMemory, r.Connections, idlePool, load))
	}

	if r.DBType == DbTypeShardedCluster {
		p.mongosConnections = int(math.Ceil(float64(r.Connections) / float64(p.mongosCount)))
		var mongosUse utilization
		d.MongosCpu, d.MongosMemory, mongosUse = sizeMongos(p.mongosConnections, load)
		p.configConnections = p.mongosCount * MongosPoolConnectionsPerConfig
		var configUse utilization
		d.ConfigCpu, d.ConfigMemory, configUse = sizeConfigServer(p.configConnections)
		p.components = append(p.components, configUse, mongosUse)
	}
	p.dimension = d
	return p
}

// mongodUtilization estimates how much of a mongod container the workload
// uses. The WiredTiger cache is an allocation, not load, so memory pressure
// is measured against the memory left outside the cache. Every supplied client
// connection gets the full workload allowance, including currently idle sockets.
// Additional internal idle pools get only the baseline stack allowance.
func mongodUtilization(name string, cpu int, memory float64, connections, idleConnections, load int) utilization {
	nonCache := memory - cacheBytes(memory)
	needMemory := MongodBaseOverheadMemory + MongodOverheadPct*memory +
		float64(connections)*(ConnectionStackMemory+connectionWorkMemory(load)) +
		float64(idleConnections)*ConnectionStackMemory
	needCPU := MongodBaseCPU + float64(connections)*connectionCPU(load)
	return utilization{component: name, cpu: needCPU / float64(cpu), memory: needMemory / nonCache}
}

// sizeMongos sizes one mongos so the estimated load sits at
// MongosTargetUtilization, within the mongos minimum and maximum.
func sizeMongos(connections, load int) (int, float64, utilization) {
	needCPU := MongosBaseCPU + float64(connections)*connectionCPU(load)*MongosCPUFactor
	needMemory := MongosBaseMemory + float64(connections)*(ConnectionStackMemory+connectionWorkMemory(load)/2)
	cpu := int(roundUp(needCPU/MongosTargetUtilization, 100))
	cpu = min(max(cpu, MongosMinCPU), MongosMaxCPU)
	memory := roundUp(needMemory/MongosTargetUtilization, 64*mib)
	memory = math.Min(math.Max(memory, MongosMinMemory), MongosMaxMemory)
	return cpu, memory, utilization{component: "mongos", cpu: needCPU / float64(cpu), memory: needMemory / memory, autoSized: true}
}

// sizeConfigServer picks the smallest config-server tier that keeps the
// modelled mongos load below CloseLimitPct. Metadata volume is not an input,
// so metadata-heavy clusters may need a larger tier than this returns.
func sizeConfigServer(connections int) (int, float64, utilization) {
	var tier ConfigServerTier
	var use utilization
	for _, tier = range ConfigServerTiers {
		use = mongodUtilization("config server", tier.Cpu, tier.Memory, connections, 0, LoadTypeMostlyReads)
		if use.pct() <= CloseLimitPct {
			break
		}
	}
	use.autoSized = true
	return tier.Cpu, tier.Memory, use
}

func connectionWorkMemory(load int) float64 {
	return []float64{0, 0.25 * mib, 0.5 * mib, 1 * mib, 1.5 * mib}[load]
}

// connectionCPU is the assumed average millicores per client connection under
// the selected workload profile. No active/idle discount is applied.
func connectionCPU(load int) float64 { return []float64{0, 0.5, 1, 2, 3}[load] }

// operatorCacheSizeGB reproduces how the operator derives the
// --wiredTigerCacheSizeGB argument from the mongod memory limit and the
// default cacheSizeRatio, including its 0.25GB floor and two-decimal format.
func operatorCacheSizeGB(memory float64) string {
	size := math.Floor(WiredTigerCacheRatio*(memory-gib)) / gib
	return fmt.Sprintf("%.2f", math.Max(size, MinCacheSizeGB))
}

// cacheBytes is the WiredTiger cache mongod actually runs with under the operator.
func cacheBytes(memory float64) float64 {
	gb, _ := strconv.ParseFloat(operatorCacheSizeGB(memory), 64)
	return gb * gib
}

func maxIncomingConnections(expected, extra int) int {
	return int(math.Ceil(float64(expected)*ConnectionHeadroomPct)) + extra + InternalConnectionOverhead
}

type Configurator struct {
	request  ConfigurationRequest
	plan     resourcePlan
	families map[string]Family
}

func (c *Configurator) calculate() (ResponseMessage, map[string]Family, error) {
	if c.plan.fitErr != nil {
		return ResponseMessage{OverutilizingI, OverutilizingT, c.plan.fitErr.Error()}, nil, c.plan.fitErr
	}
	worst := 0.0
	for _, u := range c.plan.components {
		if math.IsNaN(u.pct()) || math.IsInf(u.pct(), 0) {
			err := fmt.Errorf("could not estimate %s utilization", u.component)
			return ResponseMessage{ErrorexecI, InvalidRequestT, err.Error()}, nil, err
		}
		if !u.autoSized || u.pct() > CloseLimitPct {
			worst = math.Max(worst, u.pct())
		}
	}
	if worst > 1 {
		text := fmt.Sprintf("estimated capacity exceeded (%s): %s", c.connectionAssumption(), c.overloadDetails())
		return ResponseMessage{OverutilizingI, OverutilizingT, text}, nil, fmt.Errorf("requested connection count exceeds resources under the selected workload assumptions")
	}
	c.populate()
	text := fmt.Sprintf("estimated capacity utilization is %.0f%% (%s); %s", worst*100, c.utilizationDetails(), c.connectionAssumption())
	if worst > CloseLimitPct {
		return ResponseMessage{ClosetolimitI, ClosetolimitT, text}, c.families, nil
	}
	return ResponseMessage{OkI, OkT, text}, c.families, nil
}

func (c *Configurator) connectionAssumption() string {
	return fmt.Sprintf("assumes all %d open client connections use the selected workload profile", c.request.Connections)
}

func (c *Configurator) utilizationDetails() string {
	parts := []string{}
	for _, u := range c.plan.components {
		if len(c.plan.components) == 1 {
			return fmt.Sprintf("CPU %.0f%%, memory %.0f%%", u.cpu*100, u.memory*100)
		}
		parts = append(parts, fmt.Sprintf("%s %.0f%%", u.component, u.pct()*100))
	}
	return strings.Join(parts, ", ")
}

func (c *Configurator) overloadDetails() string {
	parts := []string{}
	for _, u := range c.plan.components {
		if u.pct() <= 1 {
			continue
		}
		part := fmt.Sprintf("%s at %.0f%% (CPU %.0f%%, memory %.0f%%)", u.component, u.pct()*100, u.cpu*100, u.memory*100)
		switch u.component {
		case "mongos":
			part += "; increase mongos.count"
		case "config server":
			part += "; the largest config-server tier is not enough"
		default:
			part += "; choose a larger dimension or reduce connections"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func (c *Configurator) populate() {
	r := c.request
	d := c.plan.dimension
	mongod := c.families[FamilyTypeMongoDB]
	extra := 0
	if r.DBType == DbTypeShardedCluster {
		extra = c.plan.mongosCount * MongosPoolConnectionsPerShard
	}
	c.setMongodConfiguration(mongod, d.MongoDBMemory, maxIncomingConnections(r.Connections, extra))
	c.setResources(mongod, GroupNameResources, d.MongoDBCpu, d.MongoDBMemory)
	setProbes(mongod, "", ReplicaSetReadinessProbe, MongodLivenessProbe)
	if r.Backup.Enabled {
		c.setPBMResources(mongod)
	}

	if !r.MongoDBDedicated {
		monitor := c.families[FamilyTypeMonitor]
		prefixes := []string{""}
		if r.DBType == DbTypeShardedCluster {
			prefixes = []string{"mongod.", "configserver.", "mongos."}
		}
		for _, prefix := range prefixes {
			c.setPMMResources(monitor, prefix+GroupNameResources)
			writeProbe(monitor, prefix+GroupNameLivenessProbe, PMMLivenessProbe)
		}
	}

	if r.DBType == DbTypeShardedCluster {
		config := c.families[FamilyTypeConfig]
		c.setMongodConfiguration(config, d.ConfigMemory, maxIncomingConnections(c.plan.configConnections, 0))
		c.setResources(config, GroupNameResources, d.ConfigCpu, d.ConfigMemory)
		setProbes(config, "", ConfigServerReadinessProbe, MongodLivenessProbe)
		if r.Backup.Enabled {
			c.setPBMResources(config)
		}

		mongos := c.families[FamilyTypeMongos]
		put(mongos, GroupNameConfiguration, "net.maxIncomingConnections", strconv.Itoa(maxIncomingConnections(c.plan.mongosConnections, 0)))
		c.setResources(mongos, GroupNameResources, d.MongosCpu, d.MongosMemory)
		setProbes(mongos, "", MongosReadinessProbe, MongosLivenessProbe)
	}
}

// setMongodConfiguration writes the mongod configuration and the storage
// settings. cacheSizeGB is the value the operator itself passes to mongod as
// --wiredTigerCacheSizeGB, derived from cacheSizeRatio, so it is consistent
// with what the pod will run but setting it in configuration has no effect.
func (c *Configurator) setMongodConfiguration(f Family, memory float64, maxConnections int) {
	put(f, GroupNameConfiguration, "storage.wiredTiger.engineConfig.cacheSizeGB", operatorCacheSizeGB(memory))
	put(f, GroupNameConfiguration, "net.maxIncomingConnections", strconv.Itoa(maxConnections))
	put(f, GroupNameConfiguration, "storage.engine", "wiredTiger")
	put(f, GroupNameStorage, "engine", "wiredTiger")
	put(f, GroupNameStorage, "wiredTiger.engineConfig.cacheSizeRatio", strconv.FormatFloat(WiredTigerCacheRatio, 'f', -1, 64))
}

// setResources writes limits and requests. Requests are RequestPct of the
// limit, rounded down to whole millicores and MiB, unless Guaranteed QoS is
// requested.
func (c *Configurator) setResources(f Family, group string, cpu int, memory float64) {
	requestCPU, requestMemory := int(float64(cpu)*RequestPct), floorMiB(memory*RequestPct)
	if c.request.GuaranteedQoS {
		requestCPU, requestMemory = cpu, memory
	}
	writeResources(f, group, requestCPU, cpu, requestMemory, memory)
}
func (c *Configurator) setPMMResources(f Family, group string) {
	requestCPU, requestMemory := PMMRequestCPU, float64(PMMRequestMemory)
	if c.request.GuaranteedQoS {
		requestCPU, requestMemory = PMMLimitCPU, PMMLimitMemory
	}
	writeResources(f, group, requestCPU, PMMLimitCPU, requestMemory, PMMLimitMemory)
}
func (c *Configurator) setPBMResources(f Family) {
	requestCPU, requestMemory := PBMAgentRequestCPU, float64(PBMAgentRequestMemory)
	if c.request.GuaranteedQoS {
		requestCPU, requestMemory = PBMAgentCPU, PBMAgentMemory
	}
	writeResources(f, GroupNamePBMResources, requestCPU, PBMAgentCPU, requestMemory, PBMAgentMemory)
}
func writeResources(f Family, group string, requestCPU, limitCPU int, requestMemory, limitMemory float64) {
	put(f, group, "request_cpu", fmt.Sprintf("%dm", requestCPU))
	put(f, group, "limit_cpu", fmt.Sprintf("%dm", limitCPU))
	put(f, group, "request_memory", strconv.FormatInt(int64(requestMemory), 10))
	put(f, group, "limit_memory", strconv.FormatInt(int64(limitMemory), 10))
}

func setProbes(f Family, prefix string, readiness, liveness ProbeSettings) {
	writeProbe(f, prefix+GroupNameReadinessProbe, readiness)
	writeProbe(f, prefix+GroupNameLivenessProbe, liveness)
}
func writeProbe(f Family, group string, p ProbeSettings) {
	put(f, group, "initialDelaySeconds", strconv.Itoa(p.InitialDelaySeconds))
	put(f, group, "timeoutSeconds", strconv.Itoa(p.TimeoutSeconds))
	put(f, group, "periodSeconds", strconv.Itoa(p.PeriodSeconds))
	put(f, group, "failureThreshold", strconv.Itoa(p.FailureThreshold))
	if p.SuccessThreshold > 0 {
		put(f, group, "successThreshold", strconv.Itoa(p.SuccessThreshold))
	}
	if p.StartupDelaySeconds > 0 {
		put(f, group, "startupDelaySeconds", strconv.Itoa(p.StartupDelaySeconds))
	}
}

func floorMiB(bytes float64) float64  { return math.Floor(bytes/mib) * mib }
func roundUp(x, step float64) float64 { return math.Ceil(x/step) * step }

// formatBytes renders bytes for messages using the largest exact-enough binary unit.
func formatBytes(bytes float64) string {
	switch {
	case bytes >= gib:
		return strconv.FormatFloat(math.Round(bytes/gib*100)/100, 'f', -1, 64) + "Gi"
	case bytes >= mib:
		return strconv.FormatFloat(math.Round(bytes/mib), 'f', -1, 64) + "Mi"
	default:
		return strconv.FormatFloat(bytes, 'f', -1, 64) + "B"
	}
}
