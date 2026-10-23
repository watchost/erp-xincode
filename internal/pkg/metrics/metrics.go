// Copyright 2026 zhouhouping. All Rights Reserved.

package metrics

import (
	"database/sql"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	once sync.Once

	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "erp",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "erp",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request latencies in seconds.",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Current number of in-flight HTTP requests.",
		},
	)

	// Business events metrics
	businessEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "erp",
			Subsystem: "business",
			Name:      "events_total",
			Help:      "Total count of core business domain events.",
		},
		[]string{"module", "action", "status"},
	)
)

// Init registers all basic metrics with Prometheus default registry.
func Init() {
	once.Do(func() {
		prometheus.MustRegister(
			httpRequestsTotal,
			httpRequestDuration,
			httpRequestsInFlight,
			businessEventsTotal,
		)
	})
}

// RegisterDBStats registers DB connection pool statistics with Prometheus.
func RegisterDBStats(sqlDB *sql.DB) {
	if sqlDB == nil {
		return
	}

	_ = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "db",
			Name:      "open_connections",
			Help:      "The number of established connections both in use and idle.",
		},
		func() float64 {
			return float64(sqlDB.Stats().OpenConnections)
		},
	))

	_ = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "db",
			Name:      "in_use_connections",
			Help:      "The number of connections currently in use.",
		},
		func() float64 {
			return float64(sqlDB.Stats().InUse)
		},
	))

	_ = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "db",
			Name:      "idle_connections",
			Help:      "The number of idle connections.",
		},
		func() float64 {
			return float64(sqlDB.Stats().Idle)
		},
	))

	_ = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "db",
			Name:      "wait_count",
			Help:      "The total number of connections waited for.",
		},
		func() float64 {
			return float64(sqlDB.Stats().WaitCount)
		},
	))

	_ = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "db",
			Name:      "wait_duration_seconds",
			Help:      "The total time blocked waiting for a new connection.",
		},
		func() float64 {
			return sqlDB.Stats().WaitDuration.Seconds()
		},
	))

	_ = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "erp",
			Subsystem: "db",
			Name:      "max_open_connections",
			Help:      "Maximum number of open connections to the database.",
		},
		func() float64 {
			return float64(sqlDB.Stats().MaxOpenConnections)
		},
	))
}

// RecordBusinessEvent records business operation events like sales_order_create, inbound_post, etc.
func RecordBusinessEvent(module, action, status string) {
	businessEventsTotal.WithLabelValues(module, action, status).Inc()
}

// GinMiddleware provides Prometheus metrics instrumentation for HTTP routes.
func GinMiddleware() gin.HandlerFunc {
	Init()
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/metrics" || path == "/healthz" || path == "/readyz" {
			c.Next()
			return
		}

		httpRequestsInFlight.Inc()
		start := time.Now()

		c.Next()

		httpRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()

		routePath := c.FullPath()
		if routePath == "" {
			routePath = "unmatched"
		}

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		httpRequestsTotal.WithLabelValues(method, routePath, status).Inc()
		httpRequestDuration.WithLabelValues(method, routePath, status).Observe(duration)
	}
}

// Handler returns gin HandlerFunc for the /metrics endpoint.
func Handler() gin.HandlerFunc {
	Init()
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
