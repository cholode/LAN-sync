package config

import (
	"lan-im-go/internal/archiver"
	"lan-im-go/internal/producer"
	"lan-im-go/pkg"
	"os"
	"strings"
)

var (
	KafkaProducer *producer.MessageClient
	KafkaConsumer *archiver.Worker
)

func InitKafka() {
	kafkaAddrStr := os.Getenv("KAFKA_ADDR")
	if kafkaAddrStr == "" {
		pkg.Infoln("Kafka env配置获取失败，使用默认配置")
		kafkaAddrStr = "localhost:9092"
	}
	brokers := strings.Split(kafkaAddrStr, ",")

	KafkaProducer = producer.NewMessageClient(brokers, "im_chat_messages", RedisClient)
	pkg.Infoln("Kafka 生产者准备就绪")

	// 2. 实例化后台稳态消费者(需要注入你的 MySQL 存储引擎实体)
	// KafkaConsumer = archiver.NewWorker(brokers, "im_chat_messages", "im_archiver_group", yourMySQLRepoInstance)
	// pkg.Infoln("Kafka 稳态消费者已就位")
}