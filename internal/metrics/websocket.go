package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	wsConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_ws_connections_active",
		Help: "当前活跃的 WebSocket 连接数",
	})
	wsConnectionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_connections_total",
		Help: "WebSocket 连接累计数",
	}, []string{"node_id", "status", "close_reason"})
	wsConnectionDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_ws_connection_duration_seconds",
		Help:    "WebSocket 连接存活时长分布",
		Buckets: prometheus.DefBuckets,
	}, []string{"node_id", "close_reason"})
	wsReadMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_read_messages_total",
		Help: "WebSocket 读取消息累计数",
	}, []string{"node_id", "message_type"})
	wsWriteMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_write_messages_total",
		Help: "WebSocket 写出消息累计数",
	}, []string{"node_id", "message_type"})
	wsReadErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_read_errors_total",
		Help: "WebSocket 读取错误累计数",
	}, []string{"node_id", "error_type"})
	wsWriteErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_write_errors_total",
		Help: "WebSocket 写出错误累计数",
	}, []string{"node_id", "error_type"})
)

func init() {
	register(wsConnectionsActive)
	register(wsConnectionsTotal)
	register(wsConnectionDurationSeconds)
	register(wsReadMessagesTotal)
	register(wsWriteMessagesTotal)
	register(wsReadErrorsTotal)
	register(wsWriteErrorsTotal)
}

func WSConnected() {
	wsConnectionsActive.Inc()
}

func WSDisconnected(duration time.Duration, closeReason string) {
	wsConnectionsActive.Dec()
	if closeReason == "" {
		closeReason = "normal"
	}
	wsConnectionsTotal.WithLabelValues(nodeID, "success", closeReason).Inc()
	wsConnectionDurationSeconds.WithLabelValues(nodeID, closeReason).Observe(duration.Seconds())
}

func ObserveWSReadMessage(messageType int) {
	wsReadMessagesTotal.WithLabelValues(nodeID, wsMessageType(messageType)).Inc()
}

func ObserveWSWriteMessage(messageType int) {
	wsWriteMessagesTotal.WithLabelValues(nodeID, wsMessageType(messageType)).Inc()
}

func ObserveWSReadError(err error) {
	wsReadErrorsTotal.WithLabelValues(nodeID, errorLabel(err)).Inc()
}

func ObserveWSWriteError(err error) {
	wsWriteErrorsTotal.WithLabelValues(nodeID, errorLabel(err)).Inc()
}

func wsMessageType(messageType int) string {
	switch messageType {
	case 1:
		return "text"
	case 2:
		return "binary"
	default:
		return "type_" + strconv.Itoa(messageType)
	}
}
