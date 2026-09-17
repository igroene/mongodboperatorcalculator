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
