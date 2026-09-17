package mongodboperatorcalculator

import "testing"

func TestWiredTigerCacheIsHalfPodMemory(t *testing.T) {
	var conf Configuration
	conf.Init()
	var c MongoDBOperatorCalculator
	c.Init(ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 2}, Connections: 50, MongoDBVersion: Version{7, 0, 0}}, conf)
	e, _, f := c.GetCalculate()
	if e != nil {
		t.Fatal(e)
	}
	p := f[FamilyTypeMongoDB].Groups[GroupNameConfiguration].Parameters["storage.wiredTiger.engineConfig.cacheSizeGB"]
	if p.Value != "0.94" {
		t.Fatalf("cache=%s want 0.94", p.Value)
	}
}
func TestDedicatedModeOmitsSidecars(t *testing.T) {
	var conf Configuration
	conf.Init()
	var c MongoDBOperatorCalculator
	c.Init(ConfigurationRequest{DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 1}, Connections: 50, MongoDBVersion: Version{8, 0, 0}, MongoDBDedicated: true}, conf)
	_, _, f := c.GetCalculate()
	if _, ok := f[FamilyTypeMonitor]; ok {
		t.Fatal("monitor should be omitted")
	}
}
