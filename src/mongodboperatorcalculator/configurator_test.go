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

func TestBackupAddsPBMAgentToReplicaSet(t *testing.T) {
	var conf Configuration
	conf.Init()
	var c MongoDBOperatorCalculator
	c.Init(ConfigurationRequest{
		DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 3}, LoadType: LoadType{Id: 2},
		Connections: 50, MongoDBVersion: Version{7, 0, 0}, Backup: BackupConfiguration{Enabled: true},
	}, conf)
	if err, _, families := c.GetCalculate(); err != nil {
		t.Fatal(err)
	} else {
		parameters := families[FamilyTypeMongoDB].Groups["pbm-agent.resources"].Parameters
		if parameters["request_cpu"].Value != "300m" || parameters["limit_cpu"].Value != "300m" ||
			parameters["request_memory"].Value != "1073741824" || parameters["limit_memory"].Value != "1288490188" {
			t.Fatalf("unexpected PBM resources: %+v", parameters)
		}
	}
}

func TestBackupIncreasesCapacityUtilization(t *testing.T) {
	var conf Configuration
	conf.Init()
	without := MongoDBOperatorCalculator{}
	without.Init(ConfigurationRequest{
		DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 2},
		Connections: 500, MongoDBVersion: Version{7, 0, 0},
	}, conf)
	_, withoutMessage, _ := without.GetCalculate()

	with := MongoDBOperatorCalculator{}
	with.Init(ConfigurationRequest{
		DBType: DbTypeReplicaSet, Dimension: Dimension{Id: 2}, LoadType: LoadType{Id: 2},
		Connections: 500, MongoDBVersion: Version{7, 0, 0}, Backup: BackupConfiguration{Enabled: true},
	}, conf)
	_, withMessage, _ := with.GetCalculate()
	if withoutMessage.MText == withMessage.MText {
		t.Fatalf("backup should change utilization: without=%q with=%q", withoutMessage.MText, withMessage.MText)
	}
}
