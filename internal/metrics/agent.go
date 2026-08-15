package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	agentRoomsEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_agent_rooms_enabled",
		Help: "当前启用 Agent 的房间数",
	})
	agentInflightRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "im_agent_inflight_requests",
		Help: "当前正在处理的 Agent 请求数",
	}, []string{"node_id", "room_id"})
	agentMessagesReceivedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_agent_messages_received_total",
		Help: "Agent 收到消息累计数",
	}, []string{"node_id", "room_id", "source"})
	agentMessagesTriggeredTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_agent_messages_triggered_total",
		Help: "Agent 触发回复累计数",
	}, []string{"node_id", "room_id", "trigger_type"})
	agentProcessedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_agent_processed_total",
		Help: "Agent 处理请求累计数",
	}, []string{"node_id", "model", "status"})
	agentErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_agent_errors_total",
		Help: "Agent 错误累计数",
	}, []string{"node_id", "error_type"})
	agentReplyLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_agent_reply_latency_seconds",
		Help:    "Agent 回复耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"node_id", "model", "room_id"})
)

func init() {
	register(agentRoomsEnabled)
	register(agentInflightRequests)
	register(agentMessagesReceivedTotal)
	register(agentMessagesTriggeredTotal)
	register(agentProcessedTotal)
	register(agentErrorsTotal)
	register(agentReplyLatencySeconds)
}

func SetAgentRoomsEnabled(count int) {
	agentRoomsEnabled.Set(float64(count))
}

func ObserveAgentMessageReceived(roomID int64, source string) {
	agentMessagesReceivedTotal.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10), source).Inc()
}

func ObserveAgentMessageTriggered(roomID int64, triggerType string) {
	agentMessagesTriggeredTotal.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10), triggerType).Inc()
}

func AgentRequestStarted(roomID int64) {
	agentInflightRequests.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10)).Inc()
}

func AgentRequestFinished(roomID int64, model, status string, start time.Time, err error) {
	agentInflightRequests.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10)).Dec()
	agentProcessedTotal.WithLabelValues(nodeID, model, status).Inc()
	agentReplyLatencySeconds.WithLabelValues(nodeID, model, strconv.FormatInt(roomID, 10)).Observe(time.Since(start).Seconds())
	if err != nil {
		agentErrorsTotal.WithLabelValues(nodeID, errorLabel(err)).Inc()
	}
}
