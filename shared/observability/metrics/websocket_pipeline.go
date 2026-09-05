package metrics

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	WSStageAuth        = "auth"
	WSStageMembership  = "membership"
	WSStageUpgrade     = "upgrade"
	WSStageHubRegister = "hub_register"
	WSStageRedisOnline = "redis_online"
	WSStageReadyTotal  = "ready_total"
)

var wsPipelineStages = []string{
	WSStageAuth,
	WSStageMembership,
	WSStageUpgrade,
	WSStageHubRegister,
	WSStageRedisOnline,
	WSStageReadyTotal,
}

type LatencyPercentiles struct {
	SecondUnix int64   `json:"second_unix"`
	Samples    int     `json:"samples"`
	P50MS      float64 `json:"p50_ms"`
	P95MS      float64 `json:"p95_ms"`
	P99MS      float64 `json:"p99_ms"`
}

type WSConnectionPipelineRuntime struct {
	Auth        LatencyPercentiles `json:"auth"`
	Membership  LatencyPercentiles `json:"membership"`
	Upgrade     LatencyPercentiles `json:"upgrade"`
	HubRegister LatencyPercentiles `json:"hub_register"`
	RedisOnline LatencyPercentiles `json:"redis_online"`
	ReadyTotal  LatencyPercentiles `json:"ready_total"`
}

type latencySecondBucket struct {
	second int64
	values []float64
}

// secondLatencyWindow 保留最近几秒的独立样本桶。读取时优先返回当前秒，
// 当前秒没有样本则返回上一秒，避免页面刚好跨秒刷新时短暂显示全零。
type secondLatencyWindow struct {
	mu      sync.Mutex
	buckets [3]latencySecondBucket
}

func (w *secondLatencyWindow) add(at time.Time, milliseconds float64) {
	second := at.Unix()
	index := int(second % int64(len(w.buckets)))
	w.mu.Lock()
	bucket := &w.buckets[index]
	if bucket.second != second {
		bucket.second = second
		bucket.values = bucket.values[:0]
	}
	bucket.values = append(bucket.values, milliseconds)
	w.mu.Unlock()
}

func (w *secondLatencyWindow) latest(now time.Time) LatencyPercentiles {
	w.mu.Lock()
	defer w.mu.Unlock()
	for second := now.Unix(); second >= now.Unix()-2; second-- {
		bucket := &w.buckets[int(second%int64(len(w.buckets)))]
		if bucket.second != second || len(bucket.values) == 0 {
			continue
		}
		values := append([]float64(nil), bucket.values...)
		sort.Float64s(values)
		return LatencyPercentiles{
			SecondUnix: second,
			Samples:    len(values),
			P50MS:      percentileValue(values, 0.50),
			P95MS:      percentileValue(values, 0.95),
			P99MS:      percentileValue(values, 0.99),
		}
	}
	return LatencyPercentiles{}
}

func percentileValue(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}

var (
	wsPipelineWindows = map[string]*secondLatencyWindow{
		WSStageAuth: {}, WSStageMembership: {}, WSStageUpgrade: {},
		WSStageHubRegister: {}, WSStageRedisOnline: {}, WSStageReadyTotal: {},
	}
	wsPipelineLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_ws_connection_pipeline_duration_seconds",
		Help:    "WebSocket 建连各阶段耗时",
		Buckets: []float64{0.001, 0.003, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"node_id", "stage", "result"})
	wsPipelineTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_connection_pipeline_total",
		Help: "WebSocket 建连各阶段执行次数",
	}, []string{"node_id", "stage", "result"})
)

func init() {
	register(wsPipelineLatency)
	register(wsPipelineTotal)
	for _, stageName := range wsPipelineStages {
		stage := stageName
		register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "im_ws_connection_stage_second_samples",
			Help:        "WebSocket 建连阶段最近一秒样本数",
			ConstLabels: prometheus.Labels{"node_id": nodeID, "stage": stage},
		}, func() float64 {
			return float64(wsPipelineWindows[stage].latest(time.Now()).Samples)
		}))
		for _, quantileValue := range []struct {
			label string
			read  func(LatencyPercentiles) float64
		}{
			{label: "0.50", read: func(value LatencyPercentiles) float64 { return value.P50MS }},
			{label: "0.95", read: func(value LatencyPercentiles) float64 { return value.P95MS }},
			{label: "0.99", read: func(value LatencyPercentiles) float64 { return value.P99MS }},
		} {
			quantile := quantileValue
			register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "im_ws_connection_stage_second_latency_milliseconds",
				Help: "WebSocket 建连阶段最近一秒延迟分位值（毫秒）",
				ConstLabels: prometheus.Labels{
					"node_id":  nodeID,
					"stage":    stage,
					"quantile": quantile.label,
				},
			}, func() float64 {
				return quantile.read(wsPipelineWindows[stage].latest(time.Now()))
			}))
		}
	}
}

func ObserveWSConnectionStage(stage string, startedAt time.Time, result string) {
	duration := time.Since(startedAt)
	if duration < 0 {
		duration = 0
	}
	if result == "" {
		result = "success"
	}
	wsPipelineLatency.WithLabelValues(nodeID, stage, result).Observe(duration.Seconds())
	wsPipelineTotal.WithLabelValues(nodeID, stage, result).Inc()
	if window := wsPipelineWindows[stage]; window != nil {
		window.add(time.Now(), float64(duration.Microseconds())/1000)
	}
}

func WSConnectionPipelineSnapshotNow() WSConnectionPipelineRuntime {
	now := time.Now()
	return WSConnectionPipelineRuntime{
		Auth:        wsPipelineWindows[WSStageAuth].latest(now),
		Membership:  wsPipelineWindows[WSStageMembership].latest(now),
		Upgrade:     wsPipelineWindows[WSStageUpgrade].latest(now),
		HubRegister: wsPipelineWindows[WSStageHubRegister].latest(now),
		RedisOnline: wsPipelineWindows[WSStageRedisOnline].latest(now),
		ReadyTotal:  wsPipelineWindows[WSStageReadyTotal].latest(now),
	}
}
