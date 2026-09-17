package config

import (
	"lan-im-go/services/messages/producer"
	"lan-im-go/shared/observability/logger"
)

var KafkaProducer *producer.MessageClient

func InitKafka() {
	cfg := Messaging().Kafka
	KafkaProducer = producer.NewMessageClient(cfg.Brokers, cfg.IngressTopic, producer.BatchOptions{
		MaxMessages:   cfg.ProducerBatchMessages,
		MaxBytes:      cfg.ProducerBatchBytes,
		MaxWait:       cfg.ProducerBatchWait,
		QueueCapacity: cfg.ProducerQueueCapacity,
		WriteTimeout:  cfg.ProducerWriteTimeout,
	})
	logger.Infof("Kafka 批量生产者准备就绪 brokers=%v ingress=%s batch=%d/%dB/%s queue=%d", cfg.Brokers, cfg.IngressTopic, cfg.ProducerBatchMessages, cfg.ProducerBatchBytes, cfg.ProducerBatchWait, cfg.ProducerQueueCapacity)
}
