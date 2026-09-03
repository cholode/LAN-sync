package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	hubClientsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_hub_clients_total",
		Help: "当前 Hub 管理的客户端连接数",
	})
	hubRoomsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_hub_rooms_total",
		Help: "当前 Hub 管理的房间数",
	})
	hubDispatchedMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_hub_dispatched_messages_total",
		Help: "Hub 分发消息累计数",
	}, []string{"node_id", "room_id", "status"})
	hubDispatchLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_hub_dispatch_latency_seconds",
		Help:    "Hub 消息分发耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"node_id", "room_id"})
	hubQueueDropsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_hub_queue_drops_total",
		Help: "Hub 队列满导致的消息丢弃累计数",
	}, []string{"node_id", "room_id", "reason"})
)

func init() {
	register(hubClientsTotal)
	register(hubRoomsTotal)
	register(hubDispatchedMessagesTotal)
	register(hubDispatchLatencySeconds)
	register(hubQueueDropsTotal)
}

func SetHubClientCount(count int) {
	hubClientsTotal.Set(float64(count))
}

func SetHubRoomCount(count int) {
	hubRoomsTotal.Set(float64(count))
}

func ObserveHubDispatch(roomID int64, dispatched int) {
	hubDispatchedMessagesTotal.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10), "success").Add(float64(dispatched))
}

func ObserveHubDispatchLatency(roomID int64, seconds float64) {
	hubDispatchLatencySeconds.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10)).Observe(seconds)
}

func ObserveHubQueueDrop(roomID int64, reason string) {
	hubQueueDropsTotal.WithLabelValues(nodeID, strconv.FormatInt(roomID, 10), reason).Inc()
}
