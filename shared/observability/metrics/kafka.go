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
	kafkaProducerQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "im_kafka_producer_queue_depth",
		Help: "Gateway Kafka 有界生产队列当前深度",
	}, []string{"topic"})
	kafkaProducerQueueRejections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_kafka_producer_queue_rejections_total",
		Help: "Gateway Kafka 生产请求入队失败累计数",
	}, []string{"topic", "reason"})
	kafkaProducerBatchSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_kafka_producer_batch_size",
		Help:    "Gateway 每次写入 Kafka 的消息条数",
		Buckets: []float64{1, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{"topic"})
)

func init() {
	register(kafkaProduceTotal)
	register(kafkaProduceLatencySeconds)
	register(kafkaConsumeTotal)
	register(kafkaConsumeLatencySeconds)
	register(kafkaReadErrorsTotal)
	register(kafkaConsumerLag)
	register(kafkaProducerQueueDepth)
	register(kafkaProducerQueueRejections)
	register(kafkaProducerBatchSize)
}

func SetKafkaProducerQueueDepth(topic string, depth int) {
	kafkaProducerQueueDepth.WithLabelValues(topic).Set(float64(depth))
}

func ObserveKafkaProducerQueueRejection(topic, reason string) {
	kafkaProducerQueueRejections.WithLabelValues(topic, reason).Inc()
}

func ObserveKafkaProducerBatch(topic string, size int) {
	kafkaProducerBatchSize.WithLabelValues(topic).Observe(float64(size))
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
