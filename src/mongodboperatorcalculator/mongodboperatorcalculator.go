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
	plan            resourcePlan
	initError       error
}

// Init validates and resolves the request. Validation errors are reported by
// GetCalculate; the returned request is then the original, unmodified input.
func (m *MongoDBOperatorCalculator) Init(req ConfigurationRequest, conf Configuration) ConfigurationRequest {
	m.IncomingRequest = req
	m.Conf = conf
	m.initError = nil
	m.configurator = Configurator{}
	if err := validateRequest(req, conf); err != nil {
		m.initError = err
		return req
	}
	resolved := req
	if resolved.Output == "" {
		resolved.Output = ResultOutputFormatJson
	}
	if resolved.Dimension.Id == DimensionOpen && resolved.Dimension.Memory != "" {
		bytes, err := resolved.Dimension.ConvertMemoryToBytes(resolved.Dimension.Memory)
		if err != nil {
			m.initError = err
			return req
		}
		resolved.Dimension.MemoryBytes = bytes
	}
	d, err := conf.ResolveDimension(resolved.Dimension)
	if err != nil {
		m.initError = err
		return req
	}
	resolved.Dimension = d
	resolved.LoadType = conf.GetLoadByID(req.LoadType.Id)
	if resolved.DBType == DbTypeShardedCluster && resolved.Mongos.Count == 0 {
		resolved.Mongos.Count = DefaultMongosCount
	}
	m.plan = planResources(resolved)
	resolved.Dimension = m.plan.dimension
	m.IncomingRequest = resolved
	return resolved
}
func (m *MongoDBOperatorCalculator) GetSupportedLayouts() Configuration {
	var c Configuration
	c.Init()
	return c
}
func (m *MongoDBOperatorCalculator) GetCalculate() (error, ResponseMessage, map[string]Family) {
	if m.initError != nil {
		return m.initError, ResponseMessage{ErrorexecI, InvalidRequestT, m.initError.Error()}, map[string]Family{}
	}
	if m.IncomingRequest.Dimension.Id == 0 {
		err := fmt.Errorf("Init must be called before GetCalculate")
		return err, ResponseMessage{ErrorexecI, InvalidRequestT, err.Error()}, map[string]Family{}
	}
	m.configurator = Configurator{request: m.IncomingRequest, plan: m.plan, families: m.Conf.Families(m.IncomingRequest)}
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

// GetHumanOutput renders INI-style sections. Empty groups are omitted and
// sections are separated by a blank line.
func (m *MongoDBOperatorCalculator) GetHumanOutput(msg ResponseMessage, req ConfigurationRequest, f map[string]Family) (bytes.Buffer, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "[message]\nname = %s\ntype = %d\ntext = %s\n\n", msg.MName, msg.MType, msg.MText)
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sections := map[string]string{FamilyTypeMongoDB: "mongod", FamilyTypeConfig: "configserver.mongod", FamilyTypeMonitor: "pmm-client", FamilyTypeMongos: "mongos"}
	for _, k := range keys {
		section, ok := sections[k]
		if !ok {
			section = k
		}
		groups := f[k].Groups
		gkeys := make([]string, 0, len(groups))
		for x := range groups {
			gkeys = append(gkeys, x)
		}
		sort.Strings(gkeys)
		for _, x := range gkeys {
			if len(groups[x].Parameters) == 0 {
				continue
			}
			fmt.Fprintf(&b, "[%s.%s]\n", section, x)
			pkeys := make([]string, 0, len(groups[x].Parameters))
			for p := range groups[x].Parameters {
				pkeys = append(pkeys, p)
			}
			sort.Strings(pkeys)
			for _, p := range pkeys {
				fmt.Fprintf(&b, "%s = %s\n", p, groups[x].Parameters[p].Value)
			}
			b.WriteString("\n")
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
