package mongodboperatorcalculator

import (
	"encoding/json"
	"fmt"
	"math"
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

type Dimension struct {
	Id            int     `json:"id"`
	Name          string  `json:"name"`
	Cpu           int     `json:"cpu"`
	Memory        string  `json:"memory"`
	MemoryBytes   float64 `json:"-"`
	MongoDBCpu    int     `json:"mongodbCpu"`
	MonitorCpu    int     `json:"monitorCpu"`
	MongosCpu     int     `json:"mongosCpu"`
	ConfigCpu     int     `json:"configCpu"`
	MongoDBMemory float64 `json:"mongodbMemory"`
	MonitorMemory float64 `json:"monitorMemory"`
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
	DBType           string    `json:"dbtype"`
	Dimension        Dimension `json:"dimension"`
	LoadType         LoadType  `json:"loadtype"`
	Connections      int       `json:"connections"`
	Output           string    `json:"output"`
	MongoDBVersion   Version   `json:"mongodbversion"`
	ProviderCostPct  float64   `json:"providercostpct"`
	MongoDBDedicated bool      `json:"mongodbdedicated"`
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
		{1, "XSmall", 600, "1Gi", 1 * gb, 500, 100, 0, 0, 896 * mib, 128 * mib, 0, 0},
		{2, "Small", 800, "2Gi", 2 * gb, 700, 100, 0, 0, 1920 * mib, 128 * mib, 0, 0},
		{3, "Medium", 1000, "4Gi", 4 * gb, 800, 200, 0, 0, 3840 * mib, 256 * mib, 0, 0},
		{4, "Large", 2000, "8Gi", 8 * gb, 1700, 300, 0, 0, 7680 * mib, 512 * mib, 0, 0},
		{5, "XLarge", 4000, "16Gi", 16 * gb, 3600, 400, 0, 0, 15872 * mib, 512 * mib, 0, 0},
		{6, "2XLarge", 8000, "32Gi", 32 * gb, 7500, 500, 0, 0, 31744 * mib, 1 * gb, 0, 0},
		{7, "4XLarge", 16000, "64Gi", 64 * gb, 15000, 1000, 0, 0, 63 * gb, 1 * gb, 0, 0},
		{8, "8XLarge", 32000, "128Gi", 128 * gb, 31000, 1000, 0, 0, 127 * gb, 1 * gb, 0, 0},
		{9, "12XLarge", 64000, "256Gi", 256 * gb, 62500, 1500, 0, 0, 254 * gb, 2 * gb, 0, 0},
		{10, "16XLarge", 128000, "512Gi", 512 * gb, 126000, 2000, 0, 0, 510 * gb, 2 * gb, 0, 0},
		{DimensionOpen, "Open request by resources", 0, "0", 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	c.LoadType = []LoadType{{1, "Mainly Reads", "Read-heavy workload"}, {2, "Light OLTP", "Mixed workload with moderate writes"}, {3, "Heavy OLTP", "Highly concurrent mixed workload"}, {4, "Mainly Writes", "Write-heavy workload"}}
}

const (
	mib = 1024 * 1024
	gb  = 1024 * mib
)

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
func (d Dimension) ConvertMemoryToBytes(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	units := map[string]float64{"B": 1, "K": 1 << 10, "KI": 1 << 10, "KB": 1 << 10, "KIB": 1 << 10, "M": 1 << 20, "MI": 1 << 20, "MB": 1 << 20, "MIB": 1 << 20, "G": 1 << 30, "GI": 1 << 30, "GB": 1 << 30, "GIB": 1 << 30, "T": 1 << 40, "TI": 1 << 40, "TB": 1 << 40, "TIB": 1 << 40}
	for u, m := range units {
		if strings.HasSuffix(s, u) {
			n, e := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, u)), 64)
			return n * m, e
		}
	}
	return 0, fmt.Errorf("invalid memory quantity %q", s)
}
func (c Configuration) ResolveDimension(d Dimension) (Dimension, error) {
	if d.Id != DimensionOpen {
		x := c.GetDimensionByID(d.Id)
		if x.Id == 0 {
			return d, fmt.Errorf("unknown dimension %d", d.Id)
		}
		return x, nil
	}
	if d.Cpu <= 0 || d.MemoryBytes <= 0 {
		return d, fmt.Errorf("open dimension requires cpu and memory")
	}
	d.Name = "Open request by resources"
	d.MongoDBCpu = d.Cpu
	d.MongoDBMemory = d.MemoryBytes
	d.MonitorCpu = 0
	d.MonitorMemory = 0
	d.MongosCpu = d.Cpu
	d.MongosMemory = d.MemoryBytes
	d.ConfigCpu = d.Cpu
	d.ConfigMemory = d.MemoryBytes
	return d, nil
}
func (c Configuration) Families(req ConfigurationRequest) map[string]Family {
	f := map[string]Family{}
	f[FamilyTypeMongoDB] = newFamily("mongod")
	if !req.MongoDBDedicated {
		f[FamilyTypeMonitor] = newFamily("pmm-client")
	}
	if req.DBType == DbTypeShardedCluster {
		f[FamilyTypeMongos] = newFamily("mongos")
		f[FamilyTypeConfig] = newFamily("config-server")
	}
	return f
}
func newFamily(name string) Family {
	return Family{Name: name, Groups: map[string]GroupObj{GroupNameConfiguration: {Name: "configuration", Parameters: map[string]Parameter{}}, GroupNameResources: {Name: "resources", Parameters: map[string]Parameter{}}, "readinessProbe": {Name: "readinessProbe", Parameters: map[string]Parameter{}}, "livenessProbe": {Name: "livenessProbe", Parameters: map[string]Parameter{}}}}
}
func put(f Family, group, key, value string) {
	g := f.Groups[group]
	g.Parameters[key] = Parameter{Name: key, Value: value}
	f.Groups[group] = g
}
func formatGB(bytes float64) string {
	return strconv.FormatFloat(math.Round(bytes/float64(gb)*100)/100, 'f', -1, 64)
}
