package metrics

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

var (
	dbQueryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_db_query_total",
		Help: "数据库查询累计数",
	}, []string{"db", "operation", "table", "status"})
	dbQueryErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_db_query_errors_total",
		Help: "数据库查询错误累计数",
	}, []string{"db", "operation", "error_type"})
	dbQueryDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "im_db_query_duration_seconds",
		Help:    "数据库查询耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"db", "operation", "table"})
)

func init() {
	register(dbQueryTotal)
	register(dbQueryErrorsTotal)
	register(dbQueryDurationSeconds)
}

type gormContextKey struct{}

var (
	gormMetricsMu         sync.Mutex
	gormMetricsRegistered = make(map[*gorm.DB]bool)
)

func RegisterGORMMetrics(db *gorm.DB, dbName string) {
	if db == nil {
		return
	}

	gormMetricsMu.Lock()
	defer gormMetricsMu.Unlock()
	if gormMetricsRegistered[db] {
		return
	}
	gormMetricsRegistered[db] = true

	before := func(db *gorm.DB) {
		if db.Statement == nil {
			return
		}
		if db.Statement.Context == nil {
			db.Statement.Context = context.Background()
		}
		db.Statement.Context = context.WithValue(db.Statement.Context, gormContextKey{}, time.Now())
	}

	after := func(operation string) func(*gorm.DB) {
		return func(db *gorm.DB) {
			if db.Statement == nil || db.Statement.Context == nil {
				return
			}
			start, _ := db.Statement.Context.Value(gormContextKey{}).(time.Time)
			if start.IsZero() {
				return
			}
			observeGORM(dbName, operation, tableName(db), start, db.Error)
		}
	}

	_ = db.Callback().Query().Before("gorm:query").Register("metrics:before_query", before)
	_ = db.Callback().Query().After("gorm:query").Register("metrics:after_query", after("query"))

	_ = db.Callback().Create().Before("gorm:create").Register("metrics:before_create", before)
	_ = db.Callback().Create().After("gorm:create").Register("metrics:after_create", after("create"))

	_ = db.Callback().Update().Before("gorm:update").Register("metrics:before_update", before)
	_ = db.Callback().Update().After("gorm:update").Register("metrics:after_update", after("update"))

	_ = db.Callback().Delete().Before("gorm:delete").Register("metrics:before_delete", before)
	_ = db.Callback().Delete().After("gorm:delete").Register("metrics:after_delete", after("delete"))
}

func tableName(db *gorm.DB) string {
	if db.Statement.Table != "" {
		return db.Statement.Table
	}
	if db.Statement.Schema != nil && db.Statement.Schema.Table != "" {
		return db.Statement.Schema.Table
	}
	return "unknown"
}

func observeGORM(dbName, operation, table string, start time.Time, err error) {
	dbQueryTotal.WithLabelValues(dbName, operation, table, statusLabel(err)).Inc()
	dbQueryDurationSeconds.WithLabelValues(dbName, operation, table).Observe(time.Since(start).Seconds())
	if err != nil {
		dbQueryErrorsTotal.WithLabelValues(dbName, operation, errorLabel(err)).Inc()
	}
}

func RegisterMySQLPoolMetrics(sqlDB *sql.DB) {
	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "im_db_pool_open_connections",
		Help:        "MySQL 当前打开的连接数",
		ConstLabels: prometheus.Labels{"db": "mysql"},
	}, func() float64 {
		return float64(sqlDB.Stats().OpenConnections)
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "im_db_pool_idle_connections",
		Help:        "MySQL 当前空闲连接数",
		ConstLabels: prometheus.Labels{"db": "mysql"},
	}, func() float64 {
		return float64(sqlDB.Stats().Idle)
	}))

	register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "im_db_pool_in_use_connections",
		Help:        "MySQL 当前使用中的连接数",
		ConstLabels: prometheus.Labels{"db": "mysql"},
	}, func() float64 {
		return float64(sqlDB.Stats().InUse)
	}))

	register(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "im_db_pool_wait_count_total",
		Help:        "MySQL 等待连接累计次数",
		ConstLabels: prometheus.Labels{"db": "mysql"},
	}, func() float64 {
		return float64(sqlDB.Stats().WaitCount)
	}))

	register(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "im_db_pool_wait_duration_seconds_total",
		Help:        "MySQL 等待连接累计秒数",
		ConstLabels: prometheus.Labels{"db": "mysql"},
	}, func() float64 {
		return sqlDB.Stats().WaitDuration.Seconds()
	}))
}
