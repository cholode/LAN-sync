package metrics

import "github.com/prometheus/client_golang/prometheus"

type taskPoolStats interface {
	Running() int
	Waiting() int
	Cap() int
}

// RegisterTaskPoolMetrics 注册 Gateway 独占扇出协程池的监控指标。
// 显式传入协程池，避免错误采集进程级兼容协程池的数据。
func RegisterTaskPoolMetrics(pool taskPoolStats) {
	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "im_hub_task_pool_running",
		Help:        "当前任务池正在运行的任务数",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, func() float64 {
		return float64(pool.Running())
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "im_hub_task_pool_waiting",
		Help:        "当前任务池等待执行的任务数",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, func() float64 {
		return float64(pool.Waiting())
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "im_hub_task_pool_capacity",
		Help:        "当前任务池容量",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, func() float64 {
		return float64(pool.Cap())
	}))
}
