package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/metrics"
)

const defaultInterval = 24 * time.Hour

// ServiceParams configure the cron service.
type ServiceParams struct {
	Logger   *logger.Logger
	Registry *Registry
	Lock     Lock
	Metrics  *metrics.CronJobMetrics
	Interval time.Duration
}

// Service executes registered cron jobs on a fixed cadence.
type Service struct {
	logg     *logger.Logger
	registry *Registry
	lock     Lock
	metrics  *metrics.CronJobMetrics
	interval time.Duration
}

// NewService builds a cron service.
func NewService(params ServiceParams) (*Service, error) {
	if params.Logger == nil {
		return nil, fmt.Errorf("logger required")
	}
	if params.Lock == nil {
		return nil, fmt.Errorf("lock required")
	}
	registry := params.Registry
	if registry == nil {
		registry = NewRegistry()
	}
	interval := params.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	return &Service{
		logg:     params.Logger,
		registry: registry,
		lock:     params.Lock,
		metrics:  params.Metrics,
		interval: interval,
	}, nil
}

// Run starts the cron loop until the context is canceled.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.runCycle(ctx); err != nil {
		s.logg.Error(ctx, "scheduled run failed", err)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logg.Info(ctx, "cron service context canceled")
			return ctx.Err()
		case <-ticker.C:
			if err := s.runCycle(ctx); err != nil {
				s.logg.Error(ctx, "scheduled run failed", err)
			}
		}
	}
}

func (s *Service) runCycle(ctx context.Context) error {
	locked, err := s.lock.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("lock acquire: %w", err)
	}
	if !locked {
		s.logg.Info(ctx, "another cron instance is running; skipping this cycle")
		return nil
	}
	defer func() {
		if relErr := s.lock.Release(ctx); relErr != nil {
			s.logg.Error(ctx, "failed to release cron lock", relErr)
		}
	}()

	s.logg.Info(ctx, "scheduled run starting")
	for _, job := range s.registry.Jobs() {
		s.runJob(ctx, job)
	}
	s.logg.Info(ctx, "scheduled run complete")
	return nil
}

func (s *Service) runJob(ctx context.Context, job Job) {
	jobCtx := s.logg.WithField(ctx, "job", job.Name())
	jobCtx = s.logg.WithField(jobCtx, "event", "cron.job")
	s.logg.Info(jobCtx, "job start")
	start := time.Now()
	err := job.Run(jobCtx)
	duration := time.Since(start)
	s.observeDuration(job.Name(), duration)
	jobCtx = s.logg.WithField(jobCtx, "duration_ms", duration.Milliseconds())
	if err != nil {
		s.logg.Error(jobCtx, "job failed", err)
		s.recordFailure(job.Name())
		return
	}
	s.logg.Info(jobCtx, "job completed")
	s.recordSuccess(job.Name())
}

func (s *Service) observeDuration(job string, duration time.Duration) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveDuration(job, duration)
}

func (s *Service) recordSuccess(job string) {
	if s.metrics == nil {
		return
	}
	s.metrics.IncSuccess(job)
}

func (s *Service) recordFailure(job string) {
	if s.metrics == nil {
		return
	}
	s.metrics.IncFailure(job)
}
