package metrics

import (
	"lan-im-go/shared/concurrency/taskpool"

	"github.com/prometheus/client_golang/prometheus"
)

func RegisterTaskPoolMetrics() {
	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "im_hub_task_pool_running",
		Help: "当前任务池正在运行的任务数",
	}, func() float64 {
		return float64(taskpool.Running())
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "im_hub_task_pool_waiting",
		Help: "当前任务池等待执行的任务数",
	}, func() float64 {
		return float64(taskpool.Waiting())
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "im_hub_task_pool_capacity",
		Help: "当前任务池容量",
	}, func() float64 {
		return float64(taskpool.Cap())
	}))
}
