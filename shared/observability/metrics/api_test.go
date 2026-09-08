package metrics

import (
	"testing"
	"time"
)

func TestAPISecondWindowUsesLatestCompleteSecond(t *testing.T) {
	window := &apiSecondWindow{}
	complete := time.Unix(100, 0)
	for _, milliseconds := range []int{1, 2, 3, 4, 100} {
		window.observe(complete, time.Duration(milliseconds)*time.Millisecond)
	}
	window.observe(time.Unix(101, 0), time.Millisecond)

	snapshot := window.latestComplete(time.Unix(101, 500_000_000))
	if snapshot.second != 100 || snapshot.samples != 5 || snapshot.p50 != 3 || snapshot.p95 != 100 || snapshot.p99 != 100 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestAPISecondWindowCapsDurationsOverTenSeconds(t *testing.T) {
	window := &apiSecondWindow{}
	window.observe(time.Unix(100, 0), 11*time.Second)

	snapshot := window.latestComplete(time.Unix(101, 0))
	if snapshot.p50 != 10000 || snapshot.p95 != 10000 || snapshot.p99 != 10000 {
		t.Fatalf("unexpected capped percentiles: %#v", snapshot)
	}
}

func TestAPISecondWindowReusesRingSlot(t *testing.T) {
	window := &apiSecondWindow{}
	window.observe(time.Unix(100, 0), 9*time.Millisecond)
	window.observe(time.Unix(103, 0), 2*time.Millisecond)

	snapshot := window.latestComplete(time.Unix(104, 0))
	if snapshot.second != 103 || snapshot.samples != 1 || snapshot.p50 != 2 {
		t.Fatalf("unexpected reused bucket: %#v", snapshot)
	}
}
