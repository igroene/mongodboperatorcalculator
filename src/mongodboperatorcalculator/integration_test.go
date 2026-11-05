package mongodboperatorcalculator

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Exercise capacity decisions for total open connections, including idle client
// sockets. The same count must be used for workload sizing and admission limits.
func TestTotalOpenConnectionScenarios(t *testing.T) {
	cases := []struct {
		name        string
		cpu         int
		memory      string
		connections int
		load        int
		mongos      int
		wantType    int
		wantDetail  string
	}{
		{"small pool", 2000, "8Gi", 100, 2, 0, OkI, "CPU 12%, memory 15%"},
		{"1000 open including idle", 2000, "8Gi", 1000, 2, 0, OkI, "CPU 69%, memory 45%"},
		{"near CPU limit", 2000, "8Gi", 1300, 2, 0, ClosetolimitI, "CPU 88%, memory 55%"},
		{"too many for CPU", 2000, "8Gi", 1600, 2, 0, OverutilizingI, "CPU 106%, memory 65%"},
		{"more CPU fits", 4000, "8Gi", 1600, 2, 0, OkI, "CPU 47%, memory 65%"},
		{"write profile needs more CPU", 2000, "8Gi", 1000, 4, 0, OverutilizingI, "CPU 194%, memory 67%"},
		{"memory bound", 16000, "2Gi", 1000, 2, 0, OverutilizingI, "CPU 7%, memory 122%"},
		{"more CPU cannot fix memory", 32000, "2Gi", 1000, 2, 0, OverutilizingI, "CPU 3%, memory 122%"},
		{"more memory fits", 16000, "4Gi", 1000, 2, 0, OkI, "CPU 7%, memory 75%"},
		{"three routers share clients", 4000, "16Gi", 1500, 2, 3, OkI, "shard mongod 44%, config server 20%, mongos 69%"},
		{"routers do not dilute shard load", 2000, "8Gi", 1600, 2, 6, OverutilizingI, "shard mongod at 106%"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := ConfigurationRequest{
				DBType: DbTypeReplicaSet, Dimension: Dimension{Id: DimensionOpen, Cpu: tc.cpu, Memory: tc.memory},
				Connections: tc.connections, LoadType: LoadType{Id: tc.load},
			}
			if tc.mongos > 0 {
				req.DBType = DbTypeShardedCluster
				req.Mongos.Count = tc.mongos
			}
			resolved, msg, families, err := calculate(t, req)
			t.Logf("%dm/%s, %d open connections, profile %d: [%d] %s", tc.cpu, tc.memory, tc.connections, tc.load, msg.MType, msg.MText)
			if msg.MType != tc.wantType || !strings.Contains(msg.MText, tc.wantDetail) {
				t.Fatalf("got %+v, want type %d and %q", msg, tc.wantType, tc.wantDetail)
			}
			if resolved.Connections != tc.connections {
				t.Fatalf("requested count was changed: got %d, want %d", resolved.Connections, tc.connections)
			}
			assumption := fmt.Sprintf("assumes all %d open client connections use the selected workload profile", tc.connections)
			if !strings.Contains(msg.MText, assumption) {
				t.Fatalf("result does not explain the total-connection assumption: %s", msg.MText)
			}
			if tc.wantType == OverutilizingI {
				if err == nil || len(families) != 0 {
					t.Fatalf("overloaded request must fail without configuration: %v %+v", err, families)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for family, count := range map[string]int{FamilyTypeMongoDB: tc.connections, FamilyTypeMongos: tc.connections} {
				if family == FamilyTypeMongos {
					if tc.mongos == 0 {
						continue
					}
					count = (count + tc.mongos - 1) / tc.mongos
				}
				limit, err := strconv.Atoi(param(t, families, family, GroupNameConfiguration, "net.maxIncomingConnections"))
				if err != nil || limit <= count {
					t.Fatalf("%s connection limit %d must admit %d open connections with headroom: %v", family, limit, count, err)
				}
			}
		})
	}
}

func TestShardedClusterFamiliesArePerComponent(t *testing.T) {
	resolved, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 2}, Connections: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{FamilyTypeMongoDB, FamilyTypeMongos, FamilyTypeConfig, FamilyTypeMonitor} {
		if _, ok := f[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	if resolved.Mongos.Count != DefaultMongosCount {
		t.Fatalf("mongos.count should default to %d, got %d", DefaultMongosCount, resolved.Mongos.Count)
	}
	for _, prefix := range []string{"mongod.", "configserver.", "mongos."} {
		p := f[FamilyTypeMonitor].Groups[prefix+GroupNameResources].Parameters
		if p["request_cpu"].Value != "300m" || p["limit_cpu"].Value != "400m" || p["request_memory"].Value != "150000000" || p["limit_memory"].Value != "256000000" {
			t.Fatalf("unexpected PMM resources for %s: %+v", prefix, p)
		}
		if _, ok := f[FamilyTypeMonitor].Groups[prefix+GroupNameLivenessProbe]; !ok {
			t.Fatalf("missing PMM liveness probe for %s", prefix)
		}
		if _, ok := f[FamilyTypeMonitor].Groups[prefix+GroupNameReadinessProbe]; ok {
			t.Fatalf("unexpected PMM readiness probe for %s", prefix)
		}
	}
	// 500 client connections split over 3 mongos.
	if got := param(t, f, FamilyTypeMongos, GroupNameConfiguration, "net.maxIncomingConnections"); got != "293" {
		t.Fatalf("mongos maxIncomingConnections=%s want 293", got)
	}
	// Shard: 500 x 1.15 + 3 mongos x 10 pool + 100 internal.
	if got := param(t, f, FamilyTypeMongoDB, GroupNameConfiguration, "net.maxIncomingConnections"); got != "705" {
		t.Fatalf("shard maxIncomingConnections=%s want 705", got)
	}
	if got := param(t, f, FamilyTypeConfig, GroupNameReadinessProbe, "failureThreshold"); got != "3" {
		t.Fatalf("config readiness failureThreshold=%s want 3", got)
	}
}

func TestConfigServerDoesNotScaleWithShard(t *testing.T) {
	for _, dim := range []int{4, 10} {
		resolved, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: dim}, LoadType: LoadType{Id: 2}, Connections: 500})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Dimension.ConfigCpu != ConfigServerTiers[0].Cpu || resolved.Dimension.ConfigMemory != ConfigServerTiers[0].Memory {
			t.Fatalf("dimension %d: config server should stay on the first tier: %dm/%v", dim, resolved.Dimension.ConfigCpu, resolved.Dimension.ConfigMemory)
		}
		if resolved.Dimension.MongosCpu > 1000 || resolved.Dimension.MongosMemory > 1*gib {
			t.Fatalf("dimension %d: mongos should follow connections, not shard size: %dm/%v", dim, resolved.Dimension.MongosCpu, resolved.Dimension.MongosMemory)
		}
	}
	many, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 2}, Connections: 500, Mongos: MongosConfiguration{Count: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if many.Dimension.ConfigCpu <= ConfigServerTiers[0].Cpu {
		t.Fatalf("config server should grow with mongos count, got %dm", many.Dimension.ConfigCpu)
	}
}

func TestMongosScalesWithConnectionsAndCount(t *testing.T) {
	few, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 8}, LoadType: LoadType{Id: 2}, Connections: 6000, Mongos: MongosConfiguration{Count: 2}})
	if err != nil {
		t.Fatal(err)
	}
	more, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 8}, LoadType: LoadType{Id: 2}, Connections: 6000, Mongos: MongosConfiguration{Count: 6}})
	if err != nil {
		t.Fatal(err)
	}
	if more.Dimension.MongosCpu >= few.Dimension.MongosCpu || more.Dimension.MongosMemory >= few.Dimension.MongosMemory {
		t.Fatalf("more mongos should mean smaller mongos: 2=%+v 6=%+v", few.Dimension, more.Dimension)
	}
	_, msg, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 10}, LoadType: LoadType{Id: 3}, Connections: 50000, Mongos: MongosConfiguration{Count: 1}})
	if err == nil || !strings.Contains(msg.MText, "increase mongos.count") {
		t.Fatalf("an overloaded mongos should be reported: %v %+v", err, msg)
	}
}

// Every mongos keeps idle pool connections to each shard. They cost thread
// stack memory on the shard even when there is almost no client traffic.
func TestShardCapacityIncludesMongosPoolMemory(t *testing.T) {
	req := ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 1}, Connections: 1, Mongos: MongosConfiguration{Count: 100}}
	_, msg, _, err := calculate(t, req)
	if err != nil || msg.MType != ClosetolimitI || !strings.Contains(msg.MText, "shard mongod 86%") {
		t.Fatalf("1000 idle pool connections should push a Small shard close to its limit: %v %+v", err, msg)
	}
	req.Connections = 200
	_, msg, _, err = calculate(t, req)
	if err == nil || msg.MType != OverutilizingI || !strings.Contains(msg.MText, "shard mongod at") {
		t.Fatalf("pool connections plus 200 clients must overload a Small shard: %v %+v", err, msg)
	}
	few, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 1}, Connections: 100, Mongos: MongosConfiguration{Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	many, _, _, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 5}, LoadType: LoadType{Id: 1}, Connections: 100, Mongos: MongosConfiguration{Count: 100}})
	if err != nil {
		t.Fatal(err)
	}
	fewPlan, manyPlan := planResources(few), planResources(many)
	if manyPlan.components[0].memory <= fewPlan.components[0].memory {
		t.Fatalf("more mongos should raise shard memory use: 1=%v 100=%v", fewPlan.components[0], manyPlan.components[0])
	}
	if manyPlan.components[0].cpu != fewPlan.components[0].cpu {
		t.Fatalf("idle pool connections must not add CPU: 1=%v 100=%v", fewPlan.components[0], manyPlan.components[0])
	}
}

func TestBackupAddsPBMAgentToShardAndConfigServer(t *testing.T) {
	_, _, f, err := calculate(t, ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 2}, Connections: 500, Backup: BackupConfiguration{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{FamilyTypeMongoDB, FamilyTypeConfig} {
		if _, ok := f[family].Groups[GroupNamePBMResources]; !ok {
			t.Fatalf("missing PBM resources for %s", family)
		}
	}
	if _, ok := f[FamilyTypeMongos].Groups[GroupNamePBMResources]; ok {
		t.Fatal("mongos must not get a PBM agent")
	}
}

func TestInvalidRequestsAreEchoedUnchanged(t *testing.T) {
	base := ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 4}, LoadType: LoadType{Id: 2}, Connections: 10, MongoDBVersion: Version{8, 0, 0}}
	cases := []struct {
		name   string
		modify func(*ConfigurationRequest)
		want   string
	}{
		{"loadtype", func(r *ConfigurationRequest) { r.LoadType.Id = 9 }, "unsupported loadtype 9"},
		{"missing loadtype", func(r *ConfigurationRequest) { r.LoadType.Id = 0 }, "loadtype is required"},
		{"missing dimension", func(r *ConfigurationRequest) { r.Dimension.Id = 0 }, "dimension is required"},
		{"version range", func(r *ConfigurationRequest) { r.MongoDBVersion = Version{8, 500, -1} }, "unsupported MongoDB version"},
		{"version major", func(r *ConfigurationRequest) { r.MongoDBVersion = Version{6, 0, 0} }, "unsupported MongoDB version"},
		{"output", func(r *ConfigurationRequest) { r.Output = "xml" }, "unsupported output"},
		{"dbtype", func(r *ConfigurationRequest) { r.DBType = "standalone" }, "unsupported dbtype"},
		{"connections", func(r *ConfigurationRequest) { r.Connections = 0 }, "greater than zero"},
		{"connections max", func(r *ConfigurationRequest) { r.Connections = MaxConnections + 1 }, "at most"},
		{"providercostpct", func(r *ConfigurationRequest) { r.ProviderCostPct = 12 }, "providercostpct"},
		{"dedicated sharded", func(r *ConfigurationRequest) { r.DBType = DbTypeShardedCluster; r.MongoDBDedicated = true }, "dedicated"},
		{"mongos on replica set", func(r *ConfigurationRequest) { r.Mongos.Count = 2 }, "only valid for sharded_cluster"},
		{"mongos count", func(r *ConfigurationRequest) { r.DBType = DbTypeShardedCluster; r.Mongos.Count = 101 }, "between 1 and 100"},
		{"memory", func(r *ConfigurationRequest) { r.Dimension = Dimension{Id: DimensionOpen, Cpu: 1000, Memory: "NaNGi"} }, "invalid memory quantity"},
	}
	for _, tc := range cases {
		req := base
		tc.modify(&req)
		resolved, msg, f, err := calculate(t, req)
		if err == nil || msg.MType != ErrorexecI || !strings.Contains(msg.MText, tc.want) || len(f) != 0 {
			t.Errorf("%s: got %v %+v, want %q", tc.name, err, msg, tc.want)
		}
		if !reflect.DeepEqual(resolved, req) {
			t.Errorf("%s: incoming was modified:\n got  %+v\n want %+v", tc.name, resolved, req)
		}
	}
}
