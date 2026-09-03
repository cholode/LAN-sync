package metrics

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/v2/event"
)

var (
	dbCommandTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_db_command_total",
		Help: "MongoDB 命令执行累计数",
	}, []string{"db", "command", "status"})
	dbCommandErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_db_command_errors_total",
		Help: "MongoDB 命令错误累计数",
	}, []string{"db", "command", "error_type"})
	dbCommandDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_db_command_duration_seconds",
		Help:    "MongoDB 命令执行耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"db", "command"})
)

func init() {
	register(dbCommandTotal)
	register(dbCommandErrorsTotal)
	register(dbCommandDurationSeconds)
}

func NewMongoCommandMonitor(dbName string) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
		},
		Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
			command := strings.ToLower(evt.CommandName)
			dbCommandTotal.WithLabelValues(dbName, command, "success").Inc()
			dbCommandDurationSeconds.WithLabelValues(dbName, command).Observe(evt.Duration.Seconds())
		},
		Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
			command := strings.ToLower(evt.CommandName)
			dbCommandTotal.WithLabelValues(dbName, command, "error").Inc()
			dbCommandErrorsTotal.WithLabelValues(dbName, command, errorLabel(evt.Failure)).Inc()
			dbCommandDurationSeconds.WithLabelValues(dbName, command).Observe(evt.Duration.Seconds())
		},
	}
}
