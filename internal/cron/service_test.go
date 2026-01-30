package cron

import (
	"context"
	"errors"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type fakeLock struct {
	acquired bool
}

func (f *fakeLock) Acquire(context.Context) (bool, error) {
	if f.acquired {
		return false, nil
	}
	f.acquired = true
	return true, nil
}

func (f *fakeLock) Release(context.Context) error { f.acquired = false; return nil }

type testJob struct {
	name string
	err  error
	runs int
}

func (t *testJob) Name() string { return t.name }

func (t *testJob) Run(context.Context) error {
	t.runs++
	return t.err
}

func TestServiceRunCycleRunsAllJobsEvenOnFailure(t *testing.T) {
	logg := logger.New(logger.Options{ServiceName: "cron-test"})
	registry := NewRegistry(&testJob{name: "success"}, &testJob{name: "fail", err: errors.New("boom")})
	service, err := NewService(ServiceParams{
		Logger:   logg,
		Registry: registry,
		Lock:     &fakeLock{},
		Interval: 0,
	})
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}
	ctx := context.Background()
	if err := service.runCycle(ctx); err != nil {
		t.Fatalf("run cycle: %v", err)
	}
	jobs := registry.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if success, ok := jobs[0].(*testJob); ok {
		if success.runs != 1 {
			t.Fatalf("expected success job to run once, ran %d", success.runs)
		}
	} else {
		t.Fatalf("first job type mismatch")
	}
	if failure, ok := jobs[1].(*testJob); ok {
		if failure.runs != 1 {
			t.Fatalf("expected failure job to run once, ran %d", failure.runs)
		}
	} else {
		t.Fatalf("second job type mismatch")
	}
}
