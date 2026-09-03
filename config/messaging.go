package config

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRedisAddr          = "localhost:6379"
	defaultKafkaBroker        = "localhost:9092"
	defaultKafkaTopic         = "im_chat_messages"
	defaultKafkaArchiverGroup = "im_archiver_group"
)

// MessagingConfig is the single source of truth for Redis and Kafka settings.
// KAFKA_ADDR remains supported for backwards compatibility; new deployments
// should use the clearer KAFKA_BROKERS name.
type MessagingConfig struct {
	Redis RedisConfig
	Kafka KafkaConfig
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type KafkaConfig struct {
	Brokers       []string
	Topic         string
	ArchiverGroup string
	ProducerAsync bool
}

var (
	messagingConfigOnce sync.Once
	messagingConfig     MessagingConfig
)

func Messaging() MessagingConfig {
	messagingConfigOnce.Do(func() {
		brokersValue := envString("KAFKA_BROKERS", "")
		if brokersValue == "" {
			brokersValue = envString("KAFKA_ADDR", defaultKafkaBroker)
		}

		messagingConfig = MessagingConfig{
			Redis: RedisConfig{
				Addr:         envString("REDIS_ADDR", defaultRedisAddr),
				Password:     os.Getenv("REDIS_PASSWORD"),
				DB:           envInt("REDIS_DB", 0),
				PoolSize:     envInt("REDIS_POOL_SIZE", 0),
				MinIdleConns: envInt("REDIS_MIN_IDLE_CONNS", 0),
				DialTimeout:  envDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
				ReadTimeout:  envDuration("REDIS_READ_TIMEOUT", 3*time.Second),
				WriteTimeout: envDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			},
			Kafka: KafkaConfig{
				Brokers:       splitNonEmpty(brokersValue),
				Topic:         envString("KAFKA_TOPIC", defaultKafkaTopic),
				ArchiverGroup: envString("KAFKA_ARCHIVER_GROUP", defaultKafkaArchiverGroup),
				ProducerAsync: envBool("KAFKA_PRODUCER_ASYNC", true),
			},
		}
	})
	return messagingConfig
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if broker := strings.TrimSpace(part); broker != "" {
			result = append(result, broker)
		}
	}
	if len(result) == 0 {
		return []string{defaultKafkaBroker}
	}
	return result
}
