package mongodboperatorcalculator

import "testing"

func TestConfigurationAndMemory(t *testing.T) {
	var c Configuration
	c.Init()
	if len(c.Dimension) != 5 || c.GetDimensionByID(2).MemoryBytes == 0 {
		t.Fatal("configuration not initialized")
	}
	d := Dimension{}
	got, e := d.ConvertMemoryToBytes("8Gi")
	if e != nil || got != 8*gb {
		t.Fatalf("memory conversion got %v %v", got, e)
	}
}
func TestOpenDimension(t *testing.T) {
	var c Configuration
	c.Init()
	d, e := c.ResolveDimension(Dimension{Id: DimensionOpen, Cpu: 4000, MemoryBytes: 8 * gb})
	if e != nil || d.MongoDBMemory != 8*gb || d.MongoDBCpu != 4000 {
		t.Fatalf("bad open dimension: %+v %v", d, e)
	}
}
