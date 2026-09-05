package metrics

import (
	"testing"
	"time"
)

func TestSecondLatencyWindowReturnsPercentilesForLatestSecond(t *testing.T) {
	window := &secondLatencyWindow{}
	now := time.Unix(100, 0)
	for _, value := range []float64{1, 2, 3, 4, 100} {
		window.add(now, value)
	}

	snapshot := window.latest(now)
	if snapshot.Samples != 5 || snapshot.P50MS != 3 || snapshot.P95MS != 100 || snapshot.P99MS != 100 {
		t.Fatalf("unexpected latency snapshot: %#v", snapshot)
	}
}

func TestSecondLatencyWindowFallsBackToPreviousSecond(t *testing.T) {
	window := &secondLatencyWindow{}
	window.add(time.Unix(99, 0), 12)

	snapshot := window.latest(time.Unix(100, 0))
	if snapshot.SecondUnix != 99 || snapshot.P50MS != 12 {
		t.Fatalf("unexpected previous-second snapshot: %#v", snapshot)
	}
}
