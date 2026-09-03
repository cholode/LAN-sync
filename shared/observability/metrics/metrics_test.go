package metrics

import "testing"

func TestRegistryIncludesRuntimeAndProcessMetrics(t *testing.T) {
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	found := make(map[string]bool, len(metricFamilies))
	for _, family := range metricFamilies {
		found[family.GetName()] = true
	}

	for _, name := range []string{
		"process_cpu_seconds_total",
		"process_resident_memory_bytes",
		"go_goroutines",
		"go_memstats_heap_alloc_bytes",
	} {
		if !found[name] {
			t.Errorf("expected metric %q to be registered", name)
		}
	}
}
