package mongodboperatorcalculator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type MongoDBOperatorCalculator struct {
	IncomingRequest ConfigurationRequest
	Conf            Configuration
	configurator    Configurator
	initError       error
}

func (m *MongoDBOperatorCalculator) Init(req ConfigurationRequest, conf Configuration) ConfigurationRequest {
	m.IncomingRequest = req
	m.Conf = conf
	if m.IncomingRequest.Dimension.Id == DimensionOpen && m.IncomingRequest.Dimension.MemoryBytes == 0 {
		m.IncomingRequest.Dimension.MemoryBytes, m.initError = m.IncomingRequest.Dimension.ConvertMemoryToBytes(m.IncomingRequest.Dimension.Memory)
	}
	d, e := conf.ResolveDimension(m.IncomingRequest.Dimension)
	if e != nil && m.initError == nil {
		m.initError = e
	}
	if e == nil {
		if req.ProviderCostPct > 0 {
			d.Cpu = int(float64(d.Cpu) * (1 - req.ProviderCostPct))
			d.MemoryBytes *= 1 - req.ProviderCostPct
			d.Memory = fmt.Sprintf("%dB", int64(d.MemoryBytes))
			d.MongoDBCpu = int(float64(d.MongoDBCpu) * (1 - req.ProviderCostPct))
			d.MongoDBMemory *= 1 - req.ProviderCostPct
			d.MonitorCpu = int(float64(d.MonitorCpu) * (1 - req.ProviderCostPct))
			d.MonitorMemory *= 1 - req.ProviderCostPct
		}
		if m.IncomingRequest.DBType == DbTypeShardedCluster {
			// A sharded request is a planning envelope for one shard member,
			// one config-server member, one mongos, and PMM.
			d.MongoDBCpu = int(float64(d.Cpu) * 0.70)
			d.ConfigCpu = int(float64(d.Cpu) * 0.15)
			d.MongosCpu = int(float64(d.Cpu) * 0.10)
			d.MonitorCpu = d.Cpu - d.MongoDBCpu - d.ConfigCpu - d.MongosCpu
			d.MongoDBMemory = d.MemoryBytes * 0.70
			d.ConfigMemory = d.MemoryBytes * 0.15
			d.MongosMemory = d.MemoryBytes * 0.10
			d.MonitorMemory = d.MemoryBytes - d.MongoDBMemory - d.ConfigMemory - d.MongosMemory
		} else if m.IncomingRequest.MongoDBDedicated {
			d.MongoDBCpu = d.Cpu
			d.MongoDBMemory = d.MemoryBytes
		}
		m.IncomingRequest.Dimension = d
	}
	m.IncomingRequest.LoadType = conf.GetLoadByID(req.LoadType.Id)
	return m.IncomingRequest
}
func (m *MongoDBOperatorCalculator) GetSupportedLayouts() Configuration {
	var c Configuration
	c.Init()
	return c
}
func (m *MongoDBOperatorCalculator) GetCalculate() (error, ResponseMessage, map[string]Family) {
	if m.initError != nil {
		return m.initError, ResponseMessage{ErrorexecI, "Invalid request", m.initError.Error()}, map[string]Family{}
	}
	if m.IncomingRequest.Dimension.Id == 0 || m.IncomingRequest.LoadType.Id == 0 {
		return fmt.Errorf("dimension and loadtype are required"), ResponseMessage{ErrorexecI, "Invalid request", "dimension and loadtype are required"}, map[string]Family{}
	}
	m.configurator = Configurator{request: m.IncomingRequest, dimension: m.IncomingRequest.Dimension, families: m.Conf.Families(m.IncomingRequest)}
	msg, f, e := m.configurator.calculate()
	if f == nil {
		f = map[string]Family{}
	}
	return e, msg, f
}
func (m *MongoDBOperatorCalculator) GetJSONOutput(msg ResponseMessage, req ConfigurationRequest, f map[string]Family) (bytes.Buffer, error) {
	var b bytes.Buffer
	x := struct {
		Message  ResponseMessage      `json:"message"`
		Incoming ConfigurationRequest `json:"incoming"`
		Answer   map[string]Family    `json:"answer"`
	}{msg, req, f}
	out, e := json.MarshalIndent(x, "", "  ")
	b.Write(out)
	return b, e
}
func (m *MongoDBOperatorCalculator) GetHumanOutput(msg ResponseMessage, req ConfigurationRequest, f map[string]Family) (bytes.Buffer, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "[message]\nname = %s\ntype = %d\ntext = %s\n", msg.MName, msg.MType, msg.MText)
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		section := k
		if k == FamilyTypeMongoDB {
			section = "mongod"
		} else if k == FamilyTypeConfig {
			section = "configserver.mongod"
		} else if k == FamilyTypeMonitor {
			section = "pmm-client"
		} else if k == FamilyTypeMongos {
			section = "mongos"
		}
		groups := f[k].Groups
		gkeys := make([]string, 0, len(groups))
		for x := range groups {
			gkeys = append(gkeys, x)
		}
		sort.Strings(gkeys)
		for _, x := range gkeys {
			fmt.Fprintf(&b, "[%s.%s]\n", section, x)
			pkeys := make([]string, 0, len(groups[x].Parameters))
			for p := range groups[x].Parameters {
				pkeys = append(pkeys, p)
			}
			sort.Strings(pkeys)
			for _, p := range pkeys {
				fmt.Fprintf(&b, "%s = %s\n", p, groups[x].Parameters[p].Value)
			}
		}
	}
	return b, nil
}
func (m *MongoDBOperatorCalculator) GetFamily(name string) (Family, error) {
	f, ok := m.configurator.families[name]
	if !ok {
		return Family{}, fmt.Errorf("invalid family %q", name)
	}
	return f, nil
}
