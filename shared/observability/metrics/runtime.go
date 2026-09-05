package metrics

import (
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	wsCurrent          int64
	wsEstablishedTotal int64
	wsClosedTotal      int64
	wsAbnormalTotal    int64
	wsDurationSumMS    int64
	wsDurationCount    int64
	wsSendQueueBacklog int64
	wsSlowClients      int64
)

var (
	apiRequestsLastMinute = newSecondWindow(300)
	apiRequestsWindow     = apiRequestsLastMinute
	wsReadWindow          = newSecondWindow(300)
	wsWriteWindow         = newSecondWindow(300)
	apiLatencyWindow      = newLatencyWindow(1024)
	api4xx                int64
	api5xx                int64
	apiTotal              int64
	apiLatencySumMS       int64
	apiLatencyCount       int64
)

// RuntimeSnapshot 汇总运行时指标，供管理端和 Prometheus 使用。
type RuntimeSnapshot struct {
	WebSocket WebSocketRuntime `json:"websocket"`
	Golang    GoRuntime        `json:"golang"`
	API       APIRuntime       `json:"api"`
}

type WebSocketRuntime struct {
	CurrentConnections          int64   `json:"current_connections"`
	EstablishedTotal            int64   `json:"established_total"`
	ClosedTotal                 int64   `json:"closed_total"`
	AbnormalClosedTotal         int64   `json:"abnormal_closed_total"`
	AverageConnectionDurationMS float64 `json:"average_connection_duration_ms"`
	ReadMessagesPerMinute       int64   `json:"read_messages_per_minute"`
	WriteMessagesPerMinute      int64   `json:"write_messages_per_minute"`
	SendQueueBacklog            int64   `json:"send_queue_backlog"`
	SlowClients                 int64   `json:"slow_clients"`
}

type GoRuntime struct {
	Goroutines    int     `json:"goroutines"`
	GOMAXPROCS    int     `json:"gomaxprocs"`
	HeapAlloc     uint64  `json:"heap_alloc"`
	HeapSys       uint64  `json:"heap_sys"`
	GCCount       uint32  `json:"gc_count"`
	LastGCUnix    int64   `json:"last_gc_unix"`
	GCPauseNS     uint64  `json:"gc_pause_ns"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

type APIRuntime struct {
	QPS1m            float64 `json:"qps_1m"`
	QPS5m            float64 `json:"qps_5m"`
	AverageLatencyMS float64 `json:"average_latency_ms"`
	P50LatencyMS     float64 `json:"p50_latency_ms"`
	P95LatencyMS     float64 `json:"p95_latency_ms"`
	P99LatencyMS     float64 `json:"p99_latency_ms"`
	Status4xx        int64   `json:"status_4xx"`
	Status5xx        int64   `json:"status_5xx"`
	ErrorRate        float64 `json:"error_rate"`
}

// RecordWSConnected 记录一次 WebSocket 建立并增加当前连接数。
func RecordWSConnected() {
	atomic.AddInt64(&wsCurrent, 1)
	atomic.AddInt64(&wsEstablishedTotal, 1)
}

// RecordWSDisconnected 记录一次 WebSocket 断开。
func RecordWSDisconnected(duration time.Duration, abnormal bool) {
	if atomic.AddInt64(&wsCurrent, -1) < 0 {
		atomic.StoreInt64(&wsCurrent, 0)
	}
	atomic.AddInt64(&wsClosedTotal, 1)
	if abnormal {
		atomic.AddInt64(&wsAbnormalTotal, 1)
	}
	ms := duration.Milliseconds()
	atomic.AddInt64(&wsDurationSumMS, ms)
	atomic.AddInt64(&wsDurationCount, 1)
}

// RecordWSReadMessage 记录一条从 WebSocket 读取的消息。
func RecordWSReadMessage() {
	wsReadWindow.Add(time.Now().Unix())
}

// RecordWSWriteMessage 记录一条写入 WebSocket 的消息。
func RecordWSWriteMessage() {
	wsWriteWindow.Add(time.Now().Unix())
}

// SetWSSendQueueBacklog 更新发送队列积压量。
func SetWSSendQueueBacklog(backlog int) {
	atomic.StoreInt64(&wsSendQueueBacklog, int64(backlog))
}

// RecordWSSlowClient 记录一次 Hub 慢客户端事件。
func RecordWSSlowClient() {
	atomic.AddInt64(&wsSlowClients, 1)
}

// ObserveAPIRequest 记录 HTTP API 请求，用于计算 QPS、延迟和错误率。
func ObserveAPIRequest(status int, duration time.Duration) {
	now := time.Now().Unix()
	apiRequestsWindow.Add(now)
	atomic.AddInt64(&apiTotal, 1)
	ms := float64(duration.Microseconds()) / 1000.0
	apiLatencyWindow.Add(ms)
	atomic.AddInt64(&apiLatencySumMS, int64(ms))
	atomic.AddInt64(&apiLatencyCount, 1)
	if status >= 500 {
		atomic.AddInt64(&api5xx, 1)
	} else if status >= 400 {
		atomic.AddInt64(&api4xx, 1)
	}
}

// RuntimeSnapshotNow 返回当前运行时指标快照。
func RuntimeSnapshotNow() RuntimeSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	wsCount := atomic.LoadInt64(&wsDurationCount)
	var wsAvg float64
	if wsCount > 0 {
		wsAvg = float64(atomic.LoadInt64(&wsDurationSumMS)) / float64(wsCount)
	}

	apiTotalValue := atomic.LoadInt64(&apiTotal)
	var apiAvg float64
	apiCount := atomic.LoadInt64(&apiLatencyCount)
	if apiCount > 0 {
		apiAvg = float64(atomic.LoadInt64(&apiLatencySumMS)) / float64(apiCount)
	}
	api1m := apiRequestsWindow.Sum(60)
	api5m := apiRequestsWindow.Sum(300)
	errTotal := atomic.LoadInt64(&api4xx) + atomic.LoadInt64(&api5xx)
	var errRate float64
	if apiTotalValue > 0 {
		errRate = float64(errTotal) / float64(apiTotalValue)
	}

	return RuntimeSnapshot{
		WebSocket: WebSocketRuntime{
			CurrentConnections:          atomic.LoadInt64(&wsCurrent),
			EstablishedTotal:            atomic.LoadInt64(&wsEstablishedTotal),
			ClosedTotal:                 atomic.LoadInt64(&wsClosedTotal),
			AbnormalClosedTotal:         atomic.LoadInt64(&wsAbnormalTotal),
			AverageConnectionDurationMS: wsAvg,
			ReadMessagesPerMinute:       wsReadWindow.Sum(60),
			WriteMessagesPerMinute:      wsWriteWindow.Sum(60),
			SendQueueBacklog:            atomic.LoadInt64(&wsSendQueueBacklog),
			SlowClients:                 atomic.LoadInt64(&wsSlowClients),
		},
		Golang: GoRuntime{
			Goroutines:    runtime.NumGoroutine(),
			GOMAXPROCS:    runtime.GOMAXPROCS(0),
			HeapAlloc:     mem.HeapAlloc,
			HeapSys:       mem.HeapSys,
			GCCount:       mem.NumGC,
			LastGCUnix:    int64(mem.LastGC / 1e9),
			GCPauseNS:     mem.PauseNs[(mem.NumGC+255)%256],
			UptimeSeconds: time.Since(startTime).Seconds(),
		},
		API: APIRuntime{
			QPS1m:            float64(api1m) / 60.0,
			QPS5m:            float64(api5m) / 300.0,
			AverageLatencyMS: apiAvg,
			P50LatencyMS:     apiLatencyWindow.Percentile(0.50),
			P95LatencyMS:     apiLatencyWindow.Percentile(0.95),
			P99LatencyMS:     apiLatencyWindow.Percentile(0.99),
			Status4xx:        atomic.LoadInt64(&api4xx),
			Status5xx:        atomic.LoadInt64(&api5xx),
			ErrorRate:        errRate,
		},
	}
}

type secondWindow struct {
	mu        sync.Mutex
	buckets   []int64
	lastSec   int64
	lastIndex int
}

func newSecondWindow(size int) *secondWindow {
	return &secondWindow{buckets: make([]int64, size)}
}

func (w *secondWindow) Add(now int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	idx := int(now % int64(len(w.buckets)))
	if now == w.lastSec {
		w.buckets[idx]++
		return
	}
	if now > w.lastSec {
		// 清空时间跳跃期间已经过期的桶。
		steps := int(now - w.lastSec)
		if steps > len(w.buckets) {
			steps = len(w.buckets)
		}
		for i := 1; i <= steps; i++ {
			clearIdx := (idx - i + len(w.buckets)) % len(w.buckets)
			w.buckets[clearIdx] = 0
		}
	}
	w.buckets[idx] = 1
	w.lastSec = now
	w.lastIndex = idx
}

func (w *secondWindow) Sum(lastSeconds int) int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if lastSeconds <= 0 || lastSeconds > len(w.buckets) {
		lastSeconds = len(w.buckets)
	}
	var sum int64
	now := time.Now().Unix()
	start := now - int64(lastSeconds) + 1
	for i := 0; i < lastSeconds; i++ {
		sec := start + int64(i)
		sum += w.buckets[int(sec%int64(len(w.buckets)))]
	}
	return sum
}

type latencyWindow struct {
	mu     sync.Mutex
	values []float64
	cursor int
	count  int
}

func newLatencyWindow(size int) *latencyWindow {
	return &latencyWindow{values: make([]float64, size)}
}

func (w *latencyWindow) Add(value float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.values[w.cursor] = value
	w.cursor = (w.cursor + 1) % len(w.values)
	if w.count < len(w.values) {
		w.count++
	}
}

func (w *latencyWindow) Percentile(p float64) float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.count == 0 {
		return 0
	}
	values := make([]float64, w.count)
	copy(values, w.values[:w.count])
	sort.Float64s(values)
	index := int(math.Ceil(p*float64(w.count))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}
