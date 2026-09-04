package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	loginAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_auth_login_attempts_total",
		Help: "Login attempts grouped by result.",
	}, []string{"result"})
	loginDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "im_auth_login_duration_seconds",
		Help:    "End-to-end login request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	})
	bcryptActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_auth_bcrypt_active",
		Help: "Password comparisons currently executing.",
	})
)

func init() {
	register(loginAttempts)
	register(loginDuration)
	register(bcryptActive)
}

func ObserveLogin(start time.Time, result string) {
	loginAttempts.WithLabelValues(result).Inc()
	loginDuration.Observe(time.Since(start).Seconds())
}

func BcryptStarted()  { bcryptActive.Inc() }
func BcryptFinished() { bcryptActive.Dec() }
