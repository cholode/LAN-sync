package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	wsConnectionsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "im_ws_connections_active",
		Help: "Current active WebSocket connections",
	}, []string{"node_id"})
	wsConnectionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_connections_total",
		Help: "Total WebSocket connections",
	}, []string{"node_id", "status", "close_reason"})
	wsConnectionDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_ws_connection_duration_seconds",
		Help:    "WebSocket connection duration distribution",
		Buckets: prometheus.DefBuckets,
	}, []string{"node_id", "close_reason"})
	wsReadMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_read_messages_total",
		Help: "Total WebSocket read messages",
	}, []string{"node_id", "message_type"})
	wsWriteMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_write_messages_total",
		Help: "Total WebSocket written messages",
	}, []string{"node_id", "message_type"})
	wsWriteFramesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_write_frames_total",
		Help: "Total WebSocket frames written after message aggregation",
	}, []string{"node_id", "message_type"})
	wsWriteBatchMessages = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_ws_write_batch_messages",
		Help:    "Number of logical messages carried by each WebSocket frame",
		Buckets: []float64{1, 2, 4, 8, 16, 32, 64},
	}, []string{"node_id", "message_type"})
	gatewayMessageTransitSeconds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_gateway_message_transit_seconds_total",
		Help: "Accumulated server-side time from Gateway ingress to successful WebSocket frame write",
	}, []string{"node_id"})
	gatewayMessageTransitTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_gateway_message_transit_total",
		Help: "Logical messages included in Gateway server-side transit latency",
	}, []string{"node_id"})
	gatewayFrameOldestMessageLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_gateway_frame_oldest_message_latency_seconds",
		Help:    "Server-side transit latency of the oldest logical message in each successful WebSocket frame",
		Buckets: []float64{0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.15, 0.2, 0.3, 0.5, 0.75, 1, 2, 5},
	}, []string{"node_id"})
	wsReadErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_read_errors_total",
		Help: "Total WebSocket read errors",
	}, []string{"node_id", "error_type"})
	wsWriteErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_ws_write_errors_total",
		Help: "Total WebSocket write errors",
	}, []string{"node_id", "error_type"})
)

func init() {
	register(wsConnectionsActive)
	register(wsConnectionsTotal)
	register(wsConnectionDurationSeconds)
	register(wsReadMessagesTotal)
	register(wsWriteMessagesTotal)
	register(wsWriteFramesTotal)
	register(wsWriteBatchMessages)
	register(gatewayMessageTransitSeconds)
	register(gatewayMessageTransitTotal)
	register(gatewayFrameOldestMessageLatency)
	register(wsReadErrorsTotal)
	register(wsWriteErrorsTotal)
}

func WSConnected() {
	wsConnectionsActive.WithLabelValues(nodeID).Inc()
	RecordWSConnected()
}

func WSDisconnected(duration time.Duration, closeReason string) {
	wsConnectionsActive.WithLabelValues(nodeID).Dec()
	if closeReason == "" {
		closeReason = "normal"
	}
	wsConnectionsTotal.WithLabelValues(nodeID, "success", closeReason).Inc()
	wsConnectionDurationSeconds.WithLabelValues(nodeID, closeReason).Observe(duration.Seconds())
	RecordWSDisconnected(duration, closeReason != "normal")
}

func ObserveWSReadMessage(messageType int) {
	wsReadMessagesTotal.WithLabelValues(nodeID, wsMessageType(messageType)).Inc()
	RecordWSReadMessage()
}

func ObserveWSWriteMessage(messageType int) {
	ObserveWSWriteBatch(messageType, 1)
}

// ObserveWSWriteBatch 记录一个成功写出的 WebSocket 帧及其中承载的业务消息数。
func ObserveWSWriteBatch(messageType, messages int) {
	if messages <= 0 {
		return
	}
	labels := []string{nodeID, wsMessageType(messageType)}
	wsWriteMessagesTotal.WithLabelValues(labels...).Add(float64(messages))
	wsWriteFramesTotal.WithLabelValues(labels...).Inc()
	wsWriteBatchMessages.WithLabelValues(labels...).Observe(float64(messages))
	RecordWSWriteMessages(messages)
}

// ObserveGatewayFrameTransit 批量记录消息在 Gateway 内部的停留时间。
// latencySecondsSum 是当前帧内所有逻辑消息延迟之和，oldestSeconds 用于观察尾延迟。
func ObserveGatewayFrameTransit(messages int, latencySecondsSum, oldestSeconds float64) {
	if messages <= 0 || latencySecondsSum < 0 || oldestSeconds < 0 {
		return
	}
	gatewayMessageTransitSeconds.WithLabelValues(nodeID).Add(latencySecondsSum)
	gatewayMessageTransitTotal.WithLabelValues(nodeID).Add(float64(messages))
	gatewayFrameOldestMessageLatency.WithLabelValues(nodeID).Observe(oldestSeconds)
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
