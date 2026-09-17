package mongodboperatorcalculator

import (
	"fmt"
	"math"
	"strconv"
)

type Configurator struct {
	request    ConfigurationRequest
	dimension  Dimension
	families   map[string]Family
	memoryUsed float64
	cpuUsed    float64
}

func (c *Configurator) calculate() (ResponseMessage, map[string]Family, error) {
	r := c.request
	if r.Connections <= 0 {
		return ResponseMessage{ErrorexecI, "Invalid request", "connections must be greater than zero"}, nil, fmt.Errorf("connections must be greater than zero")
	}
	if r.DBType != DbTypeReplicaSet && r.DBType != DbTypeShardedCluster {
		return ResponseMessage{ErrorexecI, "Invalid request", "unsupported dbtype"}, nil, fmt.Errorf("unsupported dbtype %q", r.DBType)
	}
	if r.MongoDBVersion.Major < 7 || r.MongoDBVersion.Major > 8 {
		return ResponseMessage{ErrorexecI, "Invalid request", "only MongoDB 7 and 8 are supported"}, nil, fmt.Errorf("unsupported MongoDB version")
	}
	if r.MongoDBDedicated && r.DBType == DbTypeShardedCluster {
		return ResponseMessage{ErrorexecI, "Invalid request", "dedicated mode is only valid for replica_set"}, nil, fmt.Errorf("dedicated mode cannot be used with sharded_cluster")
	}
	if r.ProviderCostPct < 0 || r.ProviderCostPct >= 1 {
		return ResponseMessage{ErrorexecI, "Invalid request", "providercostpct must be greater than or equal to 0 and less than 1"}, nil, fmt.Errorf("invalid providercostpct")
	}
	f := c.families
	if c.dimension.MongoDBCpu == 0 {
		c.dimension.MongoDBCpu = c.dimension.Cpu
	}
	if c.dimension.MongoDBMemory == 0 {
		c.dimension.MongoDBMemory = c.dimension.MemoryBytes
	}
	if c.dimension.MongoDBCpu < MinMongodCPU {
		return ResponseMessage{ErrorexecI, "Invalid request", fmt.Sprintf("mongod CPU must be at least %dm", MinMongodCPU)}, nil, fmt.Errorf("mongod CPU below minimum")
	}
	c.sizeMongo(FamilyTypeMongoDB, c.dimension.MongoDBCpu, c.dimension.MongoDBMemory)
	if !r.MongoDBDedicated {
		c.sizeMonitor(FamilyTypeMonitor, c.dimension.MonitorCpu, c.dimension.MonitorMemory)
		if r.DBType == DbTypeShardedCluster {
			c.sizeMongos()
			c.sizeConfig()
		}
	}
	pct := c.capacityPct()
	msg := ResponseMessage{}
	if pct > 1 {
		msg = ResponseMessage{OverutilizingI, "Resources overloaded", fmt.Sprintf("safe capacity exceeded (%.0f%% estimated)", pct*100)}
		return msg, nil, fmt.Errorf("requested workload exceeds safe resources")
	}
	if pct > CloseLimitPct {
		msg = ResponseMessage{ClosetolimitI, ClosetolimitT, fmt.Sprintf("estimated capacity utilization is %.0f%%", pct*100)}
	} else {
		msg = ResponseMessage{OkI, OkT, fmt.Sprintf("estimated capacity utilization is %.0f%%", pct*100)}
	}
	return msg, f, nil
}
func (c *Configurator) sizeMongo(name string, cpu int, mem float64) {
	if cpu == 0 {
		cpu = c.dimension.Cpu
	}
	if mem == 0 {
		mem = c.dimension.MemoryBytes
	}
	cache := mem * WiredTigerCachePct
	conn := float64(c.request.Connections) * connectionMemory(c.request.LoadType.Id)
	c.memoryUsed = cache + conn + mem*0.15
	c.cpuUsed = float64(c.request.Connections) * connectionCPU(c.request.LoadType.Id)
	f := c.families[name]
	put(f, GroupNameConfiguration, "storage.wiredTiger.engineConfig.cacheSizeGB", formatGB(cache))
	put(f, GroupNameConfiguration, "net.maxIncomingConnections", strconv.Itoa(int(math.Ceil(float64(c.request.Connections)*1.15))))
	put(f, GroupNameConfiguration, "storage.engine", "wiredTiger")
	c.setResources(f, cpu, mem)
	c.setProbes(f)
	c.families[name] = f
}
func (c *Configurator) sizeConfig() {
	d := c.dimension
	mem := d.ConfigMemory
	cpu := d.ConfigCpu
	if mem == 0 {
		mem = d.MemoryBytes * 0.15
	}
	if cpu == 0 {
		cpu = max(500, int(float64(d.Cpu)*0.15))
	}
	c.sizeRole(FamilyTypeConfig, cpu, mem, false)
}
func (c *Configurator) sizeMongos() {
	d := c.dimension
	mem := d.MongosMemory
	cpu := d.MongosCpu
	if mem == 0 {
		mem = d.MemoryBytes * 0.15
	}
	if cpu == 0 {
		cpu = int(float64(d.Cpu) * 0.15)
	}
	c.sizeRole(FamilyTypeMongos, cpu, mem, true)
}
func (c *Configurator) sizeRole(name string, cpu int, mem float64, router bool) {
	f := c.families[name]
	if router {
		put(f, GroupNameConfiguration, "net.maxIncomingConnections", strconv.Itoa(int(math.Ceil(float64(c.request.Connections)*1.15))))
	}
	c.setResources(f, cpu, mem)
	c.setProbes(f)
	c.families[name] = f
}
func (c *Configurator) sizeMonitor(name string, cpu int, mem float64) {
	if cpu == 0 {
		cpu = 200
	}
	if mem == 0 {
		mem = 256 * 1024 * 1024
	}
	f := c.families[name]
	c.setResources(f, cpu, mem)
	c.setProbes(f)
	c.families[name] = f
}
func (c *Configurator) setResources(f Family, cpu int, mem float64) {
	put(f, GroupNameResources, "request_cpu", fmt.Sprintf("%dm", int(float64(cpu)*RequestPct)))
	put(f, GroupNameResources, "limit_cpu", fmt.Sprintf("%dm", cpu))
	put(f, GroupNameResources, "request_memory", strconv.FormatInt(int64(mem*RequestPct), 10))
	put(f, GroupNameResources, "limit_memory", strconv.FormatInt(int64(mem), 10))
}
func (c *Configurator) setProbes(f Family) {
	factor := 1 + float64(c.request.Connections)/1000
	put(f, "readinessProbe", "timeoutSeconds", strconv.Itoa(max(5, int(math.Ceil(15*factor)))))
	put(f, "livenessProbe", "timeoutSeconds", strconv.Itoa(max(10, int(math.Ceil(30*factor)))))
}
func (c *Configurator) capacityPct() float64 {
	d := c.dimension
	mem := d.MongoDBMemory
	if mem == 0 {
		mem = d.MemoryBytes
	}
	cpu := float64(d.MongoDBCpu)
	if cpu == 0 {
		cpu = float64(d.Cpu)
	}
	return math.Max(c.memoryUsed/mem, c.cpuUsed/cpu)
}
func connectionMemory(load int) float64 {
	return []float64{0, 256 * 1024, 384 * 1024, 512 * 1024, 768 * 1024}[validLoad(load)]
}
func connectionCPU(load int) float64 { return []float64{0, 0.5, 0.8, 1.2, 1.6}[validLoad(load)] }
func validLoad(x int) int {
	if x < 1 || x > 4 {
		return 2
	}
	return x
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
