package config

import (
	"lan-im-go/internal/archiver"
	"lan-im-go/internal/producer"
	"log"
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
		kafkaAddrStr = "localhost:9092"
	}
	brokers := strings.Split(kafkaAddrStr, ",")

	//1.实例化kafka生产这供websocket调用
	KafkaProducer = producer.NewMessageClient(brokers, "im_chat_messages")
	log.Println("Kafka 生产者准备就绪")

	// 2. 实例化后台稳态消费者 (需注入你的 MySQL 存储引擎实体)
	// KafkaConsumer = archiver.NewWorker(brokers, "im_chat_messages", "im_archiver_group", yourMySQLRepoInstance)
	// log.Println("Kafka 稳态消费者已就位")
}
