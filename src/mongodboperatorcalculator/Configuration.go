package mongodboperatorcalculator

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}
type Versions struct {
	Min Version `json:"min"`
	Max Version `json:"max"`
}
type ResponseMessage struct {
	MType int    `json:"type"`
	MName string `json:"name"`
	MText string `json:"text"`
}

// Dimension describes the resources of one shard or replica-set mongod pod.
// Cpu and Memory are the pod totals as requested. The allocation fields are
// filled in by the calculator: mongod, PMM, and PBM containers share the pod
// budget, while mongos and config-server values describe separate pods.
type Dimension struct {
	Id            int     `json:"id"`
	Name          string  `json:"name"`
	Cpu           int     `json:"cpu"`
	Memory        string  `json:"memory"`
	MemoryBytes   float64 `json:"-"`
	MongoDBCpu    int     `json:"mongodbCpu"`
	MonitorCpu    int     `json:"monitorCpu"`
	BackupCpu     int     `json:"backupCpu"`
	MongosCpu     int     `json:"mongosCpu"`
	ConfigCpu     int     `json:"configCpu"`
	MongoDBMemory float64 `json:"mongodbMemory"`
	MonitorMemory float64 `json:"monitorMemory"`
	BackupMemory  float64 `json:"backupMemory"`
	MongosMemory  float64 `json:"mongosMemory"`
	ConfigMemory  float64 `json:"configMemory"`
}
type LoadType struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Example string `json:"example"`
}
type Configuration struct {
	DBType          []string    `json:"dbtype"`
	Dimension       []Dimension `json:"dimension"`
	LoadType        []LoadType  `json:"loadtype"`
	Connections     []int       `json:"connections"`
	Output          []string    `json:"output"`
	MongoDBVersions Versions    `json:"mongodbversions"`
}
type ConfigurationRequest struct {
	DBType           string              `json:"dbtype"`
	Dimension        Dimension           `json:"dimension"`
	LoadType         LoadType            `json:"loadtype"`
	Connections      int                 `json:"connections"`
	Output           string              `json:"output"`
	MongoDBVersion   Version             `json:"mongodbversion"`
	ProviderCostPct  float64             `json:"providercostpct"`
	MongoDBDedicated bool                `json:"mongodbdedicated"`
	GuaranteedQoS    bool                `json:"guaranteedqos"`
	Backup           BackupConfiguration `json:"backup"`
	Mongos           MongosConfiguration `json:"mongos"`
}
type BackupConfiguration struct {
	Enabled bool `json:"enabled"`
}

// MongosConfiguration describes the mongos tier of a sharded cluster. Count
// is the number of mongos pods that share the requested client connections.
type MongosConfiguration struct {
	Count int `json:"count"`
}
type Parameter struct {
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	Default  string   `json:"-"`
	Min      uint64   `json:"-"`
	Max      uint64   `json:"-"`
	Versions Versions `json:"-"`
}

func (p Parameter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{p.Name, p.Value})
}

type GroupObj struct {
	Name       string               `json:"name"`
	Parameters map[string]Parameter `json:"parameters"`
}
type Family struct {
	Name   string              `json:"name"`
	Groups map[string]GroupObj `json:"groups"`
}

func (c *Configuration) Init() {
	c.DBType = []string{DbTypeReplicaSet, DbTypeShardedCluster}
	c.Output = []string{ResultOutputFormatHuman, ResultOutputFormatJson}
	c.Connections = []int{50, 100, 200, 500, 1000, 2000}
	c.MongoDBVersions = Versions{Version{7, 0, 0}, Version{8, 99, 99}}
	c.Dimension = []Dimension{
		{Id: 1, Name: "XSmall", Cpu: 600, Memory: "1Gi", MemoryBytes: 1 * gib},
		{Id: 2, Name: "Small", Cpu: 800, Memory: "2Gi", MemoryBytes: 2 * gib},
		{Id: 3, Name: "Medium", Cpu: 1000, Memory: "4Gi", MemoryBytes: 4 * gib},
		{Id: 4, Name: "Large", Cpu: 2000, Memory: "8Gi", MemoryBytes: 8 * gib},
		{Id: 5, Name: "XLarge", Cpu: 4000, Memory: "16Gi", MemoryBytes: 16 * gib},
		{Id: 6, Name: "2XLarge", Cpu: 8000, Memory: "32Gi", MemoryBytes: 32 * gib},
		{Id: 7, Name: "4XLarge", Cpu: 16000, Memory: "64Gi", MemoryBytes: 64 * gib},
		{Id: 8, Name: "8XLarge", Cpu: 32000, Memory: "128Gi", MemoryBytes: 128 * gib},
		{Id: 9, Name: "12XLarge", Cpu: 64000, Memory: "256Gi", MemoryBytes: 256 * gib},
		{Id: 10, Name: "16XLarge", Cpu: 128000, Memory: "512Gi", MemoryBytes: 512 * gib},
		{Id: DimensionOpen, Name: "Open request by resources", Memory: "0"},
	}
	c.LoadType = []LoadType{{1, "Mainly Reads", "Read-heavy workload"}, {2, "Light OLTP", "Mixed workload with moderate writes"}, {3, "Heavy OLTP", "Highly concurrent mixed workload"}, {4, "Mainly Writes", "Write-heavy workload"}}
}

func (c Configuration) GetDimensionByID(id int) Dimension {
	for _, d := range c.Dimension {
		if d.Id == id {
			return d
		}
	}
	return Dimension{}
}
func (c Configuration) GetLoadByID(id int) LoadType {
	for _, l := range c.LoadType {
		if l.Id == id {
			return l
		}
	}
	return LoadType{}
}

var memoryQuantityPattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]*)?|\.[0-9]+)\s*([A-Za-z]*)$`)

// memoryUnits follows Kubernetes quantity semantics: Ki/Mi/Gi/Ti/Pi are
// binary, k/M/G/T/P are decimal. KiB-style and KB-style spellings are accepted
// as aliases of the binary and decimal forms. Lookups are case-insensitive.
var memoryUnits = map[string]float64{
	"": 1, "b": 1,
	"k": 1e3, "kb": 1e3, "ki": 1 << 10, "kib": 1 << 10,
	"m": 1e6, "mb": 1e6, "mi": 1 << 20, "mib": 1 << 20,
	"g": 1e9, "gb": 1e9, "gi": 1 << 30, "gib": 1 << 30,
	"t": 1e12, "tb": 1e12, "ti": 1 << 40, "tib": 1 << 40,
	"p": 1e15, "pb": 1e15, "pi": 1 << 50, "pib": 1 << 50,
}

// ConvertMemoryToBytes parses a memory quantity such as 8Gi, 16G, 4096Mi, or
// 8589934592 and returns whole bytes.
func (d Dimension) ConvertMemoryToBytes(s string) (float64, error) {
	s = strings.TrimSpace(s)
	match := memoryQuantityPattern.FindStringSubmatch(s)
	if match == nil {
		return 0, fmt.Errorf("invalid memory quantity %q: expected a number followed by an optional unit such as Gi, Mi, G, or M", s)
	}
	if match[2] == "m" {
		return 0, fmt.Errorf("invalid memory quantity %q: lowercase m means milli; use Mi or M", s)
	}
	multiplier, ok := memoryUnits[strings.ToLower(match[2])]
	if !ok {
		return 0, fmt.Errorf("invalid memory quantity %q: unknown unit %q", s, match[2])
	}
	n, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory quantity %q", s)
	}
	bytes := math.Floor(n * multiplier)
	if bytes <= 0 || bytes > MaxDimensionMemory {
		return 0, fmt.Errorf("memory %q must be greater than 0 and at most 16Ti", s)
	}
	return bytes, nil
}

// ResolveDimension returns the catalog dimension for a predefined ID or the
// validated user-supplied resources for the open dimension. Allocation fields
// are reset; the calculator fills them in.
func (c Configuration) ResolveDimension(d Dimension) (Dimension, error) {
	if d.Id == DimensionConnectionDriven {
		return d, fmt.Errorf("dimension %d (connection-driven sizing) is not supported", d.Id)
	}
	if d.Id != DimensionOpen {
		x := c.GetDimensionByID(d.Id)
		if x.Id == 0 {
			return d, fmt.Errorf("unknown dimension %d", d.Id)
		}
		if d.Cpu != 0 && d.Cpu != x.Cpu {
			return d, fmt.Errorf("cpu can only be set with dimension %d; dimension %d has %dm", DimensionOpen, x.Id, x.Cpu)
		}
		if d.Memory != "" {
			bytes, err := d.ConvertMemoryToBytes(d.Memory)
			if err != nil || bytes != x.MemoryBytes {
				return d, fmt.Errorf("memory can only be set with dimension %d; dimension %d has %s", DimensionOpen, x.Id, x.Memory)
			}
		}
		return x, nil
	}
	if d.Cpu <= 0 || d.Cpu > MaxDimensionCPU {
		return d, fmt.Errorf("open dimension requires cpu between 1 and %d millicores", MaxDimensionCPU)
	}
	if math.IsNaN(d.MemoryBytes) || d.MemoryBytes <= 0 || d.MemoryBytes > MaxDimensionMemory {
		return d, fmt.Errorf("open dimension requires memory greater than 0 and at most 16Ti")
	}
	return Dimension{Id: DimensionOpen, Name: "Open request by resources", Cpu: d.Cpu, Memory: d.Memory, MemoryBytes: math.Floor(d.MemoryBytes)}, nil
}

func (c Configuration) Families(req ConfigurationRequest) map[string]Family {
	f := map[string]Family{}
	f[FamilyTypeMongoDB] = newFamily("mongod", true)
	if !req.MongoDBDedicated {
		f[FamilyTypeMonitor] = Family{Name: "pmm-client", Groups: map[string]GroupObj{}}
	}
	if req.DBType == DbTypeShardedCluster {
		f[FamilyTypeMongos] = newFamily("mongos", true)
		f[FamilyTypeConfig] = newFamily("config-server", true)
	}
	return f
}
func newFamily(name string, configuration bool) Family {
	groups := map[string]GroupObj{}
	names := []string{GroupNameResources, GroupNameReadinessProbe, GroupNameLivenessProbe}
	if configuration {
		names = append(names, GroupNameConfiguration)
	}
	for _, g := range names {
		groups[g] = GroupObj{Name: g, Parameters: map[string]Parameter{}}
	}
	return Family{Name: name, Groups: groups}
}
func put(f Family, group, key, value string) {
	g, ok := f.Groups[group]
	if !ok {
		g = GroupObj{Name: group, Parameters: map[string]Parameter{}}
	}
	putGroup(&g, key, value)
	f.Groups[group] = g
}
func putGroup(g *GroupObj, key, value string) {
	g.Parameters[key] = Parameter{Name: key, Value: value}
}
