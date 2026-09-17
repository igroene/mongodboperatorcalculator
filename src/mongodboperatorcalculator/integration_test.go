package mongodboperatorcalculator

import "testing"

func TestShardedClusterFamiliesArePerComponent(t *testing.T) {
	var conf Configuration
	conf.Init()
	var c MongoDBOperatorCalculator
	c.Init(ConfigurationRequest{DBType: DbTypeShardedCluster, Dimension: Dimension{Id: 3}, LoadType: LoadType{Id: 2}, Connections: 500, MongoDBVersion: Version{7, 0, 0}}, conf)
	e, _, f := c.GetCalculate()
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{FamilyTypeMongoDB, FamilyTypeMongos, FamilyTypeConfig, FamilyTypeMonitor} {
		if _, ok := f[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	mongoResources := f[FamilyTypeMongoDB].Groups[GroupNameResources].Parameters
	if mongoResources["limit_cpu"].Value != "800m" || mongoResources["limit_memory"].Value != "4026531840" {
		t.Fatalf("shard mongod did not retain full dimension allocation: %+v", mongoResources)
	}
	if cache := f[FamilyTypeMongoDB].Groups[GroupNameConfiguration].Parameters["storage.wiredTiger.engineConfig.cacheSizeGB"].Value; cache != "1.88" {
		t.Fatalf("shard cache=%s want 1.88", cache)
	}
	configResources := f[FamilyTypeConfig].Groups[GroupNameResources].Parameters
	if configResources["limit_cpu"].Value != "500m" || configResources["limit_memory"].Value != "1073741824" {
		t.Fatalf("config profile should be independent: %+v", configResources)
	}
	monitor := f[FamilyTypeMonitor].Groups
	for _, group := range []string{"mongod.resources", "configserver.resources", "mongos.resources"} {
		if _, ok := monitor[group]; !ok {
			t.Fatalf("missing PMM group %s", group)
		}
	}
}
func TestNoAutoscaling(t *testing.T) {
	var conf Configuration
	conf.Init()
	var c MongoDBOperatorCalculator
	c.Init(ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 4}, Connections: 10000, MongoDBVersion: Version{7, 0, 0}}, conf)
	e, msg, _ := c.GetCalculate()
	if e == nil || msg.MType != OverutilizingI {
		t.Fatalf("expected overload, got %v %d", e, msg.MType)
	}
}
