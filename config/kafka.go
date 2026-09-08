package config

import (
	"lan-im-go/pkg"
	"lan-im-go/services/messages/producer"
)

var KafkaProducer *producer.MessageClient

func InitKafka() {
	cfg := Messaging().Kafka
	KafkaProducer = producer.NewMessageClient(cfg.Brokers, cfg.IngressTopic)
	pkg.Infof("Kafka 生产者准备就绪 brokers=%v ingress=%s，同步等待确认", cfg.Brokers, cfg.IngressTopic)
}
