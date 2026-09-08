package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	apiLatencyBucketMilliseconds = 10000
	apiSecondBucketCount         = 3
)

var (
	apiRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_api_requests_total",
		Help: "Completed HTTP API requests.",
	}, []string{"component", "method", "route", "status"})
	apiRequestDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_api_request_duration_seconds",
		Help:    "End-to-end HTTP API request duration in seconds.",
		Buckets: []float64{0.001, 0.003, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
	}, []string{"component", "method", "route"})
	apiServiceWindows = struct {
		sync.RWMutex
		values map[string]*apiSecondWindow
	}{values: make(map[string]*apiSecondWindow)}
)

type apiLatencySnapshot struct {
	second  int64
	samples uint64
	p50     float64
	p95     float64
	p99     float64
}

type apiSecondBucket struct {
	mu      sync.Mutex
	second  int64
	counts  [apiLatencyBucketMilliseconds + 1]uint64
	samples uint64
}

// apiSecondWindow keeps the current second, the latest complete second, and one
// spare second. Each completed request increments one fixed 1 ms latency bucket.
type apiSecondWindow struct {
	buckets [apiSecondBucketCount]apiSecondBucket
}

func (w *apiSecondWindow) observe(at time.Time, duration time.Duration) {
	second := at.Unix()
	bucket := &w.buckets[int(second%apiSecondBucketCount)]
	bucket.mu.Lock()
	if bucket.second != second {
		clear(bucket.counts[:])
		bucket.second = second
		bucket.samples = 0
	}
	index := apiLatencyIndex(duration)
	bucket.counts[index]++
	bucket.samples++
	bucket.mu.Unlock()
}

func apiLatencyIndex(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	microseconds := duration.Microseconds()
	index := int((microseconds - 1) / 1000)
	if index >= apiLatencyBucketMilliseconds {
		return apiLatencyBucketMilliseconds
	}
	return index
}

// latestComplete returns the newest finished second. It deliberately does not
// read the current, still-growing second so QPS and percentiles share one sample set.
func (w *apiSecondWindow) latestComplete(now time.Time) apiLatencySnapshot {
	second := now.Unix() - 1
	bucket := &w.buckets[int(second%apiSecondBucketCount)]
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	if bucket.second != second || bucket.samples == 0 {
		return apiLatencySnapshot{second: second}
	}
	return apiLatencySnapshot{
		second:  second,
		samples: bucket.samples,
		p50:     apiBucketPercentile(bucket.counts[:], bucket.samples, 0.50),
		p95:     apiBucketPercentile(bucket.counts[:], bucket.samples, 0.95),
		p99:     apiBucketPercentile(bucket.counts[:], bucket.samples, 0.99),
	}
}

func apiBucketPercentile(counts []uint64, samples uint64, percentile float64) float64 {
	if samples == 0 {
		return 0
	}
	target := uint64(float64(samples)*percentile + 0.999999999)
	var cumulative uint64
	for index, count := range counts {
		cumulative += count
		if cumulative >= target {
			if index == apiLatencyBucketMilliseconds {
				return apiLatencyBucketMilliseconds
			}
			return float64(index + 1)
		}
	}
	return apiLatencyBucketMilliseconds
}

func apiWindow(service string) *apiSecondWindow {
	apiServiceWindows.RLock()
	window := apiServiceWindows.values[service]
	apiServiceWindows.RUnlock()
	if window != nil {
		return window
	}
	apiServiceWindows.Lock()
	defer apiServiceWindows.Unlock()
	if window = apiServiceWindows.values[service]; window == nil {
		window = &apiSecondWindow{}
		apiServiceWindows.values[service] = window
	}
	return window
}

// ObserveServiceAPIRequest records a request only after its full handler chain
// has returned. The duration starts when the request enters the service router.
func ObserveServiceAPIRequest(service, method, route string, status int, duration time.Duration) {
	if service == "" {
		service = "unknown"
	}
	if route == "" {
		route = "unmatched"
	}
	apiRequestsTotal.WithLabelValues(service, method, route, strconv.Itoa(status)).Inc()
	apiRequestDurationSeconds.WithLabelValues(service, method, route).Observe(duration.Seconds())
	apiWindow(service).observe(time.Now(), duration)
	ObserveAPIRequest(status, duration)
}

type apiSecondCollector struct {
	qpsDesc     *prometheus.Desc
	samplesDesc *prometheus.Desc
	latencyDesc *prometheus.Desc
	secondDesc  *prometheus.Desc
}

func newAPISecondCollector() *apiSecondCollector {
	return &apiSecondCollector{
		qpsDesc:     prometheus.NewDesc("im_api_completed_qps", "Completed end-to-end API requests in the latest complete second.", []string{"component"}, nil),
		samplesDesc: prometheus.NewDesc("im_api_completion_samples", "API completion samples in the latest complete second.", []string{"component"}, nil),
		latencyDesc: prometheus.NewDesc("im_api_completion_latency_milliseconds", "API latency percentile for the latest complete second, capped at 10000 ms.", []string{"component", "quantile"}, nil),
		secondDesc:  prometheus.NewDesc("im_api_completion_second", "Unix second represented by the API completion metrics.", []string{"component"}, nil),
	}
}

func (c *apiSecondCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.qpsDesc
	ch <- c.samplesDesc
	ch <- c.latencyDesc
	ch <- c.secondDesc
}

func (c *apiSecondCollector) Collect(ch chan<- prometheus.Metric) {
	apiServiceWindows.RLock()
	defer apiServiceWindows.RUnlock()
	now := time.Now()
	for service, window := range apiServiceWindows.values {
		snapshot := window.latestComplete(now)
		ch <- prometheus.MustNewConstMetric(c.qpsDesc, prometheus.GaugeValue, float64(snapshot.samples), service)
		ch <- prometheus.MustNewConstMetric(c.samplesDesc, prometheus.GaugeValue, float64(snapshot.samples), service)
		ch <- prometheus.MustNewConstMetric(c.secondDesc, prometheus.GaugeValue, float64(snapshot.second), service)
		ch <- prometheus.MustNewConstMetric(c.latencyDesc, prometheus.GaugeValue, snapshot.p50, service, "0.50")
		ch <- prometheus.MustNewConstMetric(c.latencyDesc, prometheus.GaugeValue, snapshot.p95, service, "0.95")
		ch <- prometheus.MustNewConstMetric(c.latencyDesc, prometheus.GaugeValue, snapshot.p99, service, "0.99")
	}
}

func init() {
	register(apiRequestsTotal)
	register(apiRequestDurationSeconds)
	register(newAPISecondCollector())
}
