package mongodboperatorcalculator

import (
	"strconv"
	"strings"
	"testing"
)

func calculate(t *testing.T, req ConfigurationRequest) (ConfigurationRequest, ResponseMessage, map[string]Family, error) {
	t.Helper()
	var conf Configuration
	conf.Init()
	if req.MongoDBVersion.Major == 0 {
		req.MongoDBVersion = Version{8, 0, 0}
	}
	var c MongoDBOperatorCalculator
	resolved := c.Init(req, conf)
	err, msg, families := c.GetCalculate()
	return resolved, msg, families, err
}

func param(t *testing.T, f map[string]Family, family, group, name string) string {
	t.Helper()
	p, ok := f[family].Groups[group].Parameters[name]
	if !ok {
		t.Fatalf("missing %s/%s/%s", family, group, name)
	}
	return p.Value
}

func cpuValue(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
	if err != nil {
		t.Fatalf("bad cpu %q", s)
	}
	return n
}

func memValue(t *testing.T, s string) float64 {
	t.Helper()
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("bad memory %q", s)
	}
	return n
}

// The operator passes --wiredTigerCacheSizeGB=%.2f of
// max(0.25, floor(cacheSizeRatio x (limit - 1GiB)) / 1GiB), which overrides
// any cacheSizeGB in the mongod configuration. The calculator must match it.
func TestCacheSizeMatchesOperator(t *testing.T) {
	for memory, want := range map[float64]string{
		512 * mib:  "0.25",
		1 * gib:    "0.25",
		1536 * mib: "0.25",
		2 * gib:    "0.50",
		7947 * mib: "3.38",
		16 * gib:   "7.50",
		// %.2f rounds: 3.38477GiB becomes 3.38, one MiB more gives 3.38525GiB and 3.39.
		7956 * mib:  "3.38",
		7957 * mib:  "3.39",
		512 * gib:   "255.50",
		15635316736: "6.78",
	} {
		if got := operatorCacheSizeGB(memory); got != want {
			t.Errorf("operatorCacheSizeGB(%v) = %s, want %s", memory, got, want)
		}
	}
	_, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 2}, Connections: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := param(t, f, FamilyTypeMongoDB, GroupNameConfiguration, "storage.wiredTiger.engineConfig.cacheSizeGB"); got != "3.38" {
		t.Fatalf("cacheSizeGB=%s want 3.38", got)
	}
	if got := param(t, f, FamilyTypeMongoDB, GroupNameStorage, "wiredTiger.engineConfig.cacheSizeRatio"); got != "0.5" {
		t.Fatalf("cacheSizeRatio=%s want 0.5", got)
	}
}

func TestProbesMatchOperatorDefaults(t *testing.T) {
	_, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 2}, Connections: 500})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{
		FamilyTypeMongoDB + "/" + GroupNameReadinessProbe:       {"initialDelaySeconds": "10", "timeoutSeconds": "2", "periodSeconds": "3", "failureThreshold": "8", "successThreshold": "1"},
		FamilyTypeMongoDB + "/" + GroupNameLivenessProbe:        {"initialDelaySeconds": "60", "timeoutSeconds": "10", "periodSeconds": "30", "failureThreshold": "4", "startupDelaySeconds": "7200"},
		FamilyTypeConfig + "/" + GroupNameReadinessProbe:        {"initialDelaySeconds": "10", "timeoutSeconds": "2", "periodSeconds": "3", "failureThreshold": "3", "successThreshold": "1"},
		FamilyTypeConfig + "/" + GroupNameLivenessProbe:         {"initialDelaySeconds": "60", "timeoutSeconds": "10", "periodSeconds": "30", "failureThreshold": "4", "startupDelaySeconds": "7200"},
		FamilyTypeMongos + "/" + GroupNameReadinessProbe:        {"initialDelaySeconds": "10", "timeoutSeconds": "1", "periodSeconds": "1", "failureThreshold": "3", "successThreshold": "1"},
		FamilyTypeMongos + "/" + GroupNameLivenessProbe:         {"initialDelaySeconds": "60", "timeoutSeconds": "10", "periodSeconds": "30", "failureThreshold": "4", "startupDelaySeconds": "10"},
		FamilyTypeMonitor + "/mongos." + GroupNameLivenessProbe: {"initialDelaySeconds": "60", "timeoutSeconds": "5", "periodSeconds": "10", "failureThreshold": "3"},
	}
	for key, params := range want {
		family, group, _ := strings.Cut(key, "/")
		got := f[family].Groups[group].Parameters
		if len(got) != len(params) {
			t.Errorf("%s: got %d parameters %+v, want %+v", key, len(got), got, params)
		}
		for name, value := range params {
			if got[name].Value != value {
				t.Errorf("%s %s=%q want %q", key, name, got[name].Value, value)
			}
		}
	}
	for _, prefix := range []string{"mongod.", "configserver.", "mongos."} {
		if _, ok := f[FamilyTypeMonitor].Groups[prefix+GroupNameReadinessProbe]; ok {
			t.Errorf("PMM has no readiness probe by default, got one for %s", prefix)
		}
	}
}

// Every successful result must fit mongod and its sidecars in the dimension,
// keep mongod above the minimum, and emit a valid cache size.
func TestPodBudgetFitsDimension(t *testing.T) {
	var conf Configuration
	conf.Init()
	for _, d := range conf.Dimension {
		if d.Id == DimensionOpen {
			continue
		}
		for _, dbtype := range []string{DbTypeReplicaSet, DbTypeShardedCluster} {
			for _, dedicated := range []bool{false, true} {
				if dedicated && dbtype == DbTypeShardedCluster {
					continue
				}
				for _, backup := range []bool{false, true} {
					for _, cost := range []float64{0, 0.2} {
						req := ConfigurationRequest{DBType: dbtype, Dimension: Dimension{Id: d.Id}, LoadType: LoadType{Id: 1}, Connections: 10,
							MongoDBDedicated: dedicated, Backup: BackupConfiguration{Enabled: backup}, ProviderCostPct: cost}
						resolved, msg, f, err := calculate(t, req)
						if err != nil {
							if msg.MType != OverutilizingI || !strings.Contains(msg.MText, "minimum") {
								t.Fatalf("%+v: unexpected error %v %+v", req, err, msg)
							}
							continue
						}
						cpu := cpuValue(t, param(t, f, FamilyTypeMongoDB, GroupNameResources, "limit_cpu"))
						mem := memValue(t, param(t, f, FamilyTypeMongoDB, GroupNameResources, "limit_memory"))
						if cpu < MinMongodCPU || mem < MinMongodMemory {
							t.Fatalf("%+v: mongod below minimum: %dm %v", req, cpu, mem)
						}
						if !dedicated {
							group := GroupNameResources
							if dbtype == DbTypeShardedCluster {
								group = "mongod." + GroupNameResources
							}
							cpu += cpuValue(t, param(t, f, FamilyTypeMonitor, group, "limit_cpu"))
							mem += memValue(t, param(t, f, FamilyTypeMonitor, group, "limit_memory"))
						}
						if backup {
							cpu += cpuValue(t, param(t, f, FamilyTypeMongoDB, GroupNamePBMResources, "limit_cpu"))
							mem += memValue(t, param(t, f, FamilyTypeMongoDB, GroupNamePBMResources, "limit_memory"))
						}
						if float64(cpu) > float64(d.Cpu)*(1-cost) || mem > d.MemoryBytes*(1-cost) {
							t.Fatalf("%+v: pod %dm/%v exceeds dimension %dm/%v", req, cpu, mem, d.Cpu, d.MemoryBytes)
						}
						dim := resolved.Dimension
						if dim.MongoDBCpu+dim.MonitorCpu+dim.BackupCpu > d.Cpu {
							t.Fatalf("%+v: incoming allocation exceeds dimension: %+v", req, dim)
						}
						cache, _ := strconv.ParseFloat(param(t, f, FamilyTypeMongoDB, GroupNameConfiguration, "storage.wiredTiger.engineConfig.cacheSizeGB"), 64)
						if cache < MinCacheSizeGB {
							t.Fatalf("%+v: cacheSizeGB %v below minimum", req, cache)
						}
					}
				}
			}
		}
	}
}

func TestDimensionTooSmallForSidecars(t *testing.T) {
	for _, req := range []ConfigurationRequest{
		{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 1}, LoadType: LoadType{Id: 1}, Connections: 1},
		{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 1}, Connections: 1, Backup: BackupConfiguration{Enabled: true}},
		{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: DimensionOpen, Cpu: 100, Memory: "256Mi"}, LoadType: LoadType{Id: 1}, Connections: 1},
		{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 1}, Connections: 1, ProviderCostPct: 0.99},
	} {
		_, msg, f, err := calculate(t, req)
		if err == nil || msg.MType != OverutilizingI || !strings.Contains(msg.MText, "minimum") || len(f) != 0 {
			t.Errorf("%+v: expected too-small error, got %v %+v", req, err, msg)
		}
	}
	_, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 1}, LoadType: LoadType{Id: 1}, Connections: 10, MongoDBDedicated: true})
	if err != nil {
		t.Fatalf("XSmall dedicated should fit: %v", err)
	}
}

func TestDedicatedModeOmitsPMM(t *testing.T) {
	resolved, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 1}, Connections: 50, MongoDBDedicated: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f[FamilyTypeMonitor]; ok {
		t.Fatal("monitor should be omitted")
	}
	if resolved.Dimension.MongoDBCpu != 800 || resolved.Dimension.MongoDBMemory != 2*gib || resolved.Dimension.MonitorCpu != 0 {
		t.Fatalf("dedicated mongod should get the full dimension: %+v", resolved.Dimension)
	}
}

func TestBackupAddsPBMAgentAndReducesMongod(t *testing.T) {
	without, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 2}, Connections: 50})
	if err != nil {
		t.Fatal(err)
	}
	with, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 2}, Connections: 50, Backup: BackupConfiguration{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	p := f[FamilyTypeMongoDB].Groups[GroupNamePBMResources].Parameters
	if p["request_cpu"].Value != "300m" || p["limit_cpu"].Value != "300m" || p["request_memory"].Value != "1073741824" || p["limit_memory"].Value != "1288490188" {
		t.Fatalf("unexpected PBM resources: %+v", p)
	}
	if without.Dimension.MongoDBCpu-with.Dimension.MongoDBCpu != PBMAgentCPU || with.Dimension.BackupCpu != PBMAgentCPU {
		t.Fatalf("PBM CPU should come out of the pod budget: without=%+v with=%+v", without.Dimension, with.Dimension)
	}
}

func TestMaxIncomingConnectionsIncludesInternalOverhead(t *testing.T) {
	_, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 1}, Connections: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := param(t, f, FamilyTypeMongoDB, GroupNameConfiguration, "net.maxIncomingConnections"); got != "102" {
		t.Fatalf("maxIncomingConnections=%s want 102", got)
	}
}

func TestProbesDoNotScaleWithConnections(t *testing.T) {
	var first map[string]Parameter
	for _, conns := range []int{1, 1000, 10000} {
		_, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 10}, LoadType: LoadType{Id: 1}, Connections: conns})
		if err != nil {
			t.Fatal(err)
		}
		live := f[FamilyTypeMongoDB].Groups[GroupNameLivenessProbe].Parameters
		if live["timeoutSeconds"].Value != "10" || live["startupDelaySeconds"].Value != "7200" {
			t.Fatalf("unexpected liveness probe: %+v", live)
		}
		ready := f[FamilyTypeMongoDB].Groups[GroupNameReadinessProbe].Parameters
		if first == nil {
			first = ready
		} else if ready["timeoutSeconds"] != first["timeoutSeconds"] || ready["failureThreshold"].Value != "8" {
			t.Fatalf("readiness probe changed with connections: %+v vs %+v", ready, first)
		}
	}
}

func TestRequestsAreRoundedAndGuaranteedQoS(t *testing.T) {
	_, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 2}, Connections: 100, Backup: BackupConfiguration{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"request_memory", "limit_memory"} {
		if v := memValue(t, param(t, f, FamilyTypeMongoDB, GroupNameResources, name)); v != floorMiB(v) {
			t.Fatalf("%s=%v is not a whole MiB", name, v)
		}
	}
	if param(t, f, FamilyTypeMongoDB, GroupNameResources, "request_cpu") == param(t, f, FamilyTypeMongoDB, GroupNameResources, "limit_cpu") {
		t.Fatal("default requests should be below limits")
	}
	_, _, f, err = calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 2}, Connections: 100, Backup: BackupConfiguration{Enabled: true}, GuaranteedQoS: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct{ family, group string }{
		{FamilyTypeMongoDB, GroupNameResources}, {FamilyTypeMongoDB, GroupNamePBMResources}, {FamilyTypeMonitor, GroupNameResources},
	} {
		p := f[g.family].Groups[g.group].Parameters
		if p["request_cpu"].Value != p["limit_cpu"].Value || p["request_memory"].Value != p["limit_memory"].Value {
			t.Fatalf("%s/%s: requests must equal limits with guaranteedqos: %+v", g.family, g.group, p)
		}
	}
}

func TestUtilizationReflectsWorkload(t *testing.T) {
	pct := func(dim, load, conns int) string {
		_, msg, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: dim}, LoadType: LoadType{Id: load}, Connections: conns})
		if err != nil {
			t.Fatalf("dim %d load %d conns %d: %v", dim, load, conns, err)
		}
		return msg.MText
	}
	small, large := pct(10, 1, 100), pct(3, 1, 100)
	if small == large {
		t.Fatalf("utilization should differ between dimensions: %q", small)
	}
	if !strings.HasPrefix(small, "estimated capacity utilization is 10%") {
		t.Fatalf("idle 16XLarge should report low utilization, got %q", small)
	}
	if pct(5, 1, 1000) == pct(5, 4, 1000) {
		t.Fatal("load type should change utilization")
	}
}

func TestNoAutoscaling(t *testing.T) {
	_, msg, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 4}, Connections: 10000})
	if err == nil || msg.MType != OverutilizingI || len(f) != 0 || !strings.Contains(msg.MText, "mongod at") {
		t.Fatalf("expected overload, got %v %+v", err, msg)
	}
}
