package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	kafkaProduceTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_kafka_produce_total",
		Help: "Kafka 生产消息累计数",
	}, []string{"topic", "status"})
	kafkaProduceLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_kafka_produce_latency_seconds",
		Help:    "Kafka 生产消息耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})
	kafkaConsumeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_kafka_consume_total",
		Help: "Kafka 消费消息累计数",
	}, []string{"topic", "status"})
	kafkaConsumeLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_kafka_consume_latency_seconds",
		Help:    "Kafka 消费消息耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})
	kafkaReadErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_kafka_read_errors_total",
		Help: "Kafka 读取错误累计数",
	}, []string{"topic", "error_type"})
	kafkaConsumerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "im_kafka_consumer_lag",
		Help: "Kafka 消费者当前滞后量",
	}, []string{"topic", "partition"})
)

func init() {
	register(kafkaProduceTotal)
	register(kafkaProduceLatencySeconds)
	register(kafkaConsumeTotal)
	register(kafkaConsumeLatencySeconds)
	register(kafkaReadErrorsTotal)
	register(kafkaConsumerLag)
}

func ObserveKafkaProduce(topic string, start time.Time, err error) {
	status := statusLabel(err)
	kafkaProduceTotal.WithLabelValues(topic, status).Inc()
	kafkaProduceLatencySeconds.WithLabelValues(topic).Observe(time.Since(start).Seconds())
}

func ObserveKafkaConsume(topic string, start time.Time, err error) {
	status := statusLabel(err)
	kafkaConsumeTotal.WithLabelValues(topic, status).Inc()
	kafkaConsumeLatencySeconds.WithLabelValues(topic).Observe(time.Since(start).Seconds())
}

func ObserveKafkaReadError(topic string, err error) {
	kafkaReadErrorsTotal.WithLabelValues(topic, errorLabel(err)).Inc()
}

func SetKafkaConsumerLag(topic string, partition int, lag int64) {
	if lag < 0 {
		return
	}
	kafkaConsumerLag.WithLabelValues(topic, strconv.Itoa(partition)).Set(float64(lag))
}
