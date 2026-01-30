package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// CronJobMetrics records metadata for scheduled jobs.
type CronJobMetrics struct {
	duration *prometheus.HistogramVec
	success  *prometheus.CounterVec
	failure  *prometheus.CounterVec
}

// NewCronJobMetrics registers the cron job metrics on the provided registerer.
func NewCronJobMetrics(reg prometheus.Registerer) *CronJobMetrics {
	if reg == nil {
		return &CronJobMetrics{}
	}
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "job_duration_seconds",
		Help:    "Duration of cron jobs in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"job"})
	success := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "job_success",
		Help: "Successful cron job executions.",
	}, []string{"job"})
	failure := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "job_failure",
		Help: "Failed cron job executions.",
	}, []string{"job"})
	reg.MustRegister(duration, success, failure)
	return &CronJobMetrics{
		duration: duration,
		success:  success,
		failure:  failure,
	}
}

// ObserveDuration records the duration for the named job.
func (c *CronJobMetrics) ObserveDuration(job string, duration time.Duration) {
	if c == nil || c.duration == nil {
		return
	}
	c.duration.WithLabelValues(normalizeLabel(job)).Observe(duration.Seconds())
}

// IncSuccess increments the success counter for the named job.
func (c *CronJobMetrics) IncSuccess(job string) {
	if c == nil || c.success == nil {
		return
	}
	c.success.WithLabelValues(normalizeLabel(job)).Inc()
}

// IncFailure increments the failure counter for the named job.
func (c *CronJobMetrics) IncFailure(job string) {
	if c == nil || c.failure == nil {
		return
	}
	c.failure.WithLabelValues(normalizeLabel(job)).Inc()
}

func normalizeLabel(job string) string {
	if job == "" {
		return "unknown"
	}
	return job
}
