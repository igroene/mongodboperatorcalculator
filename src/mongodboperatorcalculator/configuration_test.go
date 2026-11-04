package mongodboperatorcalculator

import (
	"strings"
	"testing"
)

func TestConfigurationCatalog(t *testing.T) {
	var c Configuration
	c.Init()
	if len(c.Dimension) != 11 {
		t.Fatalf("expected 11 dimensions, got %d", len(c.Dimension))
	}
	for id := 1; id <= 10; id++ {
		d := c.GetDimensionByID(id)
		if d.Id != id || d.Cpu <= 0 || d.MemoryBytes <= 0 {
			t.Fatalf("dimension %d is incomplete: %+v", id, d)
		}
		parsed, err := d.ConvertMemoryToBytes(d.Memory)
		if err != nil || parsed != d.MemoryBytes {
			t.Fatalf("dimension %d memory %q parses to %v (%v), want %v", id, d.Memory, parsed, err, d.MemoryBytes)
		}
	}
}

func TestConvertMemoryToBytes(t *testing.T) {
	cases := map[string]float64{
		"8Gi": 8 * gib, "8gi": 8 * gib, "8GiB": 8 * gib, "8 Gi": 8 * gib, " 8Gi ": 8 * gib,
		"8G": 8e9, "8GB": 8e9, "8gb": 8e9,
		"4096Mi": 4096 * mib, "4096MiB": 4096 * mib, "512M": 512e6, "512MB": 512e6,
		"1048576Ki": 1 * gib, "1048576KiB": 1 * gib, "1000k": 1e6, "1000KB": 1e6,
		"1Ti": 1 * tib, "1TiB": 1 * tib, "1T": 1e12, "1TB": 1e12,
		"1.5Gi": 1.5 * gib, ".5Gi": 0.5 * gib, "2.Gi": 2 * gib,
		"8589934592": 8 * gib, "8589934592B": 8 * gib, "16Ti": 16 * tib,
	}
	var d Dimension
	// Repeat to catch any dependency on map iteration order.
	for i := 0; i < 50; i++ {
		for input, want := range cases {
			got, err := d.ConvertMemoryToBytes(input)
			if err != nil || got != want {
				t.Fatalf("ConvertMemoryToBytes(%q) = %v, %v; want %v", input, got, err, want)
			}
		}
	}
}

func TestConvertMemoryToBytesRejectsInvalidValues(t *testing.T) {
	var d Dimension
	for _, input := range []string{
		"", "Gi", "NaNGi", "InfGi", "+InfGi", "1e30Gi", "1e3", "0x10Gi", "-8Gi", "8xGi", "8XB",
		"8m", "0", "0Gi", "17Ti", "1Pi", "8 G i", "8Gi8",
	} {
		if got, err := d.ConvertMemoryToBytes(input); err == nil {
			t.Errorf("ConvertMemoryToBytes(%q) = %v, want error", input, got)
		}
	}
}

func TestResolveDimension(t *testing.T) {
	var c Configuration
	c.Init()
	d, err := c.ResolveDimension(Dimension{Id: DimensionOpen, Cpu: 4000, MemoryBytes: 8 * gib})
	if err != nil || d.Cpu != 4000 || d.MemoryBytes != 8*gib || d.MongoDBCpu != 0 {
		t.Fatalf("bad open dimension: %+v %v", d, err)
	}
	if _, err := c.ResolveDimension(Dimension{Id: 4, Cpu: 2000, Memory: "8Gi"}); err != nil {
		t.Fatalf("echoing catalog values must be accepted: %v", err)
	}
	for _, tc := range []struct {
		d    Dimension
		want string
	}{
		{Dimension{Id: 4, Cpu: 64000}, "cpu can only be set"},
		{Dimension{Id: 4, Memory: "256Gi"}, "memory can only be set"},
		{Dimension{Id: DimensionConnectionDriven}, "not supported"},
		{Dimension{Id: 11}, "unknown dimension"},
		{Dimension{Id: DimensionOpen, Cpu: 0, MemoryBytes: gib}, "requires cpu"},
		{Dimension{Id: DimensionOpen, Cpu: MaxDimensionCPU + 1, MemoryBytes: gib}, "requires cpu"},
		{Dimension{Id: DimensionOpen, Cpu: 1000}, "requires memory"},
	} {
		if _, err := c.ResolveDimension(tc.d); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ResolveDimension(%+v) error = %v, want %q", tc.d, err, tc.want)
		}
	}
}
