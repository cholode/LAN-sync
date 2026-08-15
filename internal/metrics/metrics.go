package metrics

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry = prometheus.NewRegistry()
	nodeID   = firstNonEmpty(os.Getenv("NODE_ID"), "lan-im-node-1")
)

func NodeID() string {
	return nodeID
}

func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func register(collector prometheus.Collector) {
	registry.MustRegister(collector)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func statusLabel(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}

func errorLabel(err error) string {
	if err == nil {
		return "none"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return "not_found"
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "deadline exceeded"):
		return "deadline"
	case strings.Contains(msg, "connection"):
		return "connection"
	case strings.Contains(msg, "context canceled"):
		return "canceled"
	default:
		return "error"
	}
}

func init() {
	buildInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "im_build_info",
		Help:        "IM 服务构建信息",
		ConstLabels: prometheus.Labels{"version": "dev"},
	})
	buildInfo.Set(1)
	register(buildInfo)

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "im_uptime_seconds",
		Help: "IM 服务进程已运行秒数",
	}, func() float64 {
		return time.Since(startTime).Seconds()
	}))
}

var startTime = time.Now()
