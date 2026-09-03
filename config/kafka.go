package config

import (
	"lan-im-go/internal/producer"
	"lan-im-go/pkg"
)

var KafkaProducer *producer.MessageClient

func InitKafka() {
	cfg := Messaging().Kafka
	KafkaProducer = producer.NewMessageClient(cfg.Brokers, cfg.Topic, cfg.ProducerAsync, RedisClient)
	pkg.Infof("Kafka 生产者准备就绪 brokers=%v topic=%s async=%t", cfg.Brokers, cfg.Topic, cfg.ProducerAsync)
}
