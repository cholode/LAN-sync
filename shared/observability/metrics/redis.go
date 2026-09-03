package metrics

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	redisOpsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_redis_ops_total",
		Help: "Redis 命令执行累计数",
	}, []string{"operation", "status"})
	redisErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_redis_errors_total",
		Help: "Redis 命令错误累计数",
	}, []string{"operation", "error_type"})
	redisLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_redis_latency_seconds",
		Help:    "Redis 命令执行耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})
	redisPubSubTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_redis_pubsub_total",
		Help: "Redis Pub/Sub 事件累计数",
	}, []string{"channel", "action", "status"})
	redisUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_redis_up",
		Help: "Redis 连接是否可用，1 表示可用",
	})
)

func init() {
	register(redisOpsTotal)
	register(redisErrorsTotal)
	register(redisLatencySeconds)
	register(redisPubSubTotal)
	register(redisUp)
}

func SetRedisUp(up bool) {
	if up {
		redisUp.Set(1)
		return
	}
	redisUp.Set(0)
}

func ObserveRedisPubSub(channel, action string, err error) {
	redisPubSubTotal.WithLabelValues(channel, action, statusLabel(err)).Inc()
}

type redisContextKey struct{}

type RedisHook struct{}

func NewRedisHook() *RedisHook {
	return &RedisHook{}
}

func (h *RedisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, redisContextKey{}, time.Now()), nil
}

func (h *RedisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	start, _ := ctx.Value(redisContextKey{}).(time.Time)
	observeRedisCommand(cmd.Name(), start, cmd.Err())
	return nil
}

func (h *RedisHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, redisContextKey{}, time.Now()), nil
}

func (h *RedisHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	start, _ := ctx.Value(redisContextKey{}).(time.Time)
	err := firstRedisError(cmds)
	observeRedisCommand("pipeline", start, err)
	return nil
}

func observeRedisCommand(operation string, start time.Time, err error) {
	if err == nil || errors.Is(err, redis.Nil) {
		redisOpsTotal.WithLabelValues(operation, "success").Inc()
	} else {
		redisOpsTotal.WithLabelValues(operation, "error").Inc()
		redisErrorsTotal.WithLabelValues(operation, errorLabel(err)).Inc()
	}
	redisLatencySeconds.WithLabelValues(operation).Observe(time.Since(start).Seconds())
}

func firstRedisError(cmds []redis.Cmder) error {
	for _, cmd := range cmds {
		if err := cmd.Err(); err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
	}
	return nil
}

func RegisterRedisPoolMetrics(client *redis.Client) {
	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "im_redis_pool_idle_connections",
		Help: "Redis 连接池空闲连接数",
	}, func() float64 {
		return float64(client.PoolStats().IdleConns)
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "im_redis_pool_total_connections",
		Help: "Redis 连接池总连接数",
	}, func() float64 {
		return float64(client.PoolStats().TotalConns)
	}))
}
